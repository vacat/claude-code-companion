// Logs Page Initialization and Event Handling

// Format cells after page loads
document.addEventListener('DOMContentLoaded', function() {
    // Initialize common features from shared.js
    initializeCommonFeatures();
    
    // Format endpoint cells to show only domain with hover for full URL
    document.querySelectorAll('.endpoint-cell').forEach(function(cell) {
        const fullEndpoint = cell.getAttribute('data-endpoint');
        if (fullEndpoint && fullEndpoint !== 'failed') {
            const urlFormatted = formatUrlDisplay(fullEndpoint);
            cell.innerHTML = `<small><code title="${escapeHtml(urlFormatted.title)}">${escapeHtml(urlFormatted.display)}</code></small>`;
        } else {
            // For 'failed' or other non-URL values, keep as is
            cell.innerHTML = `<small>${escapeHtml(fullEndpoint)}</small>`;
        }
    });
});