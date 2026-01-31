// Subtrate UI JavaScript

// =============================================================================
// Browser Desktop Notifications
// =============================================================================

// Notification state - use var to allow re-declaration if script loads twice.
// Guard against re-initialization to preserve state across HTMX navigations.
if (typeof window.substrateInitialized === 'undefined') {
    window.substrateInitialized = true;
    var notificationsEnabled = false;
    var lastNotificationTime = 0;
    var NOTIFICATION_COOLDOWN = 5000; // 5 seconds between notifications.
}

// Request notification permission on page load.
function initNotifications() {
    if (!('Notification' in window)) {
        console.log('Browser does not support notifications');
        return;
    }

    if (Notification.permission === 'granted') {
        notificationsEnabled = true;
        console.log('Notifications already enabled');
    } else if (Notification.permission !== 'denied') {
        // Show a prompt to enable notifications.
        showNotificationPrompt();
    }
}

// Show a prompt asking user to enable notifications.
function showNotificationPrompt() {
    const prompt = document.createElement('div');
    prompt.id = 'notification-prompt';
    prompt.className = 'fixed bottom-20 right-4 bg-white rounded-lg shadow-xl border border-gray-200 p-4 z-50 max-w-sm';
    prompt.innerHTML = `
        <div class="flex items-start gap-3">
            <div class="w-10 h-10 bg-blue-100 rounded-full flex items-center justify-center flex-shrink-0">
                <svg class="w-5 h-5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
                </svg>
            </div>
            <div class="flex-1">
                <h4 class="font-medium text-gray-900 text-sm">Enable Notifications</h4>
                <p class="text-xs text-gray-500 mt-1">Get notified when new messages arrive from your agents.</p>
                <div class="flex gap-2 mt-3">
                    <button onclick="requestNotificationPermission()" class="px-3 py-1.5 bg-blue-600 text-white text-xs font-medium rounded hover:bg-blue-700">
                        Enable
                    </button>
                    <button onclick="dismissNotificationPrompt()" class="px-3 py-1.5 text-gray-600 text-xs hover:bg-gray-100 rounded">
                        Not now
                    </button>
                </div>
            </div>
            <button onclick="dismissNotificationPrompt()" class="text-gray-400 hover:text-gray-600">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
            </button>
        </div>
    `;
    document.body.appendChild(prompt);
}

// Dismiss the notification prompt.
function dismissNotificationPrompt() {
    const prompt = document.getElementById('notification-prompt');
    if (prompt) {
        prompt.remove();
    }
    // Remember dismissal in localStorage.
    localStorage.setItem('notifications-prompt-dismissed', 'true');
}

// Request permission from the browser.
function requestNotificationPermission() {
    dismissNotificationPrompt();

    Notification.requestPermission().then(permission => {
        if (permission === 'granted') {
            notificationsEnabled = true;
            showToast('Notifications enabled!', 'success');
            // Show a test notification.
            showBrowserNotification('Notifications Enabled', 'You will now receive alerts for new messages.', 'info');
        } else {
            showToast('Notifications were denied', 'warning');
        }
    });
}

// Show a browser desktop notification.
function showBrowserNotification(title, body, type = 'message', data = {}) {
    console.log('showBrowserNotification called:', title, 'enabled:', notificationsEnabled, 'permission:', Notification.permission);

    if (!notificationsEnabled) {
        console.log('Notifications disabled, skipping');
        return;
    }

    // Respect cooldown to avoid notification spam.
    const now = Date.now();
    if (now - lastNotificationTime < NOTIFICATION_COOLDOWN) {
        console.log('Notification in cooldown, skipping');
        return;
    }
    lastNotificationTime = now;

    // Note: We show notifications even when page is visible so users
    // can see them while working in the inbox.

    try {
        console.log('Creating notification:', title, body);
        const notification = new Notification(title, {
            body: body,
            icon: '/static/icons/message.svg',
            requireInteraction: type === 'urgent'
        });
        console.log('Notification created successfully');

        // Handle click - focus window and navigate if needed.
        notification.onclick = function(event) {
            event.preventDefault();
            window.focus();
            if (data.url) {
                window.location.href = data.url;
            }
            notification.close();
        };

        // Auto-close after 10 seconds for non-urgent.
        if (type !== 'urgent') {
            setTimeout(() => notification.close(), 10000);
        }
    } catch (err) {
        console.error('Failed to create notification:', err);
    }
}

