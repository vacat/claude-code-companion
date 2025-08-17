// Logs Page Modal Functions

function showLogDetails(requestId) {
    fetch(`/admin/api/logs?request_id=${requestId}`)
        .then(response => response.json())
        .then(data => {
            if (data.logs && data.logs.length > 0) {
                displayMultipleLogDetails(data.logs);
            }
        })
        .catch(error => {
            console.error('Error fetching log details:', error);
        });
}

function displayMultipleLogDetails(logs) {
    const modalBody = document.getElementById('modalBody');
    
    if (logs.length === 1) {
        displayLogDetails(logs[0]);
        return;
    }
    
    let html = `<div class="mb-3">
        <h6>请求详情 - ${logs.length} 次尝试</h6>
        <div class="alert alert-info">
            <strong>请求ID:</strong> ${escapeHtml(logs[0].request_id)}<br>
            <strong>路径:</strong> ${escapeHtml(logs[0].path)}<br>
            <strong>请求方法:</strong> ${escapeHtml(logs[0].method)}<br>
            <strong>总耗时:</strong> ${logs.reduce((sum, log) => sum + log.duration_ms, 0)}ms
        </div>
    </div>`;
    
    logs.forEach((log, index) => {
        html += generateLogAttemptHtml(log, index + 1);
    });
    
    modalBody.innerHTML = html;
    
    // Reinitialize tooltips for dynamic content
    var tooltipTriggerList = [].slice.call(modalBody.querySelectorAll('[title]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
    
    const modal = new bootstrap.Modal(document.getElementById('logModal'));
    modal.show();
}