// Inspector UI System - System configuration and tools rendering functionality
InspectorUI.prototype.renderSystem = function(system, tools) {
    let systemHtml = `
        <div class="inspector-section">
            <h6 class="inspector-title">${T('inspector_system_config', '🔧 系统配置')}</h6>
    `;

    // System Prompt
    if (system.content) {
        const systemId = 'system-prompt';
        systemHtml += `
            <div class="inspector-subsection">
                <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${systemId}')">
                    <span class="inspector-collapse-icon" id="${systemId}-icon">▶</span>
                    📝 ${T('inspector_system_prompt', 'System Prompt')} (${system.characterCount} ${T('inspector_characters', '字符')}, ${system.wordCount} ${T('inspector_words', '词')})
                </div>
                <div class="inspector-collapse-content" id="${systemId}" style="display: none;">
                    <div class="inspector-content-box">
                        <pre class="inspector-code">${this.escapeHtml(system.content)}</pre>
                    </div>
                </div>
            </div>
        `;
    }

    // Tools
    if (tools.length > 0) {
        const toolsId = 'available-tools';
        systemHtml += `
            <div class="inspector-subsection">
                <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${toolsId}')">
                    <span class="inspector-collapse-icon" id="${toolsId}-icon">▶</span>
                    🛠️ ${T('inspector_available_tools', '可用工具')} (${tools.length}${T('inspector_count_suffix', '个')})
                </div>
                <div class="inspector-collapse-content" id="${toolsId}" style="display: none;">
                    ${this.renderToolsList(tools)}
                </div>
            </div>
        `;
    }

    systemHtml += '</div>';
    this.container.appendChild(this.createElementFromHTML(systemHtml));
};

InspectorUI.prototype.renderToolsList = function(tools) {
    return tools.map(tool => {
        const toolId = `tool-${this.sanitizeId(tool.name)}`;
        return `
            <div class="inspector-tool-item">
                <div class="inspector-collapse-header inspector-tool-header" onclick="window.inspectorToggleCollapse('${toolId}')">
                    <span class="inspector-collapse-icon" id="${toolId}-icon">▶</span>
                    🔧 ${this.escapeHtml(tool.name)}
                </div>
                <div class="inspector-collapse-content" id="${toolId}" style="display: none;">
                    ${this.renderToolDetails(tool)}
                </div>
            </div>
        `;
    }).join('');
};

InspectorUI.prototype.renderToolDetails = function(tool) {
    let detailsHtml = '<div class="inspector-tool-details">';
    
    // 工具描述（可折叠）
    if (tool.description) {
        const descId = `tool-desc-${this.sanitizeId(tool.name)}`;
        detailsHtml += `
            <div class="inspector-tool-subsection">
                <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${descId}')">
                    <span class="inspector-collapse-icon" id="${descId}-icon">▶</span>
                    📖 ${T('inspector_description', '描述')}
                </div>
                <div class="inspector-collapse-content" id="${descId}" style="display: none;">
                    <div class="inspector-content-box">
                        ${this.escapeHtml(tool.description)}
                    </div>
                </div>
            </div>
        `;
    }
    
    // 参数列表（可折叠）
    if (tool.parameters.length > 0) {
        const paramsId = `tool-params-${this.sanitizeId(tool.name)}`;
        detailsHtml += `
            <div class="inspector-tool-subsection">
                <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${paramsId}')">
                    <span class="inspector-collapse-icon" id="${paramsId}-icon">▶</span>
                    📋 ${T('inspector_parameter_list', '参数列表')} (${tool.parameters.length}${T('inspector_count_suffix', '个')})
                </div>
                <div class="inspector-collapse-content" id="${paramsId}" style="display: none;">
                    <ul class="inspector-param-list">
        `;
        
        tool.parameters.forEach(param => {
            const requiredBadge = param.required ? 
                `<span class="badge bg-danger">${T('inspector_required', '必需')}</span>` : 
                `<span class="badge bg-secondary">${T('inspector_optional', '可选')}</span>`;
            detailsHtml += `
                <li class="inspector-param-item">
                    <code>${this.escapeHtml(param.name)}</code> 
                    <span class="inspector-param-type">(${this.escapeHtml(param.type)})</span>
                    ${requiredBadge}
                    ${param.description ? `<div class="inspector-param-desc">${this.escapeHtml(param.description)}</div>` : ''}
                    ${param.enum ? `<div class="inspector-param-desc">${T('inspector_enum_values', '可选值')}: ${param.enum.map(v => `<code>${this.escapeHtml(v)}</code>`).join(', ')}</div>` : ''}
                </li>
            `;
        });
        
        detailsHtml += `
                    </ul>
                </div>
            </div>
        `;
    }

    detailsHtml += '</div>';
    return detailsHtml;
};