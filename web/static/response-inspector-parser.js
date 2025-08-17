class AnthropicResponseParser {
    constructor(responseBody, isStreaming = false, finalResponseBody = '') {
        this.rawResponse = responseBody;
        this.isStreaming = isStreaming;
        this.finalResponse = finalResponseBody;
        this.parsed = {
            metadata: {},
            usage: {},
            content: [],
            streamingInfo: null,
            errors: []
        };
        this.parse();
    }

    parse() {
        try {
            if (this.isStreaming) {
                this.parseStreaming();
            } else {
                this.parseNonStreaming();
            }
        } catch (error) {
            this.parsed.errors.push(`解析失败: ${error.message}`);
        }
    }

    parseNonStreaming() {
        const data = JSON.parse(this.rawResponse);
        
        // 解析元数据
        this.parsed.metadata = {
            id: data.id,
            model: data.model,
            role: data.role,
            stop_reason: data.stop_reason,
            stop_sequence: data.stop_sequence,
            isStreaming: false,
            completedAt: new Date().toISOString()
        };

        // 解析使用统计
        if (data.usage) {
            this.parsed.usage = this.calculateUsage(data.usage);
        }

        // 解析内容块
        if (data.content && Array.isArray(data.content)) {
            this.parsed.content = data.content.map((block, index) => 
                this.parseContentBlock(block, index)
            );
        }
    }

    parseStreaming() {
        // 简化流式解析，专注于最终内容
        const events = this.parseSSEEvents();
        const mergedData = this.mergeStreamEvents(events);
        
        this.parsed.metadata = mergedData.metadata;
        this.parsed.usage = this.calculateUsage(mergedData.usage);
        this.parsed.content = mergedData.content;
        this.parsed.streamingInfo = {
            totalEvents: events.length,
            eventTypes: [...new Set(events.map(e => e.type))]
        };
    }

    parseSSEEvents() {
        const events = [];
        const lines = this.rawResponse.split('\n');
        let currentEvent = {};
        
        for (const line of lines) {
            if (line.startsWith('event: ')) {
                if (currentEvent.type) {
                    events.push({ ...currentEvent });
                }
                currentEvent = { type: line.substring(7) };
            } else if (line.startsWith('data: ')) {
                try {
                    currentEvent.data = JSON.parse(line.substring(6));
                } catch (e) {
                    currentEvent.data = line.substring(6);
                }
            }
        }
        
        if (currentEvent.type) {
            events.push(currentEvent);
        }
        
        return events;
    }

    mergeStreamEvents(events) {
        const result = { metadata: {}, usage: {}, content: [] };
        let contentBlocks = [];
        
        for (const event of events) {
            switch (event.type) {
                case 'message_start':
                    if (event.data && event.data.message) {
                        result.metadata = {
                            id: event.data.message.id,
                            model: event.data.message.model,
                            role: event.data.message.role,
                            isStreaming: true,
                            completedAt: new Date().toISOString()
                        };
                        if (event.data.message.usage) {
                            Object.assign(result.usage, event.data.message.usage);
                        }
                    }
                    break;
                    
                case 'content_block_start':
                    if (event.data) {
                        contentBlocks[event.data.index] = {
                            type: event.data.content_block.type,
                            content: event.data.content_block.text || '',
                            id: event.data.content_block.id || null,
                            name: event.data.content_block.name || null
                        };
                    }
                    break;
                    
                case 'content_block_delta':
                    if (event.data && contentBlocks[event.data.index]) {
                        if (event.data.delta.type === 'text_delta') {
                            contentBlocks[event.data.index].content += event.data.delta.text;
                        } else if (event.data.delta.type === 'input_json_delta') {
                            if (!contentBlocks[event.data.index].input_json) {
                                contentBlocks[event.data.index].input_json = '';
                            }
                            contentBlocks[event.data.index].input_json += event.data.delta.partial_json;
                        }
                    }
                    break;
                    
                case 'content_block_stop':
                    if (event.data && contentBlocks[event.data.index]) {
                        const block = contentBlocks[event.data.index];
                        if (block.input_json) {
                            try {
                                block.input = JSON.parse(block.input_json);
                                delete block.input_json;
                            } catch (e) {
                                // 保持原始字符串
                                block.input = block.input_json;
                                delete block.input_json;
                            }
                        }
                    }
                    break;
                    
                case 'message_delta':
                    if (event.data) {
                        if (event.data.delta && event.data.delta.stop_reason) {
                            result.metadata.stop_reason = event.data.delta.stop_reason;
                        }
                        if (event.data.usage) {
                            Object.assign(result.usage, event.data.usage);
                        }
                    }
                    break;
            }
        }

        result.content = contentBlocks.map((block, index) => 
            this.parseContentBlock(block, index)
        ).filter(Boolean);

        return result;
    }

    parseContentBlock(block, index) {
        if (!block) return null;
        
        const baseBlock = {
            index,
            type: block.type,
            metadata: {}
        };

        switch (block.type) {
            case 'text':
                return {
                    ...baseBlock,
                    content: block.text || block.content || '',
                    metadata: {
                        characterCount: (block.text || block.content || '').length,
                        wordCount: (block.text || block.content || '').split(/\s+/).filter(w => w.length > 0).length
                    }
                };
                
            case 'tool_use':
                return {
                    ...baseBlock,
                    content: {
                        id: block.id,
                        name: block.name,
                        input: block.input
                    },
                    metadata: {
                        inputSize: JSON.stringify(block.input || {}).length
                    }
                };
                
            case 'thinking':
                return {
                    ...baseBlock,
                    content: block.content || '',
                    metadata: {
                        characterCount: (block.content || '').length,
                        isVisible: false
                    }
                };
                
            default:
                return {
                    ...baseBlock,
                    content: block,
                    metadata: {}
                };
        }
    }

    calculateUsage(rawUsage) {
        const usage = {
            input_tokens: rawUsage.input_tokens || 0,
            output_tokens: rawUsage.output_tokens || 0,
            cache_creation_input_tokens: rawUsage.cache_creation_input_tokens || 0,
            cache_read_input_tokens: rawUsage.cache_read_input_tokens || 0
        };
        
        // 计算衍生数据
        usage.total_input_tokens = usage.input_tokens + usage.cache_creation_input_tokens + usage.cache_read_input_tokens;
        usage.total_tokens = usage.total_input_tokens + usage.output_tokens;
        
        // 计算 cache 效率
        if (usage.total_input_tokens > 0) {
            usage.cache_efficiency = ((usage.cache_read_input_tokens / usage.total_input_tokens) * 100).toFixed(1);
        } else {
            usage.cache_efficiency = 0;
        }
        
        // 计算输出比例
        if (usage.total_tokens > 0) {
            usage.output_ratio = ((usage.output_tokens / usage.total_tokens) * 100).toFixed(1);
        } else {
            usage.output_ratio = 0;
        }
        
        return usage;
    }
}