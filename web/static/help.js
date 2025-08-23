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
            content = generateEnhancedWindowsScript();
            filename = 'ccc.bat';
            break;
        case 'macos':
            content = generateEnhancedUnixScript('macOS');
            filename = 'ccc.command';
            break;
        case 'linux':
            content = generateEnhancedUnixScript('Linux');
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

// Generate enhanced Windows script
function generateEnhancedWindowsScript() {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    
    return `@echo off
REM Claude Code Companion - Windows Launcher
echo Configuring Claude Code for proxy use...

REM Use embedded Node.js to process settings.json
node -e "const fs=require('fs');const path=require('path');const os=require('os');const claudeDir=path.join(os.homedir(),'.claude');const settingsFile=path.join(claudeDir,'settings.json');const targetEnv={'ANTHROPIC_BASE_URL':'${baseUrl}','ANTHROPIC_AUTH_TOKEN':'hello','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','API_TIMEOUT_MS':'600000'};function processSettings(){if(!fs.existsSync(settingsFile)){console.log('Claude settings file not found, using environment variables only');return true;}try{const content=fs.readFileSync(settingsFile,'utf8');const settings=JSON.parse(content);if(!settings.env)settings.env={};let needsUpdate=false;let backupCreated=false;for(const[key,targetValue]of Object.entries(targetEnv)){const currentValue=settings.env[key];if(currentValue!==targetValue){if(!backupCreated){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');const backupFile=settingsFile+'.backup-'+timestamp;fs.copyFileSync(settingsFile,backupFile);console.log('Backed up settings to: '+backupFile);backupCreated=true;}if(currentValue){console.log('Updating '+key+': '+currentValue+' -> '+targetValue);}else{console.log('Adding '+key+': '+targetValue);}settings.env[key]=targetValue;needsUpdate=true;}}if(needsUpdate){fs.writeFileSync(settingsFile,JSON.stringify(settings,null,2));console.log('Settings updated successfully');}else{console.log('Settings already configured correctly');}return true;}catch(error){console.error('Error processing settings:',error.message);console.log('Using fallback environment variables...');return false;}}process.exit(processSettings()?0:1);"

REM Check configuration result
if %errorlevel% equ 0 (
    echo Starting Claude...
    claude %*
) else (
    echo Configuration failed, using fallback environment variables...
    set ANTHROPIC_BASE_URL=${baseUrl}
    set ANTHROPIC_AUTH_TOKEN=hello
    set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
    set API_TIMEOUT_MS=600000
    claude %*
)`;
}

// Generate enhanced Unix script (Linux/macOS)
function generateEnhancedUnixScript(osName) {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    
    return `#!/bin/bash
# Claude Code Companion - ${osName} Launcher
echo "Configuring Claude Code for proxy use..."

# Use embedded Node.js to process settings.json
node -e "const fs=require('fs');const path=require('path');const os=require('os');const claudeDir=path.join(os.homedir(),'.claude');const settingsFile=path.join(claudeDir,'settings.json');const targetEnv={'ANTHROPIC_BASE_URL':'${baseUrl}','ANTHROPIC_AUTH_TOKEN':'hello','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','API_TIMEOUT_MS':'600000'};function processSettings(){if(!fs.existsSync(settingsFile)){console.log('Claude settings file not found, using environment variables only');return true;}try{const content=fs.readFileSync(settingsFile,'utf8');const settings=JSON.parse(content);if(!settings.env)settings.env={};let needsUpdate=false;let backupCreated=false;for(const[key,targetValue]of Object.entries(targetEnv)){const currentValue=settings.env[key];if(currentValue!==targetValue){if(!backupCreated){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');const backupFile=settingsFile+'.backup-'+timestamp;fs.copyFileSync(settingsFile,backupFile);console.log('Backed up settings to: '+backupFile);backupCreated=true;}if(currentValue){console.log('Updating '+key+': '+currentValue+' -> '+targetValue);}else{console.log('Adding '+key+': '+targetValue);}settings.env[key]=targetValue;needsUpdate=true;}}if(needsUpdate){fs.writeFileSync(settingsFile,JSON.stringify(settings,null,2));console.log('Settings updated successfully');}else{console.log('Settings already configured correctly');}return true;}catch(error){console.error('Error processing settings:',error.message);console.log('Using fallback environment variables...');return false;}}process.exit(processSettings()?0:1);"

# Check configuration result
if [ $? -eq 0 ]; then
    echo "Starting Claude..."
    exec claude "$@"
else
    echo "Configuration failed, using fallback environment variables..."
    export ANTHROPIC_BASE_URL="${baseUrl}"
    export ANTHROPIC_AUTH_TOKEN="hello"
    export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
    export API_TIMEOUT_MS="600000"
    exec claude "$@"
fi`;
}

// Initialize help page on DOM ready
document.addEventListener('DOMContentLoaded', function() {
    initializeOSTabs();
});