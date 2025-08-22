# 多语言支持增强方案

## 问题分析

### 当前实现的局限性

1. **覆盖范围有限**
   - 只支持HTML标签包围的文本：`<tag data-t="key">content</tag>`
   - 无法处理JavaScript中的字符串（提示、错误消息等）
   - 无法处理Go代码中的字符串（后端错误消息、日志等）
   - 无法处理HTML中的纯文本（没有被标签包围的内容）

2. **技术限制**
   - 基于正则表达式的处理方式，对复杂嵌套支持有限
   - 自闭合标签支持不完整
   - 只能在运行时处理HTML响应

3. **开发体验**
   - 需要手动添加标记属性
   - 无法快速识别哪些文本需要翻译
   - 缺乏自动化工具支持

## 增强方案设计

### 核心理念

保持GNU gettext风格：**用主要语言（中文）开发，运行时动态替换**，同时确保代码可读性和开发效率。

### 1. 多层级标记语法

#### 1.1 HTML标签内容翻译（保持现有）
```html
<h1 data-t="dashboard_title">控制台</h1>
<button data-t="save_button">保存</button>
```

#### 1.2 HTML纯文本翻译（新增）
```html
<!-- 现有方式，需要额外标签 -->
<span data-t="welcome_message">欢迎使用Claude Code伴侣</span>

<!-- 新增方式，支持纯文本 -->
<!--T:welcome_message-->欢迎使用Claude Code伴侣<!--/T-->
```

#### 1.3 HTML属性翻译（增强）
```html
<!-- 现有方式 -->
<input data-t="username_placeholder" placeholder="请输入用户名">

<!-- 增强方式，支持多属性 -->
<input data-t-placeholder="username_placeholder" 
       data-t-title="username_tooltip" 
       placeholder="请输入用户名" 
       title="用户名提示">
```

#### 1.4 Go代码翻译（新增）
```go
// 使用翻译函数包装
func (s *Server) handleError(c *gin.Context, err error) {
    message := T("server_error", "服务器内部错误")
    c.JSON(500, gin.H{"error": message})
}

// 支持参数化翻译
func (s *Server) showUserInfo(username string) {
    message := Tf("welcome_user", "欢迎 %s", username)
    s.logger.Info(message)
}

// 上下文感知翻译
func (s *Server) translateForRequest(c *gin.Context, key, fallback string) string {
    return TCtx(c, key, fallback)
}
```

#### 1.5 JavaScript翻译（新增）
```javascript
// 全局翻译函数
function showError() {
    alert(T('connection_failed', '连接失败'));
}

// 参数化翻译
function showWelcome(username) {
    const message = Tf('welcome_user', '欢迎 %s', username);
    document.getElementById('welcome').textContent = message;
}

// DOM元素翻译
function updateUI() {
    translateElement('#status', 'connection_status', '连接状态');
    translateAttribute('#input', 'placeholder', 'username_placeholder', '请输入用户名');
}
```

### 2. 翻译处理引擎

#### 2.1 多阶段处理管道

```
源代码/模板 → 扫描器 → 标记处理 → 翻译引擎 → 输出
     ↓           ↓         ↓          ↓        ↓
  原始内容   识别翻译点  添加标记   查找翻译   替换内容
```

#### 2.2 处理器架构

```go
type TranslationProcessor interface {
    // 处理特定类型的内容
    Process(content string, lang Language, ctx Context) (string, error)
    
    // 提取翻译键值对
    Extract(content string) (map[string]string, error)
    
    // 验证翻译标记
    Validate(content string) []ValidationError
}

type ProcessorChain struct {
    processors []TranslationProcessor
}

// 具体处理器
type HTMLTagProcessor struct{}      // 处理HTML标签
type HTMLTextProcessor struct{}     // 处理HTML纯文本  
type HTMLAttrProcessor struct{}     // 处理HTML属性
type GoCodeProcessor struct{}       // 处理Go代码翻译函数
type JSCodeProcessor struct{}       // 处理JS翻译函数
```

#### 2.3 翻译函数实现

```go
// internal/i18n/functions.go

// 基础翻译函数
func T(key, fallback string) string {
    return GetGlobalManager().GetTranslation(key, getCurrentLanguage())
}

// 格式化翻译函数
func Tf(key, fallback string, args ...interface{}) string {
    translation := T(key, fallback)
    return fmt.Sprintf(translation, args...)
}

// 上下文感知翻译函数
func TCtx(c *gin.Context, key, fallback string) string {
    lang := GetLanguageFromContext(c)
    return GetGlobalManager().GetTranslation(key, lang)
}

// 复数形式翻译（可选）
func TPlural(key, singular, plural string, count int) string {
    // 实现复数规则
}
```

