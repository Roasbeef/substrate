// Unit tests for reviews API functions.

import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import {
  fetchReviews,
  fetchReview,
  fetchReviewIssues,
  createReview,
  resubmitReview,
  cancelReview,
  updateIssueStatus,
} from '@/api/reviews.js';

describe('reviews API', () => {
  describe('fetchReviews', () => {
    it('should fetch all reviews', async () => {
      const reviews = await fetchReviews();

      expect(Array.isArray(reviews)).toBe(true);
      expect(reviews.length).toBeGreaterThan(0);
      expect(reviews[0]).toHaveProperty('review_id');
      expect(reviews[0]).toHaveProperty('branch');
      expect(reviews[0]).toHaveProperty('state');
    });

    it('should filter by state', async () => {
      server.use(
        http.get('/api/v1/reviews', ({ request }) => {
          const url = new URL(request.url);
          const state = url.searchParams.get('state');
          return HttpResponse.json({
            reviews: [
              {
                review_id: 'r1',
                branch: 'test',
                state: state ?? 'under_review',
                review_type: 'full',
                created_at: '1000',
              },
            ],
          });
        }),
      );

      const reviews = await fetchReviews({ state: 'approved' });

      expect(reviews).toHaveLength(1);
      expect(reviews[0].state).toBe('approved');
    });

    it('should handle empty response', async () => {
      server.use(
        http.get('/api/v1/reviews', () => {
          return HttpResponse.json({ reviews: [] });
        }),
      );

      const reviews = await fetchReviews();

      expect(reviews).toHaveLength(0);
    });

    it('should handle missing reviews field', async () => {
      server.use(
        http.get('/api/v1/reviews', () => {
          return HttpResponse.json({});
        }),
      );

      const reviews = await fetchReviews();

      expect(reviews).toHaveLength(0);
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchReviews({}, controller.signal)).rejects.toThrow();
    });

    it('should build query string with all options', async () => {
      server.use(
        http.get('/api/v1/reviews', ({ request }) => {
          const url = new URL(request.url);
          expect(url.searchParams.get('state')).toBe('approved');
          expect(url.searchParams.get('requester_id')).toBe('1');
          expect(url.searchParams.get('limit')).toBe('10');
          expect(url.searchParams.get('offset')).toBe('5');
          return HttpResponse.json({ reviews: [] });
        }),
      );

      await fetchReviews({
        state: 'approved',
        requesterId: 1,
        limit: 10,
        offset: 5,
      });
    });
  });

  describe('fetchReview', () => {
    it('should fetch a single review by ID', async () => {
      const review = await fetchReview('abc123');

      expect(review.review_id).toBe('abc123');
      expect(review.branch).toBeDefined();
      expect(review.state).toBeDefined();
      expect(review.review_type).toBeDefined();
    });

    it('should handle 404 for non-existent review', async () => {
      server.use(
        http.get('/api/v1/reviews/nonexistent', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Review not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(fetchReview('nonexistent')).rejects.toThrow();
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(fetchReview('abc123', controller.signal)).rejects.toThrow();
    });

    it('should parse gateway response fields correctly', async () => {
      server.use(
        http.get('/api/v1/reviews/test-id', () => {
          return HttpResponse.json({
            review_id: 'test-id',
            thread_id: 'thread-99',
            state: 'changes_requested',
            branch: 'feature/test',
            base_branch: 'main',
            review_type: 'security',
            iterations: 3,
            open_issues: '7',
          });
        }),
      );

      const review = await fetchReview('test-id');

      expect(review.thread_id).toBe('thread-99');
      expect(review.state).toBe('changes_requested');
      expect(review.base_branch).toBe('main');
      expect(review.review_type).toBe('security');
      expect(review.iterations).toBe(3);
      expect(review.open_issues).toBe(7);
    });
  });

  describe('createReview', () => {
    it('should create a new review', async () => {
      const response = await createReview({
        branch: 'feature/new',
        commit_sha: 'abc123',
        repo_path: '/path/to/repo',
        requester_id: 1,
      });

      expect(response.review_id).toBeDefined();
      expect(response.state).toBe('under_review');
    });

    it('should handle errors', async () => {
      server.use(
        http.post('/api/v1/reviews', () => {
          return HttpResponse.json(
            { error: { code: 'invalid_argument', message: 'branch required' } },
            { status: 400 },
          );
        }),
      );

      await expect(
        createReview({
          branch: '',
          commit_sha: '',
          repo_path: '',
          requester_id: 0,
        }),
      ).rejects.toThrow();
    });
  });

  describe('resubmitReview', () => {
    it('should resubmit a review', async () => {
      const response = await resubmitReview('abc123', 'newcommit');

      expect(response.review_id).toBe('abc123');
      expect(response.state).toBe('under_review');
    });
  });

  describe('cancelReview', () => {
    it('should cancel a review', async () => {
      await expect(cancelReview('abc123')).resolves.toBeUndefined();
    });

    it('should cancel with reason', async () => {
      await expect(cancelReview('abc123', 'no longer needed')).resolves.toBeUndefined();
    });
  });

  describe('fetchReviewIssues', () => {
    it('should fetch issues for a review', async () => {
      const issues = await fetchReviewIssues('abc123');

      expect(Array.isArray(issues)).toBe(true);
      expect(issues.length).toBeGreaterThan(0);
      expect(issues[0]).toHaveProperty('id');
      expect(issues[0]).toHaveProperty('severity');
      expect(issues[0]).toHaveProperty('title');
      expect(issues[0]).toHaveProperty('status');
    });

    it('should return empty array for review with no issues', async () => {
      const issues = await fetchReviewIssues('def456');

      expect(issues).toHaveLength(0);
    });

    it('should handle missing issues field', async () => {
      server.use(
        http.get('/api/v1/reviews/test/issues', () => {
          return HttpResponse.json({});
        }),
      );

      const issues = await fetchReviewIssues('test');

      expect(issues).toHaveLength(0);
    });

    it('should parse all issue fields correctly', async () => {
      server.use(
        http.get('/api/v1/reviews/full-test/issues', () => {
          return HttpResponse.json({
            issues: [
              {
                id: '42',
                review_id: 'full-test',
                iteration_num: 2,
                issue_type: 'security',
                severity: 'critical',
                file_path: 'main.go',
                line_start: 10,
                line_end: 20,
                title: 'SQL injection',
                description: 'User input not sanitized',
                code_snippet: 'db.Query(userInput)',
                suggestion: 'Use parameterized queries',
                claude_md_ref: 'Security section',
                status: 'open',
              },
            ],
          });
        }),
      );

      const issues = await fetchReviewIssues('full-test');

      expect(issues).toHaveLength(1);
      const issue = issues[0];
      expect(issue.id).toBe(42);
      expect(issue.iteration_num).toBe(2);
      expect(issue.issue_type).toBe('security');
      expect(issue.severity).toBe('critical');
      expect(issue.file_path).toBe('main.go');
      expect(issue.line_start).toBe(10);
      expect(issue.line_end).toBe(20);
      expect(issue.code_snippet).toBe('db.Query(userInput)');
      expect(issue.suggestion).toBe('Use parameterized queries');
      expect(issue.claude_md_ref).toBe('Security section');
    });

    it('should handle abort signal', async () => {
      const controller = new AbortController();
      controller.abort();

      await expect(
        fetchReviewIssues('abc123', controller.signal),
      ).rejects.toThrow();
    });
  });

  describe('updateIssueStatus', () => {
    it('should update issue status', async () => {
      await expect(
        updateIssueStatus('abc123', 1, 'fixed'),
      ).resolves.not.toThrow();
    });

    it('should handle errors', async () => {
      server.use(
        http.patch('/api/v1/reviews/bad/issues/999', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Issue not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(
        updateIssueStatus('bad', 999, 'fixed'),
      ).rejects.toThrow();
    });
  });
});
