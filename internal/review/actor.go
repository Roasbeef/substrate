package review

import "github.com/roasbeef/subtrate/internal/baselib/actor"

// ReviewActorRef is the typed actor reference for the review service.
type ReviewActorRef = actor.ActorRef[ReviewRequest, ReviewResponse]

// ReviewTellOnlyRef is a tell-only reference to the review service.
type ReviewTellOnlyRef = actor.TellOnlyRef[ReviewRequest]
