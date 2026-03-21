package subtraterpc

import (
	"context"
	"database/sql"
	"errors"

	"github.com/roasbeef/subtrate/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// maxTextLen is the maximum length for annotation text fields.
	maxTextLen = 65536

	// maxPathLen is the maximum length for file path fields.
	maxPathLen = 4096
)

// validPlanAnnotationTypes are the allowed annotation type values for
// plan annotations.
var validPlanAnnotationTypes = map[string]bool{
	"DELETION": true, "INSERTION": true, "REPLACEMENT": true,
	"COMMENT": true, "GLOBAL_COMMENT": true,
}

// validDiffAnnotationTypes are the allowed annotation type values for
// diff annotations.
var validDiffAnnotationTypes = map[string]bool{
	"comment": true, "suggestion": true, "concern": true,
}

// validateTextLen checks that a text field does not exceed the maximum
// allowed length.
func validateTextLen(field, value string) error {
	if len(value) > maxTextLen {
		return status.Errorf(
			codes.InvalidArgument,
			"%s exceeds maximum length of %d", field, maxTextLen,
		)
	}
	return nil
}

// validatePathLen checks that a file path field does not exceed the
// maximum allowed length.
func validatePathLen(value string) error {
	if len(value) > maxPathLen {
		return status.Errorf(
			codes.InvalidArgument,
			"file_path exceeds maximum length of %d", maxPathLen,
		)
	}
	return nil
}

// isNotFound returns true if the error indicates a missing row.
func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

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
	if req.StartOffset < 0 || req.EndOffset < 0 {
		return nil, status.Error(
			codes.InvalidArgument, "offsets must be non-negative",
		)
	}
	if req.EndOffset < req.StartOffset {
		return nil, status.Error(
			codes.InvalidArgument,
			"end_offset must be >= start_offset",
		)
	}
	if !validPlanAnnotationTypes[req.AnnotationType] {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid annotation_type: %q", req.AnnotationType,
		)
	}
	for _, check := range []struct{ field, val string }{
		{"text", req.Text},
		{"original_text", req.OriginalText},
	} {
		if err := validateTextLen(check.field, check.val); err != nil {
			return nil, err
		}
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
		if isNotFound(err) {
			return nil, status.Errorf(
				codes.NotFound,
				"plan annotation not found: %s",
				req.AnnotationId,
			)
		}
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
		return nil, status.Errorf(
			codes.Internal,
			"failed to delete plan annotation: %v", err,
		)
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
	if req.LineStart < 0 || req.LineEnd < 0 {
		return nil, status.Error(
			codes.InvalidArgument, "line numbers must be non-negative",
		)
	}
	if req.LineEnd < req.LineStart {
		return nil, status.Error(
			codes.InvalidArgument,
			"line_end must be >= line_start",
		)
	}
	if !validDiffAnnotationTypes[req.AnnotationType] {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"invalid annotation_type: %q", req.AnnotationType,
		)
	}
	if err := validatePathLen(req.FilePath); err != nil {
		return nil, err
	}
	for _, check := range []struct{ field, val string }{
		{"text", req.Text},
		{"suggested_code", req.SuggestedCode},
		{"original_code", req.OriginalCode},
	} {
		if err := validateTextLen(check.field, check.val); err != nil {
			return nil, err
		}
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
		if isNotFound(err) {
			return nil, status.Errorf(
				codes.NotFound,
				"diff annotation not found: %s",
				req.AnnotationId,
			)
		}
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
		return nil, status.Errorf(
			codes.Internal,
			"failed to delete diff annotation: %v", err,
		)
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
