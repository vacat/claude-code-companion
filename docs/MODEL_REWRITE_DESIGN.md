# Model Rewrite 功能设计文档

## 需求概述

为 claude-proxy 的 endpoint 增加 model rewrite 功能，允许在请求发送到上游 endpoint 之前对模型名称进行重写。主要场景是将 Claude 模型请求重写为其他提供商的模型（如 DeepSeek）。

## 功能设计

### 1. 核心功能

- **模型重写规则**：基于通配符模式匹配，将符合条件的模型名重写为目标模型名
- **默认行为**：不进行任何重写，保持向后兼容
- **生效范围**：仅在请求发送到特定 endpoint 时生效
- **配置粒度**：每个 endpoint 可以独立配置不同的重写规则

### 2. 配置文件设计

在 `EndpointConfig` 结构中新增 `ModelRewrite` 字段：

```yaml
endpoints:
  - name: "deepseek-endpoint"
    url: "https://api.deepseek.com/v1"
    auth_type: "bearer"
    auth_value: "sk-xxx"
    enabled: true
    priority: 1
    model_rewrite:                    # 新增：模型重写配置
      enabled: true                   # 是否启用重写功能，默认 false
      rules:                          # 重写规则列表
        - source_pattern: "claude-3*haiku*"    # 源模型通配符
          target_model: "deepseek-chat"        # 目标模型名
        - source_pattern: "claude-3*sonnet*"
          target_model: "deepseek-chat"
        - source_pattern: "claude-*opus*"
          target_model: "deepseek-chat"
```

### 3. Claude 模型通配符模式

基于对 Claude 官方模型命名规则的分析，设计以下预设通配符模式：

#### 当前 Claude 模型命名规律

- Haiku: `claude-3-haiku-*`, `claude-3-5-haiku-*`
- Sonnet: `claude-3-5-sonnet-*`, `claude-3-7-sonnet-*`, `claude-sonnet-4-*`
- Opus: `claude-3-opus-*`, `claude-opus-4-*`, `claude-opus-4-1-*`

#### 推荐的通配符模式

- **Haiku 系列**: `claude-*haiku*` (匹配所有包含 haiku 的模型)
- **Sonnet 系列**: `claude-*sonnet*` (匹配所有包含 sonnet 的模型)
- **Opus 系列**: `claude-*opus*` (匹配所有包含 opus 的模型)
- **所有 Claude**: `claude-*` (匹配所有 claude 模型)

### 4. WebUI 配置界面设计

在 endpoint 配置页面新增 "Model Rewrite" 配置区块：

#### 4.1 界面元素

- **启用开关**: 是否启用 model rewrite 功能
- **规则列表**: 可动态添加/删除规则
- **每条规则包含**:
  - 源模型选择器：下拉选择 + 自定义输入
  - 目标模型输入框：手工输入目标模型名

#### 4.2 源模型选择器设计

提供预设选项 + 自定义输入：

```html
<select name="source_model_type" onchange="updateSourcePattern()">
  <option value="">请选择预设模型</option>
  <option value="haiku">Haiku 系列 (claude-*haiku*)</option>
  <option value="sonnet">Sonnet 系列 (claude-*sonnet*)</option>
  <option value="opus">Opus 系列 (claude-*opus*)</option>
  <option value="all_claude">所有 Claude (claude-*)</option>
  <option value="custom">自定义通配符</option>
</select>

<input type="text" name="source_pattern" placeholder="通配符模式" readonly />
```

当选择 "自定义通配符" 时，输入框变为可编辑状态。

### 5. 实现架构

#### 5.1 数据结构定义

```go
// 在 config/config.go 中
type EndpointConfig struct {
    Name        string              `yaml:"name"`
    URL         string              `yaml:"url"`
    // ... 其他现有字段
    ModelRewrite *ModelRewriteConfig `yaml:"model_rewrite,omitempty"`
}

type ModelRewriteConfig struct {
    Enabled bool                     `yaml:"enabled"`
    Rules   []ModelRewriteRule       `yaml:"rules"`
}

type ModelRewriteRule struct {
    SourcePattern string `yaml:"source_pattern"`  // 支持通配符的源模型模式
    TargetModel   string `yaml:"target_model"`    // 目标模型名
}
```

