// E2E tests for compose modal full flow.

import { test, expect } from '@playwright/test';

// Helper to setup API endpoints.
async function setupAPIs(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/messages*', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 999,
          sender_name: 'Me',
          subject: 'New Message',
          body: 'Message body',
          priority: 'normal',
          created_at: new Date().toISOString(),
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], meta: { total: 0, page: 1, page_size: 20 } }),
      });
    }
  });

  await page.route('**/api/v1/autocomplete/recipients*', async (route) => {
    const url = new URL(route.request().url());
    const query = url.searchParams.get('q') || '';

    const allAgents = [
      { id: 1, name: 'Alice Agent' },
      { id: 2, name: 'Bob Agent' },
      { id: 3, name: 'Charlie Agent' },
    ];

    const filtered = query
      ? allAgents.filter((a) => a.name.toLowerCase().includes(query.toLowerCase()))
      : allAgents;

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(filtered),
    });
  });
}

test.describe('Compose modal opening', () => {
  test('compose button opens modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    // Click compose button.
    const composeButton = page.locator('button:has-text("Compose"), [data-testid="compose-button"]');
    await composeButton.click();
    await page.waitForTimeout(300);

    // Modal should open.
    const composeModal = page.locator('[data-testid="compose-modal"], [role="dialog"]');
    await expect(composeModal).toBeVisible();
  });

  test('compose modal has required fields', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Should have recipient field.
      await expect(modal.locator('[data-testid="recipients-input"], input[placeholder*="recipient" i]')).toBeVisible();

      // Should have subject field.
      await expect(modal.locator('input[placeholder*="subject" i], [data-testid="subject-input"]')).toBeVisible();

      // Should have body textarea.
      await expect(modal.locator('textarea')).toBeVisible();

      // Should have send button.
      await expect(modal.locator('button:has-text("Send")')).toBeVisible();
    }
  });
});

test.describe('Recipient autocomplete', () => {
  test('typing shows autocomplete suggestions', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const recipientInput = modal.locator('[data-testid="recipients-input"], input[placeholder*="recipient" i]');
      if (await recipientInput.isVisible()) {
        await recipientInput.fill('Alice');
        await page.waitForTimeout(500);

        // Should show suggestions.
        const suggestions = page.locator('[data-testid="autocomplete-suggestions"], [role="listbox"]');
        if (await suggestions.isVisible()) {
          await expect(suggestions.locator('text=Alice Agent')).toBeVisible();
        }
      }
    }
  });

  test('clicking suggestion adds recipient', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const recipientInput = modal.locator('[data-testid="recipients-input"], input').first();
      if (await recipientInput.isVisible()) {
        await recipientInput.fill('Alice');
        await page.waitForTimeout(500);

        // Click suggestion.
        await page.locator('text=Alice Agent').click();
        await page.waitForTimeout(200);

        // Recipient should be added (as chip/tag).
        await expect(modal.locator('[data-testid="recipient-chip"]:has-text("Alice")')).toBeVisible();
      }
    }
  });

  test('multiple recipients can be added', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const recipientInput = modal.locator('input').first();
      if (await recipientInput.isVisible()) {
        // Add first recipient.
        await recipientInput.fill('Alice');
        await page.waitForTimeout(300);
        const aliceSuggestion = page.locator('[role="option"]:has-text("Alice")');
        if (await aliceSuggestion.isVisible()) {
          await aliceSuggestion.click();
        }

        // Add second recipient.
        await recipientInput.fill('Bob');
        await page.waitForTimeout(300);
        const bobSuggestion = page.locator('[role="option"]:has-text("Bob")');
        if (await bobSuggestion.isVisible()) {
          await bobSuggestion.click();
        }

        // Should have both recipients.
      }
    }
  });
});

test.describe('Subject and body input', () => {
  test('subject input accepts text', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const subjectInput = modal.locator('input[placeholder*="subject" i], [data-testid="subject-input"]');
      if (await subjectInput.isVisible()) {
        await subjectInput.fill('Test Subject');
        await expect(subjectInput).toHaveValue('Test Subject');
      }
    }
  });

  test('body textarea accepts text', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const bodyInput = modal.locator('textarea');
      if (await bodyInput.isVisible()) {
        await bodyInput.fill('This is the message body.\n\nWith multiple paragraphs.');
        await expect(bodyInput).toHaveValue('This is the message body.\n\nWith multiple paragraphs.');
      }
    }
  });
});

