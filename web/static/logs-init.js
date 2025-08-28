// Logs Page Initialization and Event Handling

// Format cells after page loads
document.addEventListener('DOMContentLoaded', function() {
    // Initialize common features from shared.js
    initializeCommonFeatures();
    
    // Format endpoint cells to show only domain with hover for full URL
    document.querySelectorAll('.endpoint-cell').forEach(function(cell) {
        const fullEndpoint = cell.getAttribute('data-endpoint');
        
        // Check if the cell already has error information (preserve it)
        const existingErrorInfo = cell.querySelector('small.text-danger');
        
        if (fullEndpoint && fullEndpoint !== 'failed') {
            const urlFormatted = formatUrlDisplay(fullEndpoint);
            const endpointHtml = `<div><small><code title="${escapeHtml(urlFormatted.title)}">${escapeHtml(urlFormatted.display)}</code></small></div>`;
            
            if (existingErrorInfo) {
                // Preserve existing error information
                cell.innerHTML = endpointHtml + existingErrorInfo.outerHTML;
            } else {
                cell.innerHTML = endpointHtml;
            }
        } else if (fullEndpoint === 'failed' || !fullEndpoint) {
            // For 'failed' or empty endpoint values, check if we should preserve existing content
            if (!existingErrorInfo) {
                cell.innerHTML = `<div><small>${escapeHtml(fullEndpoint || '')}</small></div>`;
            }
            // If existingErrorInfo exists, keep the current content as is
        }
    });
    
    // Initialize session ID badges with colors
    document.querySelectorAll('.session-id-badge').forEach(function(badge) {
        const sessionId = badge.getAttribute('data-session-id');
        
        // Always set background color, even for empty or '--' cases
        const color = generateSessionIdColor(sessionId);
        badge.style.backgroundColor = color;
        
        if (sessionId && sessionId !== '--') {
            // Set display text to last 2 characters
            const displayText = getSessionIdDisplayText(sessionId);
            badge.textContent = displayText;
        }
        // For empty or '--' cases, keep the original text content
    });
    
    // Add event listeners for log page buttons
    addLogPageEventListeners();
});

// Add event listeners for log page buttons
function addLogPageEventListeners() {
    // Event delegation for log page buttons
    document.addEventListener('click', function(e) {
        const target = e.target.closest('button');
        if (!target) return;
        
        const action = target.dataset.action;
        
        console.log('Button clicked with action:', action); // Debug log
        
        // Handle data-action buttons
        switch (action) {
            case 'show-cleanup-modal':
                e.preventDefault();
                console.log('Calling showCleanupModal'); // Debug log
                showCleanupModal();
                break;
                
            case 'toggle-failed-only':
                e.preventDefault();
                const failedOnly = target.dataset.currentFailedOnly === 'true';
                const currentPage = target.dataset.page || '1';
                console.log('Calling toggleFailedOnly with:', failedOnly, currentPage); // Debug log
                toggleFailedOnly(failedOnly, currentPage);
                break;
                
            case 'refresh-logs':
                e.preventDefault();
                const refreshPage = target.dataset.page || '1';
                const refreshFailedOnly = target.dataset.failedOnly === 'true';
                console.log('Calling refreshLogs with:', refreshPage, refreshFailedOnly); // Debug log
                refreshLogs(refreshPage, refreshFailedOnly);
                break;
                
            case 'toggle-auto-refresh':
                e.preventDefault();
                console.log('Calling toggleAutoRefresh'); // Debug log
                toggleAutoRefresh();
                break;
                
            case 'copy-request-id':
                e.preventDefault();
                const requestId = target.dataset.requestId;
                console.log('Copying request ID:', requestId); // Debug log
                copyRequestId(requestId);
                break;
                
            default:
                // Handle onclick attribute for cleanup and failed-only buttons (fallback)
                const onclick = target.getAttribute('onclick');
                if (onclick && onclick.includes('showCleanupModal')) {
                    e.preventDefault();
                    showCleanupModal();
                } else if (onclick && onclick.includes('toggleFailedOnly')) {
                    e.preventDefault();
                    // Extract parameters from onclick string
                    const match = onclick.match(/toggleFailedOnly\(([^,]+),\s*([^)]+)\)/);
                    if (match) {
                        const failedOnly = match[1].trim() === 'true';
                        const currentPage = match[2].trim().replace(/['"]/g, '');
                        toggleFailedOnly(failedOnly, currentPage);
                    }
                }
                break;
        }
    });
}