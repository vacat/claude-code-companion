// Inspector UI Core - Main class and core functionality
class InspectorUI {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.collapseStates = new Map();
    }

    render(parser) {
        if (!this.container) {
            console.error('Inspector container not found');
            return;
        }

        // 保存 parser 引用以便全局查找
        this.currentParser = parser;

        this.container.innerHTML = '';
        
        // 渲染概览
        this.renderOverview(parser.parsed.overview);
        
        // 渲染消息
        this.renderMessages(parser.parsed.messages);
        
        // 渲染系统配置（移至消息后面）
        this.renderSystem(parser.parsed.system, parser.parsed.tools);
        
        // 如果有错误，显示错误信息
        if (parser.parsed.errors.length > 0) {
            this.renderErrors(parser.parsed.errors);
        }
    }

    createElementFromHTML(htmlString) {
        const div = document.createElement('div');
        div.innerHTML = htmlString.trim();
        return div.firstChild;
    }

    escapeHtml(text) {
        return escapeHtml(text);
    }

    formatJSON(obj) {
        try {
            return JSON.stringify(obj, null, 2);
        } catch (e) {
            return String(obj);
        }
    }

    sanitizeId(str) {
        return str.replace(/[^a-zA-Z0-9-_]/g, '_');
    }
}