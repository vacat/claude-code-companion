// Dashboard Page JavaScript

document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    
    // Auto-refresh every 30 seconds
    setInterval(function() {
        location.reload();
    }, 30000);
});