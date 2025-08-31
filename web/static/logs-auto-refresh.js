// Auto-refresh functionality for logs page

let autoRefreshEnabled = false;
let autoRefreshInterval = null;
let autoRefreshTimer = 5000; // 5 seconds

// Initialize auto-refresh state on page load
document.addEventListener('DOMContentLoaded', function() {
    // Wait for translation system to be ready
    function initAutoRefresh() {
        // Check if I18n is available and translations are loaded
        if (typeof T === 'function' && window.I18n) {
            const allTranslations = window.I18n.getAllTranslations();
            const currentLang = window.I18n.getLanguage();
            
            // Check if translations for current language are loaded
            if (allTranslations[currentLang] && Object.keys(allTranslations[currentLang]).length > 0) {
                console.log('Translations loaded, initializing auto-refresh');
                
                // Load auto-refresh state from localStorage
                const savedState = localStorage.getItem('autoRefreshEnabled');
                if (savedState === 'true') {
                    autoRefreshEnabled = true;
                    startAutoRefresh();
                }
                updateAutoRefreshButton();
                return;
            }
        }
        
        // Translation system not ready yet, wait a bit
        console.log('Waiting for translations to load...');
        setTimeout(initAutoRefresh, 100);
    }
    
    initAutoRefresh();
});

function toggleAutoRefresh() {
    autoRefreshEnabled = !autoRefreshEnabled;
    
    // Save state to localStorage
    localStorage.setItem('autoRefreshEnabled', autoRefreshEnabled.toString());
    
    if (autoRefreshEnabled) {
        startAutoRefresh();
    } else {
        stopAutoRefresh();
    }
    
    updateAutoRefreshButton();
}

function startAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
    }
    
    autoRefreshInterval = setInterval(function() {
        // Check if any modal is currently open
        if (isAnyModalOpen()) {
            console.log('Modal is open, skipping auto-refresh');
            return;
        }
        
        // Get current page parameters
        const urlParams = new URLSearchParams(window.location.search);
        const currentPage = urlParams.get('page') || '1';
        const failedOnly = urlParams.get('failed_only') === 'true';
        
        // Refresh the page
        refreshLogs(currentPage, failedOnly);
    }, autoRefreshTimer);
}

function stopAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
        autoRefreshInterval = null;
    }
}

function updateAutoRefreshButton() {
    const button = document.getElementById('autoRefreshToggle');
    const icon = document.getElementById('autoRefreshIcon');
    const text = document.getElementById('autoRefreshText');
    
    // Check if elements exist and T function is available
    if (!button || !icon || !text || typeof T !== 'function') {
        console.warn('Auto-refresh button elements not found or T function not available');
        return;
    }

    if (autoRefreshEnabled) {
        button.className = 'btn btn-sm btn-success';
        icon.className = 'fas fa-sync';
        text.textContent = T('auto_refresh_on', '自动刷新中');
        // 不设置 data-t 属性，避免双重翻译
    } else {
        button.className = 'btn btn-sm btn-outline-info';
        icon.className = 'fas fa-sync';
        text.textContent = T('auto_refresh', '自动刷新');
        // 不设置 data-t 属性，避免双重翻译
    }
}

function isAnyModalOpen() {
    // Check for Bootstrap modal classes that indicate an open modal
    const modals = document.querySelectorAll('.modal');
    for (let modal of modals) {
        if (modal.classList.contains('show')) {
            return true;
        }
    }
    
    // Additional check for modal backdrop
    const backdrop = document.querySelector('.modal-backdrop');
    if (backdrop) {
        return true;
    }
    
    return false;
}

// Clean up interval when page is unloaded
window.addEventListener('beforeunload', function() {
    stopAutoRefresh();
});