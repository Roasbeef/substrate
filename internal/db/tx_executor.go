package db

import (
	"context"
	"log/slog"
	"math"
	prand "math/rand"
	"time"
)

// txExecutorOptions is a struct that holds the options for the transaction
// executor. This can be used to do things like retry a transaction due to an
// error a certain amount of times.
type txExecutorOptions struct {
	numRetries        int
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
}

// defaultTxExecutorOptions returns the default options for the transaction
// executor.
func defaultTxExecutorOptions() *txExecutorOptions {
	return &txExecutorOptions{
		numRetries:        DefaultNumTxRetries,
		initialRetryDelay: DefaultInitialRetryDelay,
		maxRetryDelay:     DefaultMaxRetryDelay,
	}
}

// randRetryDelay returns a random retry delay between -50% and +50% of the
// configured delay that is doubled for each attempt and capped at a max value.
func (t *txExecutorOptions) randRetryDelay(attempt int) time.Duration {
	halfDelay := t.initialRetryDelay / 2
	randDelay := prand.Int63n(int64(t.initialRetryDelay)) //nolint:gosec

	// 50% plus 0%-100% gives us the range of 50%-150%.
	initialDelay := halfDelay + time.Duration(randDelay)

	// If this is the first attempt, we just return the initial delay.
	if attempt == 0 {
		return initialDelay
	}

	// For each subsequent delay, we double the initial delay. This still
	// gives us a somewhat random delay, but it still increases with each
	// attempt. If we double something n times, that's the same as
	// multiplying the value with 2^n. We limit the power to 32 to avoid
	// overflows.
	factor := time.Duration(math.Pow(2, math.Min(float64(attempt), 32)))
	//nolint:durationcheck
	actualDelay := initialDelay * factor

	// Cap the delay at the maximum configured value.
	if actualDelay > t.maxRetryDelay {
		return t.maxRetryDelay
	}

	return actualDelay
}

// TxExecutorOption is a functional option that allows us to pass in optional
// argument when creating the executor.
type TxExecutorOption func(*txExecutorOptions)

// WithTxRetries is a functional option that allows us to specify the number of
// times a transaction should be retried if it fails with a repeatable error.
func WithTxRetries(numRetries int) TxExecutorOption {
	return func(o *txExecutorOptions) {
		o.numRetries = numRetries
	}
}

// WithTxRetryDelay is a functional option that allows us to specify the delay
// to wait before a transaction is retried.
func WithTxRetryDelay(delay time.Duration) TxExecutorOption {
	return func(o *txExecutorOptions) {
		o.initialRetryDelay = delay
	}
}

// TransactionExecutor is a generic struct that abstracts away from the type of
// query a type needs to run under a database transaction, and also the set of
// options for that transaction. The QueryCreator is used to create a query
// given a database transaction created by the BatchedQuerier.
type TransactionExecutor[Query any] struct {
	BatchedQuerier

	createQuery QueryCreator[Query]

	opts *txExecutorOptions

	log *slog.Logger
}

// NewTransactionExecutor creates a new instance of a TransactionExecutor given
// a Querier query object and a concrete type for the type of transactions the
// Querier understands.
func NewTransactionExecutor[Querier any](db BatchedQuerier,
	createQuery QueryCreator[Querier], log *slog.Logger,
	opts ...TxExecutorOption,
) *TransactionExecutor[Querier] {
	txOpts := defaultTxExecutorOptions()
	for _, optFunc := range opts {
		optFunc(txOpts)
	}

	return &TransactionExecutor[Querier]{
		BatchedQuerier: db,
		createQuery:    createQuery,
		opts:           txOpts,
		log:            log,
	}
}

// ExecTx is a wrapper for txBody to abstract the creation and commit of a db
// transaction. The db transaction is embedded in a `*Queries` that txBody
// needs to use when executing each one of the queries that need to be applied
// atomically. This can be used by other storage interfaces to parameterize the
// type of query and options run, in order to have access to batched operations
// related to a storage object.
func (t *TransactionExecutor[Q]) ExecTx(ctx context.Context,
	txOptions TxOptions, txBody func(Q) error,
) error {
	waitBeforeRetry := func(attemptNumber int) {
		retryDelay := t.opts.randRetryDelay(attemptNumber)

		t.log.DebugContext(
			ctx,
			"Retrying transaction due to tx serialization or "+
				"deadlock error",
			"attempt_number", attemptNumber,
			"delay", retryDelay,
		)

		// Before we try again, we'll wait with a random backoff based
		// on the retry delay.
		time.Sleep(retryDelay)
	}

	for i := 0; i < t.opts.numRetries; i++ {
		// Create the db transaction.
		tx, err := t.BeginTx(ctx, txOptions)
		if err != nil {
			dbErr := MapSQLError(err)
			if IsSerializationOrDeadlockError(dbErr) {
				// Nothing to roll back here, since we didn't
				// even get a transaction yet.
				waitBeforeRetry(i)
				continue
			}

			return dbErr
		}

		// Rollback is safe to call even if the tx is already closed,
		// so if the tx commits successfully, this is a no-op.
		defer func() {
			_ = tx.Rollback()
		}()

		if err := txBody(t.createQuery(tx)); err != nil {
			dbErr := MapSQLError(err)
			if IsSerializationOrDeadlockError(dbErr) {
				// Roll back the transaction, then pop back up
				// to try once again.
				_ = tx.Rollback()

				waitBeforeRetry(i)

				continue
			}

			return dbErr
		}

		// Commit transaction.
		if err = tx.Commit(); err != nil {
			dbErr := MapSQLError(err)
			if IsSerializationOrDeadlockError(dbErr) {
				// Commit failed due to serialization/deadlock,
				// clean up transaction state before retry.
				_ = tx.Rollback()

				waitBeforeRetry(i)

				continue
			}

			return dbErr
		}

		return nil
	}

	// If we get to this point, then we weren't able to successfully commit
	// a tx given the max number of retries.
	return ErrRetriesExceeded
}