// Track known message IDs to prevent duplicate refreshes.
// Use var to allow re-declaration if script loads twice.
if (typeof knownMessageIds === 'undefined') {
    var knownMessageIds = new Set();
}

// Track message IDs we've already sent notifications for (separate from DOM).
// This prevents the race condition where message is added to DOM before the
// new-message SSE event arrives, causing us to skip the detailed notification.
if (typeof notifiedMessageIds === 'undefined') {
    var notifiedMessageIds = new Set();
}

// Initialize known message IDs from the current DOM.
function initKnownMessageIds() {
    document.querySelectorAll('[id^="message-"]').forEach(el => {
        const match = el.id.match(/^message-(\d+)$/);
        if (match) {
            knownMessageIds.add(parseInt(match[1], 10));
        }
    });
}

// Track SSE connection state.
// Use var to allow re-declaration if script loads twice.
if (typeof sseConnected === 'undefined') {
    var sseConnected = false;
    var sseReconnectCount = 0;
    var lastUnreadCount = -1;
    var lastNewMessageEventTime = 0;  // Track when we last handled new-message event.
    var notificationEventSource = null;  // Vanilla EventSource for notifications.
}

// Listen for SSE events and show notifications.
function setupSSENotifications() {
    // Initialize known IDs on page load.
    initKnownMessageIds();

    // Get initial unread count from DOM.
    const unreadEl = document.getElementById('stat-unread');
    if (unreadEl) {
        lastUnreadCount = parseInt(unreadEl.textContent, 10) || 0;
    }

    // Re-initialize after HTMX swaps (e.g., after message list refresh).
    document.body.addEventListener('htmx:afterSwap', function(evt) {
        if (evt.detail.target && evt.detail.target.id === 'message-list') {
            initKnownMessageIds();
        }
    });

    // Handle SSE connection open - refresh messages on reconnect.
    document.body.addEventListener('htmx:sseOpen', function(evt) {
        console.log('SSE connection opened');
        if (sseConnected) {
            // This is a reconnection - we may have missed messages.
            sseReconnectCount++;
            console.log('SSE reconnected (count:', sseReconnectCount, '), triggering refresh');
            const messageList = document.getElementById('message-list');
            if (messageList) {
                htmx.trigger(messageList, 'refresh');
            }
        }
        sseConnected = true;
    });

    // Handle SSE connection errors.
    document.body.addEventListener('htmx:sseError', function(evt) {
        console.log('SSE connection error, will reconnect');
        sseConnected = false;
    });

    // Set up a vanilla EventSource for notification events.
    // HTMX SSE extension doesn't dispatch htmx:sseMessage for all events,
    // so we use a separate EventSource to reliably receive new-message events.
    setupNotificationEventSource();

    // Listen for HTMX SSE messages for unread-count (htmx:sseBeforeMessage).
    document.body.addEventListener('htmx:sseBeforeMessage', function(evt) {
        const eventType = evt.detail.type;
        const rawData = evt.detail.data;

        // Debug logging.
        console.log('SSE event received:', eventType, 'data:', rawData, 'notificationsEnabled:', notificationsEnabled);

        // If unread count increased, trigger a refresh (fallback for missed new-message events).
        // Note: The htmx:sseMessage event fires BEFORE the sse-swap happens, so rawData is the
        // new count before it's swapped into the DOM. We compare against lastUnreadCount which
        // was set from the previous event or page load.
        if (eventType === 'unread-count') {
            const newCount = parseInt(rawData, 10);
            if (!isNaN(newCount) && lastUnreadCount >= 0 && newCount > lastUnreadCount) {
                // Unread count increased - trigger message list refresh.
                const messageList = document.getElementById('message-list');
                if (messageList) {
                    htmx.trigger(messageList, 'refresh');
                }

                // Only show fallback notification if we haven't recently handled a new-message event.
                // The new-message event provides detailed sender/subject info, so we prefer that.
                // 500ms window accounts for SSE events arriving in same tick.
                const timeSinceNewMessage = Date.now() - lastNewMessageEventTime;
                if (timeSinceNewMessage > 500) {
                    // Show generic browser notification for new messages (fallback).
                    const newMsgCount = newCount - lastUnreadCount;
                    showBrowserNotification(
                        'New Message' + (newMsgCount > 1 ? 's' : ''),
                        newMsgCount + ' new message' + (newMsgCount > 1 ? 's' : '') + ' in your inbox',
                        'message',
                        { id: 'unread-' + newCount, url: '/inbox' }
                    );
                } else {
                    console.log('Skipping unread-count notification (new-message event handled', timeSinceNewMessage, 'ms ago)');
                }
            }
            if (!isNaN(newCount)) {
                lastUnreadCount = newCount;
            }
        }

        // Handle new-message events (JSON array of new messages).
        if (eventType === 'new-message') {
            // Mark that we're handling a new-message event (used to skip unread-count fallback).
            lastNewMessageEventTime = Date.now();

            try {
                const messages = JSON.parse(rawData);
                if (!Array.isArray(messages) || messages.length === 0) {
                    return;
                }

                // Check for genuinely new messages (not already in DOM).
                const newMessages = messages.filter(msg => !knownMessageIds.has(msg.id));

                if (newMessages.length > 0) {
                    console.log('Found', newMessages.length, 'new messages, triggering refresh');

                    // Add to known set to prevent duplicate refreshes.
                    newMessages.forEach(msg => knownMessageIds.add(msg.id));

                    // Trigger morph-based refresh of the message list.
                    const messageList = document.getElementById('message-list');
                    if (messageList) {
                        htmx.trigger(messageList, 'refresh');
                    }
                }

                // Show browser notification for messages we haven't notified about.
                // Use separate notifiedMessageIds set to handle race condition where
                // message is added to DOM (via unread-count refresh) before new-message event.
                const unnotifiedMessages = messages.filter(msg => !notifiedMessageIds.has(msg.id));

                if (unnotifiedMessages.length === 0) {
                    console.log('Already notified about all messages');
                    return;
                }

                console.log('Showing notifications for', unnotifiedMessages.length, 'messages');

                unnotifiedMessages.forEach(msg => {
                    // Mark as notified to prevent duplicates.
                    notifiedMessageIds.add(msg.id);

                    // Build notification title with sender and project.
                    let title = msg.sender;
                    if (msg.project) {
                        title += ' (' + msg.project + ')';
                    }

                    // Build body with subject and preview.
                    let body = msg.subject;
                    if (msg.preview && msg.preview !== msg.subject) {
                        body += '\n' + msg.preview.substring(0, 80);
                    }

                    showBrowserNotification(
                        title,
                        body,
                        msg.priority === 'urgent' ? 'urgent' : 'message',
                        { id: msg.id, url: '/inbox?open_thread=' + msg.thread_id, threadId: msg.thread_id }
                    );
                });

            } catch (e) {
                console.error('Failed to parse new-message data:', e);
            }
        }

        // Handle explicit notification events (for non-message notifications).
        if (eventType === 'notification') {
            try {
                const data = JSON.parse(rawData);
                showBrowserNotification(
                    data.title || 'Notification',
                    data.body || '',
                    data.priority === 'urgent' ? 'urgent' : 'info',
                    { id: data.id, url: data.url || '/inbox' }
                );
            } catch (e) {
                console.error('Failed to parse notification data:', e);
            }
        }
    });
}

