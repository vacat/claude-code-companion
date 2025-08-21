// Dashboard Page JavaScript

document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    
    // Format URL cells to show only domain with hover for full URL
    document.querySelectorAll('.url-cell').forEach(function(cell) {
        const fullUrl = cell.getAttribute('data-url');
        if (fullUrl) {
            const urlFormatted = formatUrlDisplay(fullUrl);
            cell.innerHTML = `<code title="${escapeHtml(urlFormatted.title)}">${escapeHtml(urlFormatted.display)}</code>`;
        }
    });
    
    // Auto-refresh every 30 seconds
    setInterval(function() {
        location.reload();
    }, 30000);
});