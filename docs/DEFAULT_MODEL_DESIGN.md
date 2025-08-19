# 默认模型配置设计文档

## 需求概述

为端点配置增加**默认模型**配置，该配置与现有的**模型重写规则**互斥：

1. **互斥关系**：
   - 当默认模型配置有值时，模型重写规则不允许开启
   - 当模型重写规则开启时，默认模型配置不能修改且为空值

2. **功能行为**：
   - 默认模型在实际执行时相当于一条 `*` → `默认模型` 的重写规则
   - 对于万用端点（无tags）：
     - 如果配置了默认模型，则隐式规则（非claude* → claude-sonnet-4-20250514）失效
     - 如果未配置默认模型，则隐式规则有效

## 当前实现分析

### 现有结构
- `EndpointConfig.ModelRewrite` - 模型重写配置
- `ModelRewriteConfig.Enabled` - 是否启用模型重写
- `ModelRewriteConfig.Rules` - 重写规则列表

### 隐式规则位置
在 `internal/modelrewrite/rewriter.go:73-84` 中实现了万用端点的隐式规则：
```go
if isGenericEndpoint && !strings.HasPrefix(originalModel, "claude") {
    // 通用端点的隐式规则：非claude模型重写为claude-sonnet-4-20250514
    rules = []config.ModelRewriteRule{
        {
            SourcePattern: "*",
            TargetModel:   "claude-sonnet-4-20250514",
        },
    }
}
```

## 设计方案

### 1. 数据结构修改

在 `EndpointConfig` 中添加默认模型字段：
```go
type EndpointConfig struct {
    // ... 现有字段
    DefaultModel      string              `yaml:"default_model,omitempty" json:"default_model,omitempty"`  // 新增：默认模型配置
    ModelRewrite      *ModelRewriteConfig `yaml:"model_rewrite,omitempty"`                                 // 现有：模型重写配置
    // ... 其他字段
}
```

### 2. 验证逻辑

添加配置验证，确保互斥关系：
- `DefaultModel` 不为空时，`ModelRewrite.Enabled` 必须为 false
- `ModelRewrite.Enabled` 为 true 时，`DefaultModel` 必须为空

### 3. 重写逻辑修改

修改 `RewriteRequestWithTags` 方法的规则确定逻辑：

```go
func (r *Rewriter) RewriteRequestWithTags(req *http.Request, modelRewriteConfig *config.ModelRewriteConfig, endpointTags []string, defaultModel string) (string, string, error) {
    // ... 现有解析逻辑
    
    // 确定重写规则
    var rules []config.ModelRewriteRule
    isGenericEndpoint := len(endpointTags) == 0
    hasExplicitRules := modelRewriteConfig != nil && modelRewriteConfig.Enabled && len(modelRewriteConfig.Rules) > 0
    hasDefaultModel := strings.TrimSpace(defaultModel) != ""

    if hasExplicitRules {
        // 使用显式配置的模型重写规则
        rules = modelRewriteConfig.Rules
    } else if hasDefaultModel {
        // 使用默认模型配置（相当于 * → defaultModel 规则）
        rules = []config.ModelRewriteRule{
            {
                SourcePattern: "*",
                TargetModel:   defaultModel,
            },
        }
    } else if isGenericEndpoint && !strings.HasPrefix(originalModel, "claude") {
        // 万用端点的隐式规则：非claude模型重写为claude-sonnet-4-20250514
        rules = []config.ModelRewriteRule{
            {
                SourcePattern: "*",
                TargetModel:   "claude-sonnet-4-20250514",
            },
        }
    } else {
        // 没有规则应用
        return "", "", nil
    }
    
    // ... 其余逻辑不变
}
```

### 4. Web界面修改

更新端点配置模态框：
- 添加默认模型输入字段
- 实现互斥UI逻辑：
  - 默认模型有值时，禁用模型重写开关
  - 模型重写开启时，禁用默认模型输入框
- 添加相关提示信息

### 5. 优先级顺序

规则应用的优先级：
1. **显式模型重写规则**（最高优先级）
2. **默认模型配置**
3. **万用端点隐式规则**（最低优先级）

## 实现步骤

1. 修改 `EndpointConfig` 数据结构
2. 添加配置验证逻辑
3. 修改模型重写器，支持默认模型参数
4. 更新调用方，传递默认模型参数
5. 更新Web界面
6. 添加相关测试

## 预期效果

- **简化配置**：用户可以直接设置默认模型，无需创建复杂的重写规则
- **保持兼容**：现有的模型重写功能完全保持不变
- **避免冲突**：通过互斥设计，避免配置冲突和意外行为
- **清晰优先级**：明确的规则应用顺序，行为可预测