// Set up a vanilla EventSource specifically for new-message notifications.
// This runs alongside the HTMX SSE connection to reliably catch new-message events.
function setupNotificationEventSource() {
    // Get agent ID from the SSE endpoint in the page (if available).
    const sseDiv = document.querySelector('[sse-connect*="/events/inbox"]');
    if (!sseDiv) {
        console.log('No SSE inbox endpoint found, skipping notification EventSource');
        return;
    }

    const sseConnect = sseDiv.getAttribute('sse-connect');
    if (!sseConnect) return;

    // Close existing connection if any.
    if (notificationEventSource) {
        notificationEventSource.close();
    }

    console.log('Setting up notification EventSource:', sseConnect);
    notificationEventSource = new EventSource(sseConnect);

    notificationEventSource.addEventListener('new-message', function(event) {
        console.log('EventSource new-message received:', event.data);
        lastNewMessageEventTime = Date.now();

        try {
            const messages = JSON.parse(event.data);
            if (!Array.isArray(messages) || messages.length === 0) {
                return;
            }

            // Check for genuinely new messages (not already in DOM).
            const newMessages = messages.filter(msg => !knownMessageIds.has(msg.id));

            if (newMessages.length > 0) {
                console.log('Found', newMessages.length, 'new messages via EventSource, triggering refresh');

                // Add to known set to prevent duplicate refreshes.
                newMessages.forEach(msg => knownMessageIds.add(msg.id));

                // Trigger morph-based refresh of the message list.
                const messageList = document.getElementById('message-list');
                if (messageList) {
                    htmx.trigger(messageList, 'refresh');
                }
            }

            // Show browser notification for messages we haven't notified about.
            const unnotifiedMessages = messages.filter(msg => !notifiedMessageIds.has(msg.id));

            if (unnotifiedMessages.length === 0) {
                console.log('Already notified about all messages');
                return;
            }

            console.log('Showing notifications for', unnotifiedMessages.length, 'messages');

            unnotifiedMessages.forEach(msg => {
                notifiedMessageIds.add(msg.id);

                let title = msg.sender;
                if (msg.project) {
                    title += ' (' + msg.project + ')';
                }

                let body = msg.subject;
                if (msg.preview && msg.preview !== msg.subject) {
                    body += '\n' + msg.preview.substring(0, 80);
                }

                showBrowserNotification(
                    title,
                    body,
                    msg.priority === 'urgent' ? 'urgent' : 'message',
                    { id: msg.id, url: '/inbox?open_thread=' + msg.thread_id, threadId: msg.thread_id }
                );
            });

        } catch (e) {
            console.error('Failed to parse new-message data:', e);
        }
    });

    notificationEventSource.onerror = function(event) {
        console.log('Notification EventSource error, will auto-reconnect');
    };
}