#### 5.2 重写逻辑实现

在代理处理请求时添加 model rewrite 步骤：

```go
// 在 proxy 包中添加函数
func (p *ProxyHandler) rewriteModelInRequest(req *http.Request, endpoint *endpoint.Endpoint) error {
    if endpoint.Config.ModelRewrite == nil || !endpoint.Config.ModelRewrite.Enabled {
        return nil // 未启用重写
    }
  
    // 解析请求体，提取 model 字段
    body, err := io.ReadAll(req.Body)
    if err != nil {
        return err
    }
  
    var requestData map[string]interface{}
    if err := json.Unmarshal(body, &requestData); err != nil {
        return err // 非 JSON 请求，跳过重写
    }
  
    modelField, exists := requestData["model"]
    if !exists {
        return nil // 没有 model 字段，跳过重写
    }
  
    originalModel, ok := modelField.(string)
    if !ok {
        return nil // model 字段非字符串，跳过重写
    }
  
    // 应用重写规则
    newModel := p.applyRewriteRules(originalModel, endpoint.Config.ModelRewrite.Rules)
    if newModel != originalModel {
        requestData["model"] = newModel
        newBody, err := json.Marshal(requestData)
        if err != nil {
            return err
        }
      
        // 更新请求体
        req.Body = io.NopCloser(bytes.NewReader(newBody))
        req.ContentLength = int64(len(newBody))
      
        // 记录重写日志
        logger.Info("Model rewritten", "original", originalModel, "new", newModel, "endpoint", endpoint.Name)
    }
  
    return nil
}

func (p *ProxyHandler) applyRewriteRules(originalModel string, rules []ModelRewriteRule) string {
    for _, rule := range rules {
        if matched, _ := filepath.Match(rule.SourcePattern, originalModel); matched {
            return rule.TargetModel
        }
    }
    return originalModel // 没有匹配的规则，返回原模型名
}
```

### 6. WebUI API 接口

#### 6.1 获取 endpoint 配置

```
GET /api/endpoints/{name}
```

#### 6.2 更新 endpoint model rewrite 配置

```
PUT /api/endpoints/{name}/model-rewrite
Content-Type: application/json

{
  "enabled": true,
  "rules": [
    {
      "source_pattern": "claude-*haiku*",
      "target_model": "deepseek-chat"
    }
  ]
}
```

#### 6.3 测试模型重写规则

```
POST /api/endpoints/{name}/test-model-rewrite
Content-Type: application/json

{
  "test_model": "claude-3-haiku-20240307"
}

Response:
{
  "original_model": "claude-3-haiku-20240307",
  "rewritten_model": "deepseek-chat",
  "matched_rule": "claude-*haiku*"
}
```

## 需要讨论的问题

### 1. 通配符语法选择

**当前方案**: 使用 Go 标准库 `filepath.Match` 的通配符语法

- `*` 匹配任意字符序列
- `?` 匹配单个字符
- `[]` 字符类匹配

**问题**: 是否需要支持更复杂的正则表达式？

**建议**: 先使用简单通配符，如需要再扩展支持正则表达式

回答： 使用简单通配符即可


### 2. 重写规则优先级

**当前方案**: 按配置文件中规则的顺序匹配，匹配到第一个规则后立即应用

**问题**: 是否需要支持规则优先级配置？

**建议**: 当前方案足够简单清晰，暂不增加优先级复杂度

回答： 当前方案就可以了

### 3. 预设通配符的维护策略

**问题**: Claude 模型命名规则可能会变化，预设通配符如何保持有效？

**建议方案**:

- 使用保守的通配符模式（如 `claude-*haiku*`），兼容性更强
- 在代码中维护一个模型名称检测函数，用于验证通配符有效性
- 提供配置验证 API，用户可以测试通配符是否按预期工作

  回答：保守通配符模式即可

### 4. 错误处理策略

**问题**: 当 model rewrite 过程中出现错误时如何处理？

**建议方案**:

- JSON 解析错误：跳过重写，使用原始请求
- 规则匹配错误：记录日志但不中断请求
- 请求体重构错误：返回 500 错误给客户端

回答：完全按照建议方案来

