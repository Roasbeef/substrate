package subtraterpc

import (
	"context"
	"database/sql"
	"errors"

	"github.com/roasbeef/subtrate/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreatePlanReview creates a new plan review record.
func (s *Server) CreatePlanReview(
	ctx context.Context, req *CreatePlanReviewRequest,
) (*PlanReviewProto, error) {
	if req.PlanReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "plan_review_id is required",
		)
	}
	if req.RequesterId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "requester_id is required",
		)
	}

	params := store.CreatePlanReviewParams{
		PlanReviewID: req.PlanReviewId,
		ThreadID:     req.ThreadId,
		RequesterID:  req.RequesterId,
		ReviewerName: req.ReviewerName,
		PlanPath:     req.PlanPath,
		PlanTitle:    req.PlanTitle,
		PlanSummary:  req.PlanSummary,
		SessionID:    req.SessionId,
	}
	if req.MessageId != 0 {
		msgID := req.MessageId
		params.MessageID = &msgID
	}

	pr, err := s.planReviewStore.CreatePlanReview(ctx, params)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to create plan review: %v", err,
		)
	}

	return planReviewToProto(pr), nil
}

// GetPlanReview retrieves a plan review by its UUID.
func (s *Server) GetPlanReview(
	ctx context.Context, req *GetPlanReviewRequest,
) (*PlanReviewProto, error) {
	if req.PlanReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "plan_review_id is required",
		)
	}

	pr, err := s.planReviewStore.GetPlanReview(ctx, req.PlanReviewId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(
				codes.NotFound,
				"plan review %q not found", req.PlanReviewId,
			)
		}
		return nil, status.Errorf(
			codes.Internal,
			"failed to get plan review: %v", err,
		)
	}

	return planReviewToProto(pr), nil
}

// GetPlanReviewByThread retrieves the latest plan review for a thread.
func (s *Server) GetPlanReviewByThread(
	ctx context.Context, req *GetPlanReviewByThreadRequest,
) (*PlanReviewProto, error) {
	if req.ThreadId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "thread_id is required",
		)
	}

	pr, err := s.planReviewStore.GetPlanReviewByThread(
		ctx, req.ThreadId,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(
				codes.NotFound,
				"no plan review for thread %q", req.ThreadId,
			)
		}
		return nil, status.Errorf(
			codes.Internal,
			"failed to get plan review by thread: %v", err,
		)
	}

	return planReviewToProto(pr), nil
}

// GetPlanReviewBySession retrieves the pending plan review for a session.
func (s *Server) GetPlanReviewBySession(
	ctx context.Context, req *GetPlanReviewBySessionRequest,
) (*PlanReviewProto, error) {
	if req.SessionId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "session_id is required",
		)
	}

	pr, err := s.planReviewStore.GetPlanReviewBySession(
		ctx, req.SessionId,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(
				codes.NotFound,
				"no pending plan review for session %q",
				req.SessionId,
			)
		}
		return nil, status.Errorf(
			codes.Internal,
			"failed to get plan review by session: %v", err,
		)
	}

	return planReviewToProto(pr), nil
}

// ListPlanReviews lists plan reviews with optional filters.
func (s *Server) ListPlanReviews(
	ctx context.Context, req *ListPlanReviewsRequest,
) (*ListPlanReviewsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)

	var (
		reviews []store.PlanReview
		err     error
	)

	switch {
	case req.State != "" && req.RequesterId != 0:
		// Filter by requester, then filter by state in memory.
		reviews, err = s.planReviewStore.ListPlanReviewsByRequester(
			ctx, req.RequesterId, limit,
		)
		if err == nil {
			filtered := make([]store.PlanReview, 0, len(reviews))
			for _, r := range reviews {
				if r.State == req.State {
					filtered = append(filtered, r)
				}
			}
			reviews = filtered
		}
	case req.State != "":
		reviews, err = s.planReviewStore.ListPlanReviewsByState(
			ctx, req.State, limit,
		)
	case req.RequesterId != 0:
		reviews, err = s.planReviewStore.ListPlanReviewsByRequester(
			ctx, req.RequesterId, limit,
		)
	default:
		reviews, err = s.planReviewStore.ListPlanReviews(
			ctx, limit, offset,
		)
	}
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to list plan reviews: %v", err,
		)
	}

	protos := make([]*PlanReviewProto, len(reviews))
	for i, pr := range reviews {
		protos[i] = planReviewToProto(pr)
	}

	return &ListPlanReviewsResponse{PlanReviews: protos}, nil
}

// UpdatePlanReviewStatus updates the status of a plan review.
func (s *Server) UpdatePlanReviewStatus(
	ctx context.Context, req *UpdatePlanReviewStatusRequest,
) (*PlanReviewProto, error) {
	if req.PlanReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "plan_review_id is required",
		)
	}
	if req.State == "" {
		return nil, status.Error(
			codes.InvalidArgument, "state is required",
		)
	}

	// Validate state value.
	validStates := map[string]bool{
		"approved":          true,
		"rejected":          true,
		"changes_requested": true,
	}
	if !validStates[req.State] {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid state %q: must be approved, rejected, "+
				"or changes_requested", req.State,
		)
	}

	params := store.UpdatePlanReviewStateParams{
		PlanReviewID:    req.PlanReviewId,
		State:           req.State,
		ReviewerComment: req.ReviewerComment,
	}
	if req.ReviewedBy != 0 {
		reviewedBy := req.ReviewedBy
		params.ReviewedBy = &reviewedBy
	}

	err := s.planReviewStore.UpdatePlanReviewState(ctx, params)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to update plan review: %v", err,
		)
	}

	// Return the updated record.
	pr, err := s.planReviewStore.GetPlanReview(
		ctx, req.PlanReviewId,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to fetch updated plan review: %v", err,
		)
	}

	return planReviewToProto(pr), nil
}

// DeletePlanReview deletes a plan review by its UUID.
func (s *Server) DeletePlanReview(
	ctx context.Context, req *DeletePlanReviewRequest,
) (*DeletePlanReviewResponse, error) {
	if req.PlanReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "plan_review_id is required",
		)
	}

	err := s.planReviewStore.DeletePlanReview(ctx, req.PlanReviewId)
	if err != nil {
		return &DeletePlanReviewResponse{
			Error: err.Error(),
		}, nil
	}

	return &DeletePlanReviewResponse{}, nil
}

// planReviewToProto converts a store.PlanReview to a PlanReviewProto.
func planReviewToProto(pr store.PlanReview) *PlanReviewProto {
	proto := &PlanReviewProto{
		Id:              pr.ID,
		PlanReviewId:    pr.PlanReviewID,
		ThreadId:        pr.ThreadID,
		RequesterId:     pr.RequesterID,
		ReviewerName:    pr.ReviewerName,
		PlanPath:        pr.PlanPath,
		PlanTitle:       pr.PlanTitle,
		PlanSummary:     pr.PlanSummary,
		State:           pr.State,
		ReviewerComment: pr.ReviewerComment,
		SessionId:       pr.SessionID,
		CreatedAt:       pr.CreatedAt.Unix(),
		UpdatedAt:       pr.UpdatedAt.Unix(),
	}

	if pr.MessageID != nil {
		proto.MessageId = *pr.MessageID
	}
	if pr.ReviewedBy != nil {
		proto.ReviewedBy = *pr.ReviewedBy
	}
	if pr.ReviewedAt != nil {
		proto.ReviewedAt = pr.ReviewedAt.Unix()
	}

	return proto
}