// Update toggle visual state (Tailwind v2 CDN doesn't support peer-checked).
function updateToggleVisual(checkbox) {
    const toggleDiv = checkbox.nextElementSibling;
    if (!toggleDiv) return;

    if (checkbox.checked) {
        toggleDiv.style.backgroundColor = '#2563eb'; // blue-600
    } else {
        toggleDiv.style.backgroundColor = '#e5e7eb'; // gray-200
    }
}

// Toggle desktop notifications from settings page.
function toggleDesktopNotifications(checkbox) {
    if (checkbox.checked) {
        // User wants to enable notifications.
        if (Notification.permission === 'granted') {
            notificationsEnabled = true;
            localStorage.setItem('notifications-enabled', 'true');
            updateToggleVisual(checkbox);
            showToast('Notifications enabled', 'success');
        } else if (Notification.permission === 'denied') {
            // Browser has blocked notifications - can't enable.
            checkbox.checked = false;
            updateToggleVisual(checkbox);
            showToast('Notifications are blocked by browser. Please enable in browser settings.', 'warning');
        } else {
            // Need to request permission.
            Notification.requestPermission().then(permission => {
                if (permission === 'granted') {
                    notificationsEnabled = true;
                    localStorage.setItem('notifications-enabled', 'true');
                    updateToggleVisual(checkbox);
                    showToast('Notifications enabled!', 'success');
                    showBrowserNotification('Notifications Enabled', 'You will now receive alerts for new messages.', 'info');
                } else {
                    checkbox.checked = false;
                    updateToggleVisual(checkbox);
                    localStorage.setItem('notifications-enabled', 'false');
                    showToast('Notification permission denied', 'warning');
                }
            });
        }
    } else {
        // User wants to disable notifications.
        notificationsEnabled = false;
        localStorage.setItem('notifications-enabled', 'false');
        updateToggleVisual(checkbox);
        showToast('Notifications disabled', 'info');
    }
}

