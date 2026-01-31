package mail

import (
	"github.com/lightninglabs/darepo-client/baselib/actor"
	"github.com/roasbeef/subtrate/internal/db"
)

// MailActorRef is the typed actor reference for the mail service.
type MailActorRef = actor.ActorRef[MailRequest, MailResponse]

// MailTellOnlyRef is a tell-only reference to the mail service.
type MailTellOnlyRef = actor.TellOnlyRef[MailRequest]

// ActorConfig holds configuration for creating a mail actor.
type ActorConfig struct {
	// ID is the unique identifier for the actor.
	ID string

	// Store is the database store.
	Store *db.Store

	// MailboxSize is the buffer capacity for the actor's mailbox.
	MailboxSize int
}

// NewMailActor creates a new mail actor with the given configuration.
func NewMailActor(cfg ActorConfig) *actor.Actor[MailRequest, MailResponse] {
	svc := NewService(cfg.Store)

	mailboxSize := cfg.MailboxSize
	if mailboxSize <= 0 {
		mailboxSize = 100
	}

	actorID := cfg.ID
	if actorID == "" {
		actorID = "mail-service"
	}

	return actor.NewActor(actor.ActorConfig[MailRequest, MailResponse]{
		ID:          actorID,
		Behavior:    svc,
		MailboxSize: mailboxSize,
	})
}

// StartMailActor creates and starts a new mail actor, returning its reference.
func StartMailActor(cfg ActorConfig) MailActorRef {
	a := NewMailActor(cfg)
	a.Start()
	return a.Ref()
}

// Ensure Service implements ActorBehavior.
var _ actor.ActorBehavior[MailRequest, MailResponse] = (*Service)(nil)
