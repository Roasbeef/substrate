package subtraterpc

import (
	"context"
	"time"

	"github.com/roasbeef/subtrate/internal/review"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateReview creates a new code review request via the review actor.
func (s *Server) CreateReview(
	ctx context.Context, req *CreateReviewRequest,
) (*CreateReviewResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(
			codes.InvalidArgument, "repo_path is required",
		)
	}
	if req.RequesterId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "requester_id is required",
		)
	}

	// Extract target information from the oneof or deprecated fields.
	var (
		branch     string
		baseBranch string
		commitSHA  string
		prNumber   int
	)

	switch t := req.Target.(type) {
	case *CreateReviewRequest_BranchTarget:
		branch = t.BranchTarget.Branch
		baseBranch = t.BranchTarget.BaseBranch
	case *CreateReviewRequest_CommitTarget:
		commitSHA = t.CommitTarget.Sha
		branch = t.CommitTarget.Branch
	case *CreateReviewRequest_CommitRangeTarget:
		// For commit ranges, use end_sha as the commit to review.
		commitSHA = t.CommitRangeTarget.EndSha
		branch = t.CommitRangeTarget.Branch
	case *CreateReviewRequest_PrTarget:
		prNumber = int(t.PrTarget.Number)
		branch = t.PrTarget.Branch
		baseBranch = t.PrTarget.BaseBranch
	default:
		// Fall back to deprecated fields for backward compatibility.
		branch = req.Branch      //nolint:staticcheck
		baseBranch = req.BaseBranch //nolint:staticcheck
		commitSHA = req.CommitSha   //nolint:staticcheck
		prNumber = int(req.PrNumber) //nolint:staticcheck
	}

	// Validate that we have enough information.
	if branch == "" && commitSHA == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"target is required (branch, commit, or PR)",
		)
	}

	resp, err := s.askReview(ctx, review.CreateReviewMsg{
		RequesterID: req.RequesterId,
		PRNumber:    prNumber,
		Branch:      branch,
		BaseBranch:  baseBranch,
		CommitSHA:   commitSHA,
		RepoPath:    req.RepoPath,
		RemoteURL:   req.RemoteUrl,
		ReviewType:  req.ReviewType,
		Priority:    req.Priority,
		Reviewers:   req.Reviewers,
		Description: req.Description,
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	createResp, ok := resp.(review.CreateReviewResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}

	result := &CreateReviewResponse{
		ReviewId: createResp.ReviewID,
		ThreadId: createResp.ThreadID,
		State:    createResp.State,
	}
	if createResp.Error != nil {
		result.Error = createResp.Error.Error()
	}

	return result, nil
}

// ListReviews lists reviews with optional filters.
func (s *Server) ListReviews(
	ctx context.Context, req *ListReviewsProtoRequest,
) (*ListReviewsProtoResponse, error) {
	resp, err := s.askReview(ctx, review.ListReviewsMsg{
		State:       req.State,
		RequesterID: req.RequesterId,
		Limit:       int(req.Limit),
		Offset:      int(req.Offset),
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	listResp, ok := resp.(review.ListReviewsResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}
	if listResp.Error != nil {
		return nil, status.Errorf(
			codes.Internal, "list reviews: %v", listResp.Error,
		)
	}

	protos := make([]*ReviewSummaryProto, len(listResp.Reviews))
	for i, r := range listResp.Reviews {
		protos[i] = &ReviewSummaryProto{
			ReviewId:    r.ReviewID,
			ThreadId:    r.ThreadID,
			RequesterId: r.RequesterID,
			Branch:      r.Branch,
			State:       r.State,
			ReviewType:  r.ReviewType,
			CreatedAt:   r.CreatedAt,
		}
	}

	return &ListReviewsProtoResponse{Reviews: protos}, nil
}

// GetReview retrieves details for a specific review.
func (s *Server) GetReview(
	ctx context.Context, req *GetReviewProtoRequest,
) (*ReviewDetailResponse, error) {
	if req.ReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "review_id is required",
		)
	}

	resp, err := s.askReview(ctx, review.GetReviewMsg{
		ReviewID: req.ReviewId,
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	getResp, ok := resp.(review.GetReviewResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}

	result := &ReviewDetailResponse{
		ReviewId:   getResp.ReviewID,
		ThreadId:   getResp.ThreadID,
		State:      getResp.State,
		Branch:     getResp.Branch,
		BaseBranch: getResp.BaseBranch,
		ReviewType: getResp.ReviewType,
		Iterations: int32(getResp.Iterations),
		OpenIssues: getResp.OpenIssues,
	}
	if getResp.Error != nil {
		result.Error = getResp.Error.Error()
	}

	return result, nil
}

// ResubmitReview re-requests review after the author has pushed changes.
func (s *Server) ResubmitReview(
	ctx context.Context, req *ResubmitReviewRequest,
) (*CreateReviewResponse, error) {
	if req.ReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "review_id is required",
		)
	}
	if req.CommitSha == "" {
		return nil, status.Error(
			codes.InvalidArgument, "commit_sha is required",
		)
	}

	resp, err := s.askReview(ctx, review.ResubmitMsg{
		ReviewID:  req.ReviewId,
		CommitSHA: req.CommitSha,
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	resubResp, ok := resp.(review.ResubmitResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}

	result := &CreateReviewResponse{
		ReviewId: resubResp.ReviewID,
		State:    resubResp.NewState,
	}
	if resubResp.Error != nil {
		result.Error = resubResp.Error.Error()
	}

	return result, nil
}

// CancelReview cancels an active review.
func (s *Server) CancelReview(
	ctx context.Context, req *CancelReviewProtoRequest,
) (*CancelReviewProtoResponse, error) {
	if req.ReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "review_id is required",
		)
	}

	resp, err := s.askReview(ctx, review.CancelReviewMsg{
		ReviewID: req.ReviewId,
		Reason:   req.Reason,
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	cancelResp, ok := resp.(review.CancelReviewResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}

	result := &CancelReviewProtoResponse{}
	if cancelResp.Error != nil {
		result.Error = cancelResp.Error.Error()
	}

	return result, nil
}

// ListReviewIssues lists issues for a specific review.
func (s *Server) ListReviewIssues(
	ctx context.Context, req *ListReviewIssuesRequest,
) (*ListReviewIssuesResponse, error) {
	if req.ReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "review_id is required",
		)
	}

	resp, err := s.askReview(ctx, review.GetIssuesMsg{
		ReviewID: req.ReviewId,
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	issuesResp, ok := resp.(review.GetIssuesResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}
	if issuesResp.Error != nil {
		return nil, status.Errorf(
			codes.Internal,
			"list review issues: %v", issuesResp.Error,
		)
	}

	protos := make([]*ReviewIssueProto, len(issuesResp.Issues))
	for i, issue := range issuesResp.Issues {
		protos[i] = &ReviewIssueProto{
			Id:           issue.ID,
			ReviewId:     issue.ReviewID,
			IterationNum: int32(issue.IterationNum),
			IssueType:    issue.IssueType,
			Severity:     issue.Severity,
			FilePath:     issue.FilePath,
			LineStart:    int32(issue.LineStart),
			Title:        issue.Title,
			Status:       issue.Status,
		}
	}

	return &ListReviewIssuesResponse{Issues: protos}, nil
}

// UpdateIssueStatus updates the status of a review issue.
func (s *Server) UpdateIssueStatus(
	ctx context.Context, req *UpdateIssueStatusRequest,
) (*UpdateIssueStatusResponse, error) {
	if req.ReviewId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "review_id is required",
		)
	}
	if req.IssueId == 0 {
		return nil, status.Error(
			codes.InvalidArgument, "issue_id is required",
		)
	}
	if req.Status == "" {
		return nil, status.Error(
			codes.InvalidArgument, "status is required",
		)
	}

	resp, err := s.askReview(ctx, review.UpdateIssueMsg{
		ReviewID: req.ReviewId,
		IssueID:  req.IssueId,
		Status:   req.Status,
	})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "review actor error: %v", err,
		)
	}

	updateResp, ok := resp.(review.UpdateIssueResp)
	if !ok {
		return nil, status.Error(
			codes.Internal, "unexpected response type",
		)
	}

	result := &UpdateIssueStatusResponse{}
	if updateResp.Error != nil {
		result.Error = updateResp.Error.Error()
	}

	return result, nil
}

// askReview sends a request to the review actor and awaits the response.
func (s *Server) askReview(
	ctx context.Context, msg review.ReviewRequest,
) (review.ReviewResponse, error) {
	askCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	future := s.reviewRef.Ask(askCtx, msg)
	result := future.Await(askCtx)

	return result.Unpack()
}
