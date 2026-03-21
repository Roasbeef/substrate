package subtraterpc

import (
	"context"

	"github.com/roasbeef/subtrate/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// Plan Annotation RPCs
// =============================================================================

// CreatePlanAnnotation creates a new plan annotation.
func (s *Server) CreatePlanAnnotation(
	ctx context.Context, req *CreatePlanAnnotationRequest,
) (*PlanAnnotationProto, error) {
	if req.PlanReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "plan_review_id is required",
		)
	}
	if req.AnnotationId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "annotation_id is required",
		)
	}

	params := store.CreatePlanAnnotationParams{
		PlanReviewID:   req.PlanReviewId,
		AnnotationID:   req.AnnotationId,
		BlockID:        req.BlockId,
		AnnotationType: req.AnnotationType,
		Text:           req.Text,
		OriginalText:   req.OriginalText,
		StartOffset:    int(req.StartOffset),
		EndOffset:      int(req.EndOffset),
		DiffContext:    req.DiffContext,
	}

	ann, err := s.annotationStore.CreatePlanAnnotation(ctx, params)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to create plan annotation: %v", err,
		)
	}

	return planAnnotationToProto(ann), nil
}

// ListPlanAnnotations lists all annotations for a plan review.
func (s *Server) ListPlanAnnotations(
	ctx context.Context, req *ListPlanAnnotationsRequest,
) (*ListPlanAnnotationsResponse, error) {
	if req.PlanReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "plan_review_id is required",
		)
	}

	annotations, err := s.annotationStore.ListPlanAnnotationsByReview(
		ctx, req.PlanReviewId,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to list plan annotations: %v", err,
		)
	}

	protos := make([]*PlanAnnotationProto, 0, len(annotations))
	for _, ann := range annotations {
		protos = append(protos, planAnnotationToProto(ann))
	}

	return &ListPlanAnnotationsResponse{Annotations: protos}, nil
}

// UpdatePlanAnnotation updates a plan annotation.
func (s *Server) UpdatePlanAnnotation(
	ctx context.Context, req *UpdatePlanAnnotationRequest,
) (*PlanAnnotationProto, error) {
	if req.AnnotationId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "annotation_id is required",
		)
	}

	params := store.UpdatePlanAnnotationParams{
		AnnotationID: req.AnnotationId,
		Text:         req.Text,
		OriginalText: req.OriginalText,
		StartOffset:  int(req.StartOffset),
		EndOffset:    int(req.EndOffset),
		DiffContext:  req.DiffContext,
	}

	err := s.annotationStore.UpdatePlanAnnotation(ctx, params)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to update plan annotation: %v", err,
		)
	}

	// Re-fetch to return the updated annotation.
	ann, err := s.annotationStore.GetPlanAnnotation(
		ctx, req.AnnotationId,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to get updated plan annotation: %v", err,
		)
	}

	return planAnnotationToProto(ann), nil
}

// DeletePlanAnnotation deletes a plan annotation.
func (s *Server) DeletePlanAnnotation(
	ctx context.Context, req *DeletePlanAnnotationRequest,
) (*DeleteAnnotationResponse, error) {
	if req.AnnotationId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "annotation_id is required",
		)
	}

	err := s.annotationStore.DeletePlanAnnotation(ctx, req.AnnotationId)
	if err != nil {
		return &DeleteAnnotationResponse{
			Error: err.Error(),
		}, nil
	}

	return &DeleteAnnotationResponse{}, nil
}

// =============================================================================
// Diff Annotation RPCs
// =============================================================================

// CreateDiffAnnotation creates a new diff annotation.
func (s *Server) CreateDiffAnnotation(
	ctx context.Context, req *CreateDiffAnnotationRequest,
) (*DiffAnnotationProto, error) {
	if req.AnnotationId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "annotation_id is required",
		)
	}
	if req.MessageId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "message_id is required",
		)
	}

	params := store.CreateDiffAnnotationParams{
		AnnotationID:   req.AnnotationId,
		MessageID:      req.MessageId,
		AnnotationType: req.AnnotationType,
		Scope:          req.Scope,
		FilePath:       req.FilePath,
		LineStart:      int(req.LineStart),
		LineEnd:        int(req.LineEnd),
		Side:           req.Side,
		Text:           req.Text,
		SuggestedCode:  req.SuggestedCode,
		OriginalCode:   req.OriginalCode,
	}

	ann, err := s.annotationStore.CreateDiffAnnotation(ctx, params)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to create diff annotation: %v", err,
		)
	}

	return diffAnnotationToProto(ann), nil
}