// Test notification - sends a direct browser notification to verify setup.
function testNotification() {
    console.log('testNotification called, permission:', Notification.permission);

    if (Notification.permission === 'denied') {
        showToast('Notifications blocked by browser. Check browser settings.', 'error');
        return;
    }

    if (Notification.permission === 'default') {
        Notification.requestPermission().then(permission => {
            if (permission === 'granted') {
                sendTestNotification();
            } else {
                showToast('Notification permission denied', 'warning');
            }
        });
        return;
    }

    sendTestNotification();
}

// Actually send the test notification.
function sendTestNotification() {
    try {
        console.log('Creating test notification...');
        const notification = new Notification('Subtrate Test', {
            body: 'If you see this, notifications are working!',
            icon: '/static/icons/message.svg',
            requireInteraction: false
        });
        console.log('Test notification created:', notification);

        notification.onshow = () => console.log('Notification shown');
        notification.onerror = (e) => console.error('Notification error:', e);
        notification.onclick = () => {
            window.focus();
            notification.close();
        };

        setTimeout(() => notification.close(), 5000);
        showToast('Test notification sent!', 'success');
    } catch (err) {
        console.error('Failed to create test notification:', err);
        showToast('Failed to create notification: ' + err.message, 'error');
    }
}

// Sync the notifications toggle checkbox state on settings page.
function syncNotificationsToggle() {
    const toggle = document.getElementById('desktop-notifications-toggle');
    if (!toggle) {
        // Not on settings page - this is normal, silently return.
        return;
    }

    // Check user preference from localStorage.
    const storedValue = localStorage.getItem('notifications-enabled');
    const userEnabled = storedValue === 'true';
    const browserAllowed = Notification.permission === 'granted';

    console.log('syncNotificationsToggle:', {
        storedValue,
        userEnabled,
        browserAllowed,
        currentChecked: toggle.checked
    });

    // Show toggle based on user preference.
    toggle.checked = userEnabled;

    // Update visual state (Tailwind v2 CDN doesn't support peer-checked).
    updateToggleVisual(toggle);

    // Only actually enable if browser also allows.
    notificationsEnabled = userEnabled && browserAllowed;

    console.log('syncNotificationsToggle result:', {
        toggleChecked: toggle.checked,
        notificationsEnabled
    });

    // If user enabled but browser not granted, auto-request permission.
    if (userEnabled && !browserAllowed && Notification.permission !== 'denied') {
        Notification.requestPermission().then(permission => {
            notificationsEnabled = permission === 'granted';
        });
    }
}

// Initialize notifications on page load.
document.addEventListener('DOMContentLoaded', function() {
    console.log('DOMContentLoaded - Initial notification state:', {
        permission: Notification.permission,
        localStorage: localStorage.getItem('notifications-enabled'),
        notificationsEnabled: notificationsEnabled
    });

    // Sync settings toggle if on settings page.
    syncNotificationsToggle();

    console.log('After syncNotificationsToggle:', {
        notificationsEnabled: notificationsEnabled
    });

    // Only show prompt if not previously dismissed and notifications not explicitly disabled.
    const userDisabled = localStorage.getItem('notifications-enabled') === 'false';
    if (!localStorage.getItem('notifications-prompt-dismissed') && !userDisabled) {
        initNotifications();
    } else if (Notification.permission === 'granted' && !userDisabled) {
        notificationsEnabled = true;
    }
    setupSSENotifications();

    // Check for open_thread query param and auto-open thread modal.
    const urlParams = new URLSearchParams(window.location.search);
    const openThread = urlParams.get('open_thread');
    if (openThread) {
        console.log('Auto-opening thread from URL param:', openThread);
        // Load thread into modal container via HTMX-style fetch.
        const modalContainer = document.getElementById('modal-container');
        if (modalContainer) {
            fetch('/thread/' + openThread)
                .then(response => response.text())
                .then(html => {
                    modalContainer.innerHTML = html;
                    // Clean up URL to remove the param.
                    window.history.replaceState({}, '', window.location.pathname);
                })
                .catch(err => console.error('Failed to load thread:', err));
        }
    }
});

