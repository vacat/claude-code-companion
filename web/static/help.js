/* Help Page JavaScript Functions */

// Detect OS and set active tab
function detectOS() {
    const userAgent = navigator.userAgent.toLowerCase();
    if (userAgent.indexOf('win') > -1) return 'windows';
    if (userAgent.indexOf('mac') > -1) return 'macos';
    if (userAgent.indexOf('linux') > -1) return 'linux';
    return 'windows'; // default
}

// Initialize tabs based on detected OS
function initializeOSTabs() {
    const detectedOS = detectOS();
    console.log('Detected OS:', detectedOS);
    
    // Always ensure that the correct tab is shown for the detected OS
    // First, remove active from all installation tabs
    document.querySelectorAll('#osTabs .nav-link').forEach(tab => {
        tab.classList.remove('active');
        tab.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('#osTabContent .tab-pane').forEach(pane => {
        pane.classList.remove('show', 'active');
    });
    
    // Then activate the correct installation tab
    const installationTab = document.getElementById(detectedOS + '-tab');
    const installationPane = document.getElementById(detectedOS);
    if (installationTab && installationPane) {
        installationTab.classList.add('active');
        installationTab.setAttribute('aria-selected', 'true');
        installationPane.classList.add('show', 'active');
    }
    
    // Do the same for script tabs
    document.querySelectorAll('#scriptTabs .nav-link').forEach(tab => {
        tab.classList.remove('active');
        tab.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('#scriptTabContent .tab-pane').forEach(pane => {
        pane.classList.remove('show', 'active');
    });
    
    // Then activate the correct script tab
    const scriptTab = document.getElementById(detectedOS + '-script-tab');
    const scriptPane = document.getElementById(detectedOS + '-script');
    if (scriptTab && scriptPane) {
        scriptTab.classList.add('active');
        scriptTab.setAttribute('aria-selected', 'true');
        scriptPane.classList.add('show', 'active');
    }
}

// Copy to clipboard function for help page
function copyToClipboard(button) {
    const codeBlock = button.parentNode.querySelector('.code-block');
    const text = codeBlock.innerText;
    
    navigator.clipboard.writeText(text).then(function() {
        const icon = button.querySelector('i');
        const originalClass = icon.className;
        icon.className = 'fas fa-check';
        button.classList.remove('btn-outline-light');
        button.classList.add('btn-success');
        
        setTimeout(function() {
            icon.className = originalClass;
            button.classList.remove('btn-success');
            button.classList.add('btn-outline-light');
        }, 2000);
    });
}

// Download script function
function downloadScript(platform) {
    let content, filename;
    
    switch(platform) {
        case 'windows':
            content = document.getElementById('windows-script-content').innerText;
            filename = 'ccc.bat';
            break;
        case 'macos':
            content = document.getElementById('macos-script-content').innerText;
            filename = 'ccc.command';
            break;
        case 'linux':
            content = document.getElementById('linux-script-content').innerText;
            filename = 'ccc.sh';
            break;
    }

    const blob = new Blob([content], { type: 'text/plain' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
}

// Initialize help page on DOM ready
document.addEventListener('DOMContentLoaded', function() {
    initializeOSTabs();
});