package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/roasbeef/subtrate/internal/db/sqlc"
)

// DefaultStoreTimeout is the default timeout used for any interaction
// with the storage/database.
var DefaultStoreTimeout = time.Second * 10

const (
	// DefaultNumTxRetries is the default number of times we'll retry a
	// transaction if it fails with an error that permits transaction
	// repetition.
	DefaultNumTxRetries = 10

	// DefaultInitialRetryDelay is the default initial delay between
	// retries. This will be used to generate a random delay between -50%
	// and +50% of this value, so 20 to 60 milliseconds. The retry will be
	// doubled after each attempt until we reach DefaultMaxRetryDelay. We
	// start with a random value to avoid multiple goroutines that are
	// created at the same time to effectively retry at the same time.
	DefaultInitialRetryDelay = time.Millisecond * 40

	// DefaultMaxRetryDelay is the default maximum delay between retries.
	DefaultMaxRetryDelay = time.Second * 3
)

// TxOptions represents a set of options one can use to control what type of
// database transaction is created. Transaction can either be read or write.
type TxOptions interface {
	// ReadOnly returns true if the transaction should be read-only.
	ReadOnly() bool
}

// BaseTxOptions defines the set of db txn options the database understands.
type BaseTxOptions struct {
	// readOnly governs if a read-only transaction is needed or not.
	readOnly bool
}

// ReadOnly returns true if the transaction should be read only.
//
// NOTE: This implements the TxOptions interface.
func (a *BaseTxOptions) ReadOnly() bool {
	return a.readOnly
}

// ReadTxOption returns a TxOptions that indicates a read-only transaction.
func ReadTxOption() *BaseTxOptions {
	return &BaseTxOptions{
		readOnly: true,
	}
}

// WriteTxOption returns a TxOptions that indicates a write transaction.
func WriteTxOption() *BaseTxOptions {
	return &BaseTxOptions{
		readOnly: false,
	}
}

// BatchedTx is a generic interface that represents the ability to execute
// several operations to a given storage interface in a single atomic
// transaction. Typically, Q here will be some subset of the main sqlc.Querier
// interface allowing it to only depend on the routines it needs to implement
// any additional business logic.
type BatchedTx[Q any] interface {
	// ExecTx will execute the passed txBody, operating upon generic
	// parameter Q (usually a storage interface) in a single transaction.
	// The set of TxOptions are passed in order to allow the caller to
	// specify if a transaction should be read-only and optionally what
	// type of concurrency control should be used.
	ExecTx(ctx context.Context, txOptions TxOptions,
		txBody func(Q) error) error
}

// Tx represents a database transaction that can be committed or rolled back.
type Tx interface {
	// Commit commits the database transaction, an error should be returned
	// if the commit isn't possible.
	Commit() error

	// Rollback rolls back an incomplete database transaction.
	// Transactions that were able to be committed can still call this as a
	// noop.
	Rollback() error
}

// QueryCreator is a generic function that's used to create a Querier, which is
// a type of interface that implements storage related methods from a database
// transaction. This will be used to instantiate an object callers can use to
// apply multiple modifications to an object interface in a single atomic
// transaction.
type QueryCreator[Q any] func(*sql.Tx) Q

// BatchedQuerier is a generic interface that allows callers to create a new
// database transaction based on an abstract type that implements the TxOptions
// interface.
type BatchedQuerier interface {
	// Querier is the underlying query source, this is in place so we can
	// pass a BatchedQuerier implementation directly into objects that
	// create a batched version of the normal methods they need.
	sqlc.Querier

	// BeginTx creates a new database transaction given the set of
	// transaction options.
	BeginTx(ctx context.Context, options TxOptions) (*sql.Tx, error)
}

// BaseDB is the base database struct that each implementation can embed to
// gain some common functionality.
type BaseDB struct {
	*sql.DB

	*sqlc.Queries
}

// NewBaseDB creates a new BaseDB instance from a sql.DB connection.
func NewBaseDB(db *sql.DB) *BaseDB {
	return &BaseDB{
		DB:      db,
		Queries: sqlc.New(db),
	}
}

// BeginTx wraps the normal sql specific BeginTx method with the TxOptions
// interface. This interface is then mapped to the concrete sql tx options
// struct.
func (s *BaseDB) BeginTx(ctx context.Context, opts TxOptions) (*sql.Tx, error) {
	sqlOptions := sql.TxOptions{
		ReadOnly: opts.ReadOnly(),
	}

	return s.DB.BeginTx(ctx, &sqlOptions)
}