// ListDiffAnnotations lists all annotations for a diff message.
func (s *Server) ListDiffAnnotations(
	ctx context.Context, req *ListDiffAnnotationsRequest,
) (*ListDiffAnnotationsResponse, error) {
	if req.MessageId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "message_id is required",
		)
	}

	annotations, err := s.annotationStore.ListDiffAnnotationsByMessage(
		ctx, req.MessageId,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to list diff annotations: %v", err,
		)
	}

	protos := make([]*DiffAnnotationProto, 0, len(annotations))
	for _, ann := range annotations {
		protos = append(protos, diffAnnotationToProto(ann))
	}

	return &ListDiffAnnotationsResponse{Annotations: protos}, nil
}

// UpdateDiffAnnotation updates a diff annotation.
func (s *Server) UpdateDiffAnnotation(
	ctx context.Context, req *UpdateDiffAnnotationRequest,
) (*DiffAnnotationProto, error) {
	if req.AnnotationId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "annotation_id is required",
		)
	}

	params := store.UpdateDiffAnnotationParams{
		AnnotationID:  req.AnnotationId,
		Text:          req.Text,
		SuggestedCode: req.SuggestedCode,
		OriginalCode:  req.OriginalCode,
	}

	err := s.annotationStore.UpdateDiffAnnotation(ctx, params)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to update diff annotation: %v", err,
		)
	}

	ann, err := s.annotationStore.GetDiffAnnotation(
		ctx, req.AnnotationId,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to get updated diff annotation: %v", err,
		)
	}

	return diffAnnotationToProto(ann), nil
}

// DeleteDiffAnnotation deletes a diff annotation.
func (s *Server) DeleteDiffAnnotation(
	ctx context.Context, req *DeleteDiffAnnotationRequest,
) (*DeleteAnnotationResponse, error) {
	if req.AnnotationId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "annotation_id is required",
		)
	}

	err := s.annotationStore.DeleteDiffAnnotation(ctx, req.AnnotationId)
	if err != nil {
		return &DeleteAnnotationResponse{
			Error: err.Error(),
		}, nil
	}

	return &DeleteAnnotationResponse{}, nil
}

// =============================================================================
// Proto conversion helpers
// =============================================================================

// planAnnotationToProto converts a store PlanAnnotation to a proto message.
func planAnnotationToProto(ann store.PlanAnnotation) *PlanAnnotationProto {
	return &PlanAnnotationProto{
		Id:             ann.ID,
		PlanReviewId:   ann.PlanReviewID,
		AnnotationId:   ann.AnnotationID,
		BlockId:        ann.BlockID,
		AnnotationType: ann.AnnotationType,
		Text:           ann.Text,
		OriginalText:   ann.OriginalText,
		StartOffset:    int32(ann.StartOffset),
		EndOffset:      int32(ann.EndOffset),
		DiffContext:    ann.DiffContext,
		CreatedAt:      ann.CreatedAt.Unix(),
		UpdatedAt:      ann.UpdatedAt.Unix(),
	}
}

// diffAnnotationToProto converts a store DiffAnnotation to a proto message.
func diffAnnotationToProto(ann store.DiffAnnotation) *DiffAnnotationProto {
	return &DiffAnnotationProto{
		Id:             ann.ID,
		AnnotationId:   ann.AnnotationID,
		MessageId:      ann.MessageID,
		AnnotationType: ann.AnnotationType,
		Scope:          ann.Scope,
		FilePath:       ann.FilePath,
		LineStart:      int32(ann.LineStart),
		LineEnd:        int32(ann.LineEnd),
		Side:           ann.Side,
		Text:           ann.Text,
		SuggestedCode:  ann.SuggestedCode,
		OriginalCode:   ann.OriginalCode,
		CreatedAt:      ann.CreatedAt.Unix(),
		UpdatedAt:      ann.UpdatedAt.Unix(),
	}
}