### 3. 自动化工具支持

#### 3.1 翻译提取工具

```bash
# 扫描所有文件，提取翻译文本
./tools/extract-translations.sh

# 生成翻译模板
./tools/generate-template.sh --lang en

# 验证翻译完整性
./tools/validate-translations.sh
```

#### 3.2 开发时检查

```go
// tools/translation-checker/main.go
// 检查是否有遗漏的翻译标记
// 验证翻译文件的完整性
// 生成翻译报告
```

### 4. 翻译文件管理

#### 4.1 分层翻译文件

```
web/locales/
├── en/
│   ├── common.json      # 通用翻译
│   ├── dashboard.json   # 控制台页面
│   ├── endpoints.json   # 端点管理
│   ├── errors.json      # 错误消息
│   └── javascript.json  # JS翻译
├── ja/
└── ...
```

#### 4.2 翻译文件格式

```json
{
  "meta": {
    "version": "1.0",
    "language": "en",
    "last_updated": "2024-01-01T00:00:00Z"
  },
  "translations": {
    "dashboard_title": "Dashboard",
    "welcome_user": "Welcome %s",
    "connection_status": "Connection Status",
    "errors": {
      "server_error": "Internal Server Error",
      "connection_failed": "Connection Failed"
    }
  }
}
```

### 5. 性能优化

#### 5.1 缓存策略

```go
type TranslationCache struct {
    // 内存缓存已处理的HTML模板
    templateCache map[string]map[Language]string
    
    // 翻译结果缓存
    translationCache map[string]map[Language]string
    
    // 缓存失效策略
    ttl time.Duration
}
```

#### 5.2 懒加载

```go
// 只在需要时加载翻译文件
type LazyTranslationLoader struct {
    loadedLanguages map[Language]bool
    translationData map[Language]map[string]string
}
```

### 6. 实施计划

#### 阶段1：核心引擎增强
- [ ] 扩展翻译处理器，支持新的标记语法
- [ ] 实现Go翻译函数
- [ ] 增强HTML处理能力

#### 阶段2：前端支持
- [ ] 实现JavaScript翻译API
- [ ] 动态翻译DOM元素
- [ ] 前端翻译缓存

#### 阶段3：工具链完善
- [ ] 翻译提取工具
- [ ] 自动化验证工具
- [ ] 开发时检查工具

#### 阶段4：性能优化
- [ ] 缓存机制实现
- [ ] 懒加载优化
- [ ] 处理性能测试

### 7. 向后兼容

- 保持现有 `data-t` 语法100%兼容
- 现有翻译文件继续有效
- 渐进式迁移，无需一次性重构所有代码

### 8. 示例对比

#### 现有方式（受限）
```html
<!-- HTML: 只能翻译标签内容 -->
<h1 data-t="title">控制台</h1>
<span data-t="status">运行中</span>

<!-- 纯文本无法翻译 -->
<div>系统状态：<span data-t="status">运行中</span></div>
```

```javascript
// JavaScript: 无法翻译
function showError() {
    alert('连接失败'); // 硬编码中文
}
```

```go
// Go: 无法翻译
func handleError() {
    log.Error("服务器错误") // 硬编码中文
}
```

#### 增强后方式（全覆盖）
```html
<!-- HTML: 完整支持 -->
<h1 data-t="title">控制台</h1>
<div><!--T:system_status-->系统状态<!--/T-->：<span data-t="status">运行中</span></div>
<input data-t-placeholder="username_hint" placeholder="请输入用户名">
```

```javascript
// JavaScript: 完整支持
function showError() {
    alert(T('connection_failed', '连接失败'));
}

function updateStatus(isRunning) {
    const status = isRunning ? T('running', '运行中') : T('stopped', '已停止');
    document.getElementById('status').textContent = status;
}
```

```go
// Go: 完整支持
func handleError(c *gin.Context) {
    message := TCtx(c, "server_error", "服务器错误")
    log.Error(message)
    c.JSON(500, gin.H{"error": message})
}

func logUserAction(username, action string) {
    log.Info(Tf("user_action", "用户 %s 执行了 %s", username, action))
}
```

## 总结

这个增强方案在保持现有功能和向后兼容的基础上，大幅扩展了多语言支持的覆盖范围，实现了：

1. **全栈翻译支持**：HTML、JavaScript、Go代码全覆盖
2. **保持开发体验**：继续用中文开发，运行时翻译
3. **增强的灵活性**：支持纯文本、属性、参数化翻译
4. **自动化工具**：提取、验证、生成工具支持
5. **性能优化**：缓存和懒加载机制

通过这个方案，可以实现真正意义上的全面多语言支持，同时保持良好的开发体验和代码可读性。