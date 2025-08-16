class AnthropicRequestParser {
    constructor(requestBody) {
        this.raw = requestBody;
        this.data = null;
        this.parsed = {
            overview: {},
            system: {},
            messages: [],
            tools: [],
            errors: []
        };
        this.toolUseMap = new Map(); // 存储 tool_use 以便配对
        this.parse();
    }

    parse() {
        try {
            this.data = JSON.parse(this.raw);
            this.parseOverview();
            this.parseSystem();
            this.parseTools();
            this.parseMessages();
        } catch (error) {
            this.parsed.errors.push(`JSON 解析失败: ${error.message}`);
        }
    }

    parseOverview() {
        this.parsed.overview = {
            model: this.data.model || 'Unknown',
            maxTokens: this.data.max_tokens || 'Not set',
            messageCount: this.data.messages ? this.data.messages.length : 0,
            toolCount: this.data.tools ? this.data.tools.length : 0,
            hasSystem: !!this.data.system,
            estimatedTokens: this.estimateTokens(),
            thinkingEnabled: !!(this.data.thinking && this.data.thinking.enabled),
            thinkingBudget: (this.data.thinking && this.data.thinking.budget_tokens) || 0
        };
    }

    parseSystem() {
        if (this.data.system) {
            // 确保 system 内容是字符串类型
            const systemContent = typeof this.data.system === 'string' ? 
                this.data.system : 
                JSON.stringify(this.data.system);
                
            this.parsed.system = {
                content: systemContent,
                characterCount: systemContent.length,
                wordCount: systemContent.split(/\s+/).filter(word => word.length > 0).length
            };
        }
    }

    parseTools() {
        if (!this.data.tools) return;

        this.data.tools.forEach(tool => {
            const parsedTool = {
                name: tool.name,
                description: tool.description || '',
                parameters: [],
                schema: tool.input_schema || {}
            };

            if (tool.input_schema && tool.input_schema.properties) {
                Object.entries(tool.input_schema.properties).forEach(([name, prop]) => {
                    parsedTool.parameters.push({
                        name: name,
                        type: prop.type || 'unknown',
                        description: prop.description || '',
                        required: tool.input_schema.required && tool.input_schema.required.includes(name),
                        enum: prop.enum || null
                    });
                });
            }

            this.parsed.tools.push(parsedTool);
        });
    }

    parseMessages() {
        if (!this.data.messages) return;

        this.data.messages.forEach((message, index) => {
            const parsedMessage = {
                index: index + 1,
                role: message.role,
                content: [],
                toolUses: [],
                systemReminders: [],
                pairedToolCalls: []
            };

            if (Array.isArray(message.content)) {
                message.content.forEach(content => {
                    this.parseMessageContent(content, parsedMessage);
                });
            } else if (typeof message.content === 'string') {
                this.parseTextContent(message.content, parsedMessage);
            }

            this.parsed.messages.push(parsedMessage);
        });

        // 配对 tool uses 和 results
        this.pairToolCalls();
    }

    parseMessageContent(content, parsedMessage) {
        if (content.type === 'text') {
            this.parseTextContent(content.text, parsedMessage);
        } else if (content.type === 'tool_use') {
            const toolUse = {
                id: content.id,
                name: content.name,
                input: content.input,
                type: 'use'
            };
            parsedMessage.toolUses.push(toolUse);
            this.toolUseMap.set(content.id, toolUse);
        } else if (content.type === 'tool_result') {
            const toolResult = {
                id: content.tool_use_id,
                result: content.content,
                isError: content.is_error || false,
                type: 'result'
            };
            parsedMessage.toolUses.push(toolResult);
        }
    }

    parseTextContent(text, parsedMessage) {
        // 提取 system reminders
        const reminders = this.extractSystemReminders(text);
        parsedMessage.systemReminders.push(...reminders);

        // 移除 system reminders 后的文本
        const cleanText = this.removeSystemReminders(text);
        
        if (cleanText.trim()) {
            parsedMessage.content.push({
                type: 'text',
                text: cleanText,
                preview: this.createPreview(cleanText)
            });
        }
    }

    extractSystemReminders(text) {
        const reminders = [];
        const regex = /<system-reminder>([\s\S]*?)<\/system-reminder>/g;
        let match;
        
        while ((match = regex.exec(text)) !== null) {
            const content = match[1].trim();
            reminders.push({
                content: content,
                preview: this.createPreview(content),
                type: this.detectReminderType(content)
            });
        }
        
        return reminders;
    }

    removeSystemReminders(text) {
        return text.replace(/<system-reminder>[\s\S]*?<\/system-reminder>/g, '').trim();
    }

    detectReminderType(content) {
        const lowerContent = content.toLowerCase();
        if (lowerContent.includes('context')) return 'context';
        if (lowerContent.includes('tool')) return 'tool';
        if (lowerContent.includes('reminder')) return 'reminder';
        if (lowerContent.includes('instruction')) return 'instruction';
        return 'general';
    }

    createPreview(text, maxLength = 100) {
        if (!text) return '';
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    }

    pairToolCalls() {
        const toolPairs = new Map();
        
        // 收集所有 tool uses 和 results
        this.parsed.messages.forEach(message => {
            message.toolUses.forEach(tool => {
                if (tool.type === 'use') {
                    toolPairs.set(tool.id, { use: tool, result: null, messageIndex: message.index });
                } else if (tool.type === 'result') {
                    if (toolPairs.has(tool.id)) {
                        toolPairs.get(tool.id).result = tool;
                    } else {
                        toolPairs.set(tool.id, { use: null, result: tool, messageIndex: message.index });
                    }
                }
            });
        });

        // 为每个消息生成配对的工具调用
        this.parsed.messages.forEach(message => {
            message.pairedToolCalls = [];
            message.toolUses.forEach(tool => {
                if (tool.type === 'use' && toolPairs.has(tool.id)) {
                    const pair = toolPairs.get(tool.id);
                    const pairedCall = {
                        id: tool.id,
                        name: tool.name,
                        input: tool.input,
                        result: pair.result ? pair.result.result : null,
                        isError: pair.result ? pair.result.isError : false,
                        status: this.getToolStatus(pair),
                        isThinking: this.isThinkingResult(pair.result)
                    };
                    message.pairedToolCalls.push(pairedCall);
                }
            });
        });
    }

    getToolStatus(pair) {
        if (!pair.result) return 'pending';
        if (pair.result.isError) return 'error';
        return 'success';
    }

    isThinkingResult(result) {
        if (!result || !result.result) return false;
        
        // 检查多种 thinking 格式
        const resultStr = typeof result.result === 'string' ? result.result : JSON.stringify(result.result);
        return resultStr.includes('<thinking>') || 
               resultStr.includes('thinking') ||
               (typeof result.result === 'object' && result.result.type === 'thinking');
    }

    estimateTokens() {
        try {
            const text = JSON.stringify(this.data);
            // 粗略估算：英文约4字符=1token，中文约1.5字符=1token
            const chineseChars = (text.match(/[\u4e00-\u9fff]/g) || []).length;
            const otherChars = text.length - chineseChars;
            return Math.ceil(chineseChars / 1.5 + otherChars / 4);
        } catch {
            return 0;
        }
    }

    // 获取工具使用统计
    getToolUsageStats() {
        const stats = {};
        this.parsed.messages.forEach(message => {
            message.pairedToolCalls.forEach(call => {
                if (!stats[call.name]) {
                    stats[call.name] = { count: 0, success: 0, error: 0 };
                }
                stats[call.name].count++;
                if (call.status === 'success') stats[call.name].success++;
                if (call.status === 'error') stats[call.name].error++;
            });
        });
        return stats;
    }

    // 获取消息统计
    getMessageStats() {
        const stats = {
            user: 0,
            assistant: 0,
            system: this.parsed.system.content ? 1 : 0,
            totalSystemReminders: 0,
            totalToolCalls: 0
        };

        this.parsed.messages.forEach(message => {
            stats[message.role]++;
            stats.totalSystemReminders += message.systemReminders.length;
            stats.totalToolCalls += message.pairedToolCalls.length;
        });

        return stats;
    }
}