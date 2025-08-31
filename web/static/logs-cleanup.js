// Logs Page Cleanup Functions

// 显示清理日志对话框
function showCleanupModal() {
    const modal = new bootstrap.Modal(document.getElementById('cleanupModal'));
    modal.show();
}

// 确认清理日志
function confirmCleanup() {
    const selectedRange = document.querySelector('input[name="cleanupRange"]:checked');
    if (!selectedRange) {
        alert(T('select_cleanup_range', '请选择清理范围'));
        return;
    }

    const days = parseInt(selectedRange.value);
    let confirmMessage;
    
    if (days === 0) {
        confirmMessage = T('confirm_clear_all_logs', '确定要清除所有日志吗？此操作不可撤销！');
    } else {
        confirmMessage = T('confirm_clear_old_logs', '确定要清除 {0} 天前的日志吗？此操作不可撤销！').replace('{0}', days);
    }

    if (!confirm(confirmMessage)) {
        return;
    }

    // 禁用确认按钮，显示加载状态
    const confirmBtn = document.querySelector('#cleanupModal .btn-danger');
    const originalText = confirmBtn.innerHTML;
    confirmBtn.disabled = true;
    confirmBtn.innerHTML = T('cleaning_logs', '<i class="fas fa-spinner fa-spin"></i> 清理中...');

    // 发送清理请求
    apiRequest('/admin/api/logs/cleanup', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            days: days
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            throw new Error(data.error);
        }
        
        // 显示成功消息
        alert(data.message);
        
        // 关闭对话框
        const modal = bootstrap.Modal.getInstance(document.getElementById('cleanupModal'));
        modal.hide();
        
        // 刷新页面显示最新的日志列表
        window.location.reload();
    })
    .catch(error => {
        console.error('Cleanup logs failed:', error);
        alert(T('cleanup_failed_error', '清理日志失败') + ': ' + error.message);
    })
    .finally(() => {
        // 恢复按钮状态
        confirmBtn.disabled = false;
        confirmBtn.innerHTML = originalText;
    });
}