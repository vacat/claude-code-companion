/* Claude Code Companion Web Admin - Shared JavaScript Functions */

// Shared utility functions
function showAlert(message, type = 'info') {
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show alert-positioned`;
    alertDiv.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.body.appendChild(alertDiv);
    
    // Auto dismiss after 3 seconds
    setTimeout(() => {
        if (alertDiv.parentNode) {
            alertDiv.remove();
        }
    }, 3000);
}

// Format utilities
function formatDuration(ms) {
    return (ms / 1000).toFixed(3) + 's';
}

function formatFileSize(bytes) {
    if (bytes === 0) return '0B';
    if (bytes < 1024) return bytes + 'B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + 'K';
    return (bytes / (1024 * 1024)).toFixed(1) + 'M';
}

function formatJson(jsonString) {
    if (!jsonString) return jsonString;
    try {
        const parsed = JSON.parse(jsonString);
        return JSON.stringify(parsed, null, 2);
    } catch {
        return jsonString;
    }
}

// Extract domain from URL
function extractDomain(url) {
    try {
        const urlObj = new URL(url);
        return urlObj.hostname;
    } catch (e) {
        return url; // If not a valid URL, return original content
    }
}

// Truncate domain if exceeds maxLength characters and add ellipsis
function truncateDomain(domain, maxLength = 25) {
    if (!domain || domain.length <= maxLength) {
        return domain;
    }
    return domain.substring(0, maxLength) + '...';
}

// Format URL display: show domain only with full URL in title, truncate if over 25 chars
function formatUrlDisplay(url) {
    const domainOnly = extractDomain(url);
    const truncatedDomain = truncateDomain(domainOnly, 25);
    return {
        display: truncatedDomain,
        title: url
    };
}

function escapeHtml(text) {
    if (!text) return text;
    // 保持中文字符不变，只转义必要的HTML字符
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// UTF-8 safe base64 encoding/decoding
function safeBase64Encode(str) {
    try {
        return btoa(encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, function(match, p1) {
            return String.fromCharCode('0x' + p1);
        }));
    } catch (error) {
        console.warn('Base64编码失败，使用备用方法:', error);
        return encodeURIComponent(str);
    }
}

function safeBase64Decode(str) {
    try {
        const decoded = atob(str);
        return decodeURIComponent(Array.prototype.map.call(decoded, function(c) {
            return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
        }).join(''));
    } catch (error) {
        console.warn('Base64解码失败，使用备用方法:', error);
        try {
            return decodeURIComponent(str);
        } catch (e) {
            console.error('所有解码方法都失败:', e);
            return str;
        }
    }
}

// File utilities
function getFileExtension(content) {
    if (!content) return 'txt';
    
    try {
        JSON.parse(content);
        return 'json';
    } catch {
        if (content.includes('event: ') && content.includes('data: ')) {
            return 'sse';
        }
        return 'txt';
    }
}

function saveAsFileFromButton(button) {
    const filename = button.getAttribute('data-filename');
    const encodedContent = button.getAttribute('data-content');
    
    if (!encodedContent || encodedContent.trim() === '') {
        alert('内容为空，无法保存');
        return;
    }

    try {
        const content = safeBase64Decode(encodedContent);
        
        const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        
        const downloadLink = document.createElement('a');
        downloadLink.href = url;
        downloadLink.download = filename;
        downloadLink.style.display = 'none';
        
        document.body.appendChild(downloadLink);
        downloadLink.click();
        
        setTimeout(() => {
            document.body.removeChild(downloadLink);
            URL.revokeObjectURL(url);
        }, 100);
    } catch (error) {
        console.error('保存文件失败:', error);
        alert('保存文件失败，请检查浏览器控制台');
    }
}

// Copy to clipboard functionality
function copyToClipboard(content) {
    if (!content || content.trim() === '') {
        showAlert('内容为空，无法复制', 'warning');
        return;
    }

    // Try to use modern clipboard API first
    if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(content).then(() => {
            showAlert('已复制到剪贴板', 'success');
        }).catch(err => {
            console.error('Clipboard API failed:', err);
            fallbackCopyToClipboard(content);
        });
    } else {
        // Fallback for older browsers or non-secure contexts
        fallbackCopyToClipboard(content);
    }
}

// Copy request ID to clipboard
function copyRequestId(requestId) {
    copyToClipboard(requestId);
}

function fallbackCopyToClipboard(content) {
    try {
        // Create a temporary textarea element
        const textarea = document.createElement('textarea');
        textarea.value = content;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '-9999px';
        document.body.appendChild(textarea);
        
        // Select and copy
        textarea.select();
        textarea.setSelectionRange(0, 99999);
        const successful = document.execCommand('copy');
        
        document.body.removeChild(textarea);
        
        if (successful) {
            showAlert('已复制到剪贴板', 'success');
        } else {
            showAlert('复制失败，请手动复制', 'danger');
        }
    } catch (err) {
        console.error('Fallback copy failed:', err);
        showAlert('复制失败，请手动复制', 'danger');
    }
}

function copyFromButton(button) {
    const encodedContent = button.getAttribute('data-content');
    if (!encodedContent) {
        showAlert('无内容可复制', 'warning');
        return;
    }
    
    try {
        const content = safeBase64Decode(encodedContent);
        copyToClipboard(content);
    } catch (error) {
        console.error('解码内容失败:', error);
        showAlert('内容解码失败', 'danger');
    }
}

// Bootstrap tooltip initialization helper
function initializeTooltips(container = document) {
    const tooltipTriggerList = [].slice.call(container.querySelectorAll('[data-bs-toggle="tooltip"]'));
    return tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
}

// Language switching functionality
function switchLanguage(lang) {
    // Set language cookie for 1 year
    const expiryDate = new Date();
    expiryDate.setFullYear(expiryDate.getFullYear() + 1);
    document.cookie = `claude_proxy_lang=${lang}; expires=${expiryDate.toUTCString()}; path=/`;
    
    // Update URL parameter and reload page
    const url = new URL(window.location);
    url.searchParams.set('lang', lang);
    window.location.href = url.toString();
}

function updateLanguageDropdown() {
    // Get current language from dropdown data attribute first, then fallback to other methods
    const dropdownElement = document.getElementById('languageDropdown');
    let currentLang = dropdownElement ? dropdownElement.getAttribute('data-current-lang') : null;
    
    if (!currentLang) {
        // Fallback: get from URL parameter
        currentLang = new URLSearchParams(window.location.search).get('lang');
    }
    
    if (!currentLang) {
        // Fallback: get from cookie
        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === 'claude_proxy_lang') {
                currentLang = value;
                break;
            }
        }
    }
    
    // Default to zh-cn if no language found
    if (!currentLang) {
        currentLang = 'zh-cn';
    }
    
    // Update dropdown display
    const flagElement = document.getElementById('currentLanguageFlag');
    const textElement = document.getElementById('currentLanguageText');
    
    if (flagElement && textElement && window.availableLanguages) {
        const langInfo = window.availableLanguages[currentLang];
        if (langInfo) {
            flagElement.textContent = langInfo.flag;
            textElement.textContent = langInfo.name;
            flagElement.style.display = 'inline-block';
            
            // Set flag color based on language
            switch (currentLang) {
                case 'en':
                    flagElement.style.backgroundColor = '#007bff'; // Blue for US
                    break;
                case 'zh-cn':
                default:
                    flagElement.style.backgroundColor = '#dc3545'; // Red for CN
                    break;
            }
        } else {
            // Fallback for unknown language
            flagElement.textContent = '??';
            textElement.textContent = currentLang;
            flagElement.style.backgroundColor = '#6c757d'; // Gray for unknown
            flagElement.style.display = 'inline-block';
        }
    }
}

// Common DOM ready initialization
function initializeCommonFeatures() {
    // Format duration cells
    document.querySelectorAll('.duration-cell').forEach(function(cell) {
        const ms = parseInt(cell.getAttribute('data-ms'));
        if (!isNaN(ms)) {
            cell.textContent = formatDuration(ms);
        }
    });
    
    // Format file size cells
    document.querySelectorAll('.filesize-cell').forEach(function(cell) {
        const bytes = parseInt(cell.getAttribute('data-bytes'));
        if (!isNaN(bytes)) {
            cell.textContent = formatFileSize(bytes);
        }
    });
    
    // Initialize Bootstrap tooltips
    initializeTooltips();
    
    // Update language dropdown
    updateLanguageDropdown();
}