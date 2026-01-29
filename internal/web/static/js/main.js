// Subtrate UI JavaScript

// Configure HTMX.
document.body.addEventListener('htmx:configRequest', function(evt) {
    // Add CSRF token if needed.
});

// Handle HTMX errors.
document.body.addEventListener('htmx:responseError', function(evt) {
    console.error('HTMX error:', evt.detail);
    showToast('Error loading content', 'error');
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