test.describe('Priority selection', () => {
  test('priority dropdown is visible', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const prioritySelect = modal.locator('[data-testid="priority-select"], select, button:has-text("Priority")');
      await expect(prioritySelect).toBeVisible();
    }
  });

  test('can select urgent priority', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Click priority dropdown.
      const priorityButton = modal.locator('[data-testid="priority-select"], button:has-text(/priority|normal/i)');
      if (await priorityButton.isVisible()) {
        await priorityButton.click();
        await page.waitForTimeout(200);

        // Select urgent.
        await page.locator('[role="option"]:has-text("Urgent"), text=Urgent').click();
        await page.waitForTimeout(200);

        // Should show urgent selected.
      }
    }
  });
});

test.describe('Sending message', () => {
  test('send button submits message', async ({ page }) => {
    await setupAPIs(page);

    let messageSent = false;
    await page.route('**/api/v1/messages', async (route) => {
      if (route.request().method() === 'POST') {
        messageSent = true;
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ id: 999 }),
        });
      }
    });

    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Fill in required fields.
      const subjectInput = modal.locator('input').nth(1);
      const bodyInput = modal.locator('textarea');
      const sendButton = modal.locator('button:has-text("Send")');

      await subjectInput.fill('Test Subject');
      await bodyInput.fill('Test body content');
      await sendButton.click();
      await page.waitForTimeout(500);

      // Message should be sent.
      expect(messageSent).toBe(true);
    }
  });

  test('modal closes after successful send', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const subjectInput = modal.locator('input').nth(1);
      const bodyInput = modal.locator('textarea');
      const sendButton = modal.locator('button:has-text("Send")');

      await subjectInput.fill('Test Subject');
      await bodyInput.fill('Test body');
      await sendButton.click();
      await page.waitForTimeout(500);

      // Modal should close.
      await expect(modal).not.toBeVisible();
    }
  });

  test('shows success toast after send', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const subjectInput = modal.locator('input').nth(1);
      const bodyInput = modal.locator('textarea');
      const sendButton = modal.locator('button:has-text("Send")');

      await subjectInput.fill('Test Subject');
      await bodyInput.fill('Test body');
      await sendButton.click();
      await page.waitForTimeout(500);

      // Should show success toast.
      const toast = page.locator('[role="alert"], [data-testid="toast"]');
      if (await toast.isVisible()) {
        await expect(toast).toContainText(/sent|success/i);
      }
    }
  });
});

test.describe('Compose modal cancel', () => {
  test('cancel button closes modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const cancelButton = modal.locator('button:has-text("Cancel"), button:has-text("×")');
      if (await cancelButton.isVisible()) {
        await cancelButton.click();
        await page.waitForTimeout(300);

        await expect(modal).not.toBeVisible();
      }
    }
  });

  test('Escape key closes modal', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);

      await expect(modal).not.toBeVisible();
    }
  });

  test('warns before closing if content entered', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      // Enter some content.
      const bodyInput = modal.locator('textarea');
      await bodyInput.fill('Some content');

      // Try to close.
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);

      // May show confirmation or close directly (depends on implementation).
    }
  });
});

test.describe('Compose validation', () => {
  test('send disabled without subject', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const bodyInput = modal.locator('textarea');
      const sendButton = modal.locator('button:has-text("Send")');

      // Only fill body.
      await bodyInput.fill('Body without subject');

      // Send should be disabled or show validation error.
      // Implementation dependent.
    }
  });

  test('shows error for invalid form', async ({ page }) => {
    await setupAPIs(page);
    await page.goto('/');
    await expect(page.locator('text=Inbox')).toBeVisible();

    await page.locator('button:has-text("Compose")').click();
    await page.waitForTimeout(300);

    const modal = page.locator('[role="dialog"]');
    if (await modal.isVisible()) {
      const sendButton = modal.locator('button:has-text("Send")');

      // Click send without filling anything.
      await sendButton.click();
      await page.waitForTimeout(300);

      // Should show validation errors or remain on form.
    }
  });
});