// =============================================================================
// HTMX Configuration
// =============================================================================

// Configure HTMX.
document.body.addEventListener('htmx:configRequest', function(evt) {
    // Add CSRF token if needed.
});

// Handle HTMX errors.
document.body.addEventListener('htmx:responseError', function(evt) {
    console.error('HTMX error:', evt.detail);
    showToast('Error loading content', 'error');
});

// Re-sync notification toggle after HTMX navigation (e.g., to settings page).
document.body.addEventListener('htmx:afterSwap', function(evt) {
    syncNotificationsToggle();
});

// Toast notification system.
function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast toast-${type} px-4 py-3 rounded-lg shadow-lg text-white animate-slide-in`;

    switch (type) {
        case 'success':
            toast.classList.add('bg-green-600');
            break;
        case 'error':
            toast.classList.add('bg-red-600');
            break;
        case 'warning':
            toast.classList.add('bg-yellow-600');
            break;
        default:
            toast.classList.add('bg-blue-600');
    }

    toast.textContent = message;
    container.appendChild(toast);

    // Auto-remove after 5 seconds.
    setTimeout(() => {
        toast.classList.add('animate-fade-out');
        setTimeout(() => toast.remove(), 300);
    }, 5000);
}

// Close modal on escape key.
document.addEventListener('keydown', function(evt) {
    if (evt.key === 'Escape') {
        const modal = document.querySelector('#modal-container > .fixed');
        if (modal) {
            modal.remove();
        }
    }
});

// Handle starred toggle.
function toggleStar(element, messageId) {
    event.stopPropagation();
    event.preventDefault();

    const isStarred = element.classList.contains('starred');

    fetch(`/api/messages/${messageId}/star`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ starred: !isStarred }),
    })
    .then(response => {
        if (response.ok) {
            element.classList.toggle('starred');
            element.classList.toggle('text-yellow-500');
            element.classList.toggle('text-gray-300');
        }
    })
    .catch(err => console.error('Failed to toggle star:', err));
}

// Handle checkbox selection.
function toggleSelect(element, messageId) {
    event.stopPropagation();

    const row = document.getElementById(`message-${messageId}`);
    if (row) {
        row.classList.toggle('selected');
        row.classList.toggle('bg-blue-50');
    }

    updateBulkActions();
}

// Update bulk action toolbar based on selection.
function updateBulkActions() {
    const selected = document.querySelectorAll('.message-row.selected');
    const toolbar = document.getElementById('bulk-actions');

    if (toolbar) {
        if (selected.length > 0) {
            toolbar.classList.remove('hidden');
            toolbar.querySelector('.count').textContent = selected.length;
        } else {
            toolbar.classList.add('hidden');
        }
    }
}

// Session tab switching (used in session-detail.html).
function switchSessionTab(button, tabId) {
    // Update button styles.
    document.querySelectorAll('.session-tab').forEach(t => {
        t.classList.remove('active', 'text-blue-600', 'border-b-2', 'border-blue-600');
        t.classList.add('text-gray-500');
    });
    button.classList.add('active', 'text-blue-600', 'border-b-2', 'border-blue-600');
    button.classList.remove('text-gray-500');

    // Show/hide content.
    document.querySelectorAll('.session-tab-content').forEach(c => c.classList.add('hidden'));
    document.getElementById('tab-' + tabId).classList.remove('hidden');
}

// Agent filter tab switching.
function setAgentFilter(button, filter) {
    // Update button styles.
    document.querySelectorAll('.filter-tab').forEach(t => {
        t.classList.remove('active');
    });
    button.classList.add('active');

    // Store filter state and update hx-get URL for polling.
    const grid = document.getElementById('agents-grid');
    if (grid) {
        grid.dataset.filter = filter;
        // Update hx-get URL so polling respects the filter.
        grid.setAttribute('hx-get', '/api/agents/cards?filter=' + filter);
    }
}

// =============================================================================
// Agent Switcher Dropdown
// =============================================================================

// Toggle the agent switcher dropdown visibility.
function toggleAgentSwitcher() {
    const dropdown = document.getElementById('agent-switcher-dropdown');
    if (!dropdown) return;

    dropdown.classList.toggle('hidden');

    // Close on outside click.
    if (!dropdown.classList.contains('hidden')) {
        setTimeout(() => {
            document.addEventListener('click', closeAgentSwitcherOnOutsideClick);
        }, 0);
    }
}

// Close the agent switcher dropdown.
function closeAgentSwitcher() {
    const dropdown = document.getElementById('agent-switcher-dropdown');
    if (dropdown) {
        dropdown.classList.add('hidden');
    }
    document.removeEventListener('click', closeAgentSwitcherOnOutsideClick);
}

// Close dropdown when clicking outside.
function closeAgentSwitcherOnOutsideClick(event) {
    const button = document.getElementById('agent-switcher-button');
    const dropdown = document.getElementById('agent-switcher-dropdown');
    if (!button || !dropdown) return;

    if (!button.contains(event.target) && !dropdown.contains(event.target)) {
        closeAgentSwitcher();
    }
}

// Update the displayed agent name in the header when switching agents.
// Also updates the checkmark in the dropdown to show the new selection.
function switchToAgent(agentName) {
    const nameSpan = document.getElementById('current-agent-name');
    if (nameSpan) {
        nameSpan.textContent = agentName;
    }

    // Update checkmarks in the dropdown.
    const dropdown = document.getElementById('agent-switcher-dropdown');
    if (!dropdown) return;

    // Checkmark SVG template (matches server-rendered structure).
    const checkmarkSVG = `<svg class="w-4 h-4 ml-auto text-blue-600" fill="currentColor" viewBox="0 0 20 20">
        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
    </svg>`;

    // Find all agent links in the dropdown.
    const links = dropdown.querySelectorAll('a');
    links.forEach(link => {
        // Remove any existing checkmark (identified by ml-auto class which pushes it right).
        const existingCheck = link.querySelector('svg.ml-auto');
        if (existingCheck) {
            existingCheck.remove();
        }

        // Remove active styling.
        link.classList.remove('bg-blue-50', 'text-blue-700', 'font-medium');
        link.classList.add('text-gray-700');

        // Update icon color to gray.
        const iconSvg = link.querySelector('svg');
        if (iconSvg) {
            iconSvg.classList.remove('text-blue-600');
            iconSvg.classList.add('text-gray-400');
        }

        // Find the agent name in this link.
        const nameEl = link.querySelector('span');
        const linkAgentName = nameEl ? nameEl.textContent.trim() : '';

        // If this is the selected agent, add checkmark and active styling.
        if (linkAgentName === agentName) {
            link.insertAdjacentHTML('beforeend', checkmarkSVG);
            link.classList.add('bg-blue-50', 'text-blue-700', 'font-medium');
            link.classList.remove('text-gray-700');
            // Update icon color to blue.
            const icon = link.querySelector('svg:not(.ml-auto)');
            if (icon) {
                icon.classList.remove('text-gray-400');
                icon.classList.add('text-blue-600');
            }
        }
    });
}

// =============================================================================
// HTMX Custom Event Handlers
// =============================================================================

// Listen for custom HX-Trigger events from the server.
document.body.addEventListener('showToast', function(evt) {
    const message = evt.detail.value || evt.detail;
    showToast(message, 'info');
});

document.body.addEventListener('closeThread', function(evt) {
    // Close the thread view modal.
    const modal = document.getElementById('modal-container');
    if (modal) {
        modal.innerHTML = '';
    }
});

document.body.addEventListener('refreshInbox', function(evt) {
    // Trigger a refresh of the message list.
    const messageList = document.getElementById('message-list');
    if (messageList) {
        htmx.trigger(messageList, 'refresh');
    }
});
