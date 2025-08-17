// Logs Page Navigation and Refresh Functions

function toggleFailedOnly(failedOnly, currentPage) {
    failedOnly = !failedOnly;
    window.location.href = `/admin/logs?page=1&failed_only=${failedOnly}`;
}

function refreshLogs(currentPage, failedOnly) {
    window.location.href = `/admin/logs?page=${currentPage}&failed_only=${failedOnly}`;
}