### 5. 性能考虑

**问题**: 每个请求都要解析 JSON 和匹配规则，是否会影响性能？

**分析**:

- JSON 解析：对于 Claude API 这种请求体通常较小的场景，影响可忽略
- 规则匹配：通配符匹配复杂度 O(n*m)，n 是规则数，m 是模型名长度，通常都很小
- 缓存优化：可以考虑对热点模型名进行匹配结果缓存

**建议**: 先实现基础功能，如发现性能问题再优化

回答：完全按照建议来


### 6. 配置热重载

**问题**: 修改 model rewrite 配置后是否需要重启服务？

**当前状况**: 项目目前没有配置热重载机制

**建议**: 保持与现有配置管理方式一致，暂不实现热重载

回答：现在虽然没有热重载，但是我怎么试验似乎有类似的效果，改了即时生效？请确认一下，这里还是希望可以有热重载的效果的

**调研结果**：经过代码分析，项目实际上**已经支持热重载**！

- **热重载接口**：`PUT /admin/api/hot-update` (internal/web/handlers.go:610)
- **支持热更新的配置**：endpoints、logging、validation、tagging 等
- **热更新流程**：
  1. 保存配置文件 (`config.SaveConfig`)
  2. 调用热更新处理器 (`hotUpdateHandler.HotUpdateConfig`)  
  3. 更新运行时配置 (`updateEndpoints`, `updateLoggingConfig` 等)
- **不支持热更新的配置**：server.host, server.port (需要重启)

**结论**：Model rewrite 配置作为 endpoint 配置的一部分，天然支持热重载！


## 实现计划

### Phase 1: 基础功能实现

1. 扩展配置结构定义
2. 实现核心 model rewrite 逻辑
3. 集成到代理请求处理流程
4. 添加相关日志记录

### Phase 2: WebUI 支持

1. 扩展 endpoint 配置 API
2. 实现 WebUI 配置界面
3. 添加规则测试功能

### Phase 3: 增强功能

1. 配置验证和错误处理完善
2. 性能优化（如需要）
3. 文档和示例完善

## 开放性问题

1. **是否需要支持响应中的 model 字段重写**？

   - 某些 API 响应中也包含 model 字段，是否需要将其改回原始模型名？

     回答： 有的话就将其改回去
2. **是否需要支持条件重写**？

   - 例如根据请求来源IP、用户标识等条件决定是否应用重写规则
     回答：不需要
3. **是否需要重写统计功能**？

   - 在 WebUI 中显示各个重写规则的使用次数、成功率等统计信息
     回答：有的话最好，没有的话也不强求

## 需要澄清的问题 - 已解决

### DeepSeek API 模型名称调研

**调研结果**：根据官方文档，DeepSeek API 目前提供两个主要模型：

1. **`deepseek-chat`** - 指向 DeepSeek-V3-0324，成本较低，适合一般对话
2. **`deepseek-reasoner`** - 指向 DeepSeek-R1-0528，成本较高，适合复杂推理

**建议**：配置示例中统一使用 `deepseek-chat` 作为目标模型，这是最通用的选择。

### 响应重写机制设计

既然您确认需要"将响应中的 model 字段改回原始模型名"，需要在设计中添加响应处理逻辑：

```go
// 新增：响应重写功能
func (p *ProxyHandler) rewriteModelInResponse(responseBody []byte, originalModel string, rewrittenModel string) ([]byte, error) {
    var responseData map[string]interface{}
    if err := json.Unmarshal(responseBody, &responseData); err != nil {
        return responseBody, nil // 非JSON响应，返回原始内容
    }
    
    // 检查是否有 model 字段且等于重写后的模型名
    if modelField, exists := responseData["model"]; exists {
        if modelStr, ok := modelField.(string); ok && modelStr == rewrittenModel {
            responseData["model"] = originalModel
            return json.Marshal(responseData)
        }
    }
    
    return responseBody, nil
}
```

### 统计功能设计（可选实现）

如果实现统计功能，建议在 endpoint 管理器中添加：
- 重写规则命中次数
- 每个规则的使用频率
- 重写成功/失败统计
- WebUI 中展示统计图表

设计确认无误，可以开始实现编码。
