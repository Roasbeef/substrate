// E2E tests for reviews list page.

import { test, expect } from '@playwright/test';

// Helper to setup mock API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  const reviews = [
    {
      review_id: 'abc123',
      thread_id: 'thread-1',
      requester_id: '1',
      branch: 'feature/add-reviews',
      state: 'under_review',
      review_type: 'full',
      created_at: String(Math.floor(Date.now() / 1000) - 3600),
    },
    {
      review_id: 'def456',
      thread_id: 'thread-2',
      requester_id: '1',
      branch: 'fix/null-pointer',
      state: 'approved',
      review_type: 'incremental',
      created_at: String(Math.floor(Date.now() / 1000) - 7200),
    },
    {
      review_id: 'ghi789',
      thread_id: 'thread-3',
      requester_id: '2',
      branch: 'security/audit',
      state: 'changes_requested',
      review_type: 'security',
      created_at: String(Math.floor(Date.now() / 1000) - 86400),
    },
  ];

  const issues = [
    {
      id: '1',
      review_id: 'abc123',
      iteration_num: 1,
      issue_type: 'bug',
      severity: 'major',
      file_path: 'internal/review/service.go',
      line_start: 42,
      line_end: 50,
      title: 'Missing nil check',
      description: 'Pointer could be nil.',
      code_snippet: 'review := s.reviews[id]',
      suggestion: 'Add nil check.',
      status: 'open',
    },
    {
      id: '2',
      review_id: 'abc123',
      iteration_num: 1,
      issue_type: 'style',
      severity: 'suggestion',
      file_path: 'internal/review/fsm.go',
      line_start: 15,
      title: 'Missing comment',
      description: 'Add function comment.',
      status: 'fixed',
    },
  ];

  // Reviews list.
  await page.route('**/api/v1/reviews', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          review_id: 'new-review',
          state: 'under_review',
        }),
      });
    } else {
      const url = new URL(route.request().url());
      const stateFilter = url.searchParams.get('state');
      const filtered = stateFilter
        ? reviews.filter((r) => r.state === stateFilter)
        : reviews;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ reviews: filtered }),
      });
    }
  });

  // Review detail.
  await page.route('**/api/v1/reviews/*/issues', async (route) => {
    const url = route.request().url();
    const match = url.match(/reviews\/([^/]+)\/issues/);
    const reviewId = match ? match[1] : '';
    const filteredIssues = issues.filter((i) => i.review_id === reviewId);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ issues: filteredIssues }),
    });
  });

  await page.route('**/api/v1/reviews/*', async (route) => {
    const url = route.request().url();
    if (url.includes('/resubmit')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ review_id: 'abc123', state: 'under_review' }),
      });
      return;
    }

    const match = url.match(/reviews\/([^/?]+)$/);
    const reviewId = match ? match[1] : '';
    const review = reviews.find((r) => r.review_id === reviewId);

    if (route.request().method() === 'DELETE') {
      await route.fulfill({ status: 204, body: '' });
      return;
    }

    if (!review) {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'not_found', message: 'Review not found' },
        }),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        review_id: review.review_id,
        thread_id: review.thread_id,
        state: review.state,
        branch: review.branch,
        base_branch: 'main',
        review_type: review.review_type,
        iterations: 1,
        open_issues: '1',
      }),
    });
  });

  // Stub other endpoints.
  await page.route('**/api/v1/agents*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        agents: [{ id: 1, name: 'Agent1', status: 'active', seconds_since_heartbeat: 0, last_active_at: new Date().toISOString() }],
        counts: { active: 1, busy: 0, idle: 0, offline: 0 },
      }),
    });
  });

  await page.route('**/api/v1/topics', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
    });
  });

  return { reviews, issues };
}

test.describe('Reviews page loading', () => {
  test('navigates to reviews page', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews');
    await expect(page.locator('text=Code Reviews')).toBeVisible();
  });

});

test.describe('Reviews filtering', () => {
  test('shows filter tabs', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews');

    await expect(page.locator('button:has-text("All")')).toBeVisible();
    await expect(page.locator('button:has-text("In Review")')).toBeVisible();
    await expect(page.locator('button:has-text("Approved")')).toBeVisible();
  });
});

test.describe('Review detail view', () => {
  test('detail page shows metadata', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    // Should show metadata.
    await expect(page.locator('text=Type')).toBeVisible();
    await expect(page.locator('text=Iterations')).toBeVisible();
    await expect(page.locator('text=Open Issues')).toBeVisible();
  });

  test('detail page shows base branch', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    await expect(page.locator('text=main')).toBeVisible();
  });

  test('detail page shows issues', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    // Should show issues.
    await expect(page.locator('text=Missing nil check')).toBeVisible();
    await expect(page.locator('text=Missing comment')).toBeVisible();
  });

  test('issue cards show severity', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    await expect(page.locator('text=major')).toBeVisible();
    await expect(page.locator('text=suggestion')).toBeVisible();
  });

  test('issue cards show file paths', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    await expect(
      page.locator('text=internal/review/service.go:42-50'),
    ).toBeVisible();
  });

  test('back button navigates to list', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    await page.locator('button:has-text("Back to Reviews")').click();
    await page.waitForURL('**/reviews');

    // Should be back on list.
    await expect(page.locator('text=Code Reviews')).toBeVisible();
  });

  test('issue details expand on click', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    // Click show details on first issue.
    const showDetails = page.locator('button:has-text("Show details")').first();
    await showDetails.click();

    // Description should be visible.
    await expect(page.locator('text=Description').first()).toBeVisible();
    await expect(page.locator('text=Pointer could be nil.')).toBeVisible();
  });

  test('cancel review button visible for active reviews', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/reviews/abc123');
    await page.waitForTimeout(500);

    await expect(
      page.locator('button:has-text("Cancel Review")'),
    ).toBeVisible();
  });
});

test.describe('Reviews sidebar navigation', () => {
  test('reviews link in sidebar', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/inbox');
    await page.waitForTimeout(500);

    // Find and click the Reviews nav link.
    const reviewsLink = page.locator('a:has-text("Reviews")');
    await expect(reviewsLink).toBeVisible();
    await reviewsLink.click();
    await page.waitForURL('**/reviews');

    await expect(page.locator('text=Code Reviews')).toBeVisible();
  });
});

test.describe('Empty state', () => {
  test('shows empty state when no reviews', async ({ page }) => {
    // Override to return empty reviews.
    await page.route('**/api/v1/reviews', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ reviews: [] }),
      });
    });

    await page.route('**/api/v1/agents*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          agents: [],
          counts: { active: 0, busy: 0, idle: 0, offline: 0 },
        }),
      });
    });

    await page.route('**/api/v1/topics', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [] }),
      });
    });

    await page.goto('/reviews');
    await page.waitForTimeout(500);

    await expect(
      page.getByRole('heading', { name: 'No reviews' }),
    ).toBeVisible();
    await expect(
      page.locator('text=substrate review request'),
    ).toBeVisible();
  });
});
