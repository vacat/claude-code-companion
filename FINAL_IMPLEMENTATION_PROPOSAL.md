# TodoWrite 工具调用格式修正 - 最终实现方案

## 实施优先级和阶段

### 第一阶段：核心修正功能 (高优先级)

#### 1.1 创建 PythonJSONFixer 组件

**文件位置**：`internal/conversion/python_json_fixer.go`

**核心功能**：
- 检测 Python 风格的字典语法
- 智能转换单引号为双引号
- 验证转换后的 JSON 有效性
- 特化处理 TodoWrite 工具的格式

**关键方法**：
```go
type PythonJSONFixer struct {
    logger *logger.Logger
}

func NewPythonJSONFixer(logger *logger.Logger) *PythonJSONFixer
func (f *PythonJSONFixer) FixPythonStyleJSON(input string) (string, bool)
func (f *PythonJSONFixer) DetectPythonStyle(input string) bool
func (f *PythonJSONFixer) isStructuralQuote(runes []rune, pos int) bool
```

#### 1.2 增强 SSE 解析器

**修改文件**：`internal/conversion/sse_parser.go`

**修改位置**：第 48-57 行的 JSON 解析错误处理

**修改逻辑**：
1. JSON 解析失败时，调用 PythonJSONFixer
2. 如果修正成功，重新尝试解析
3. 记录修正操作的详细日志
4. 保持向后兼容性

#### 1.3 增强 SimpleJSONBuffer

**修改文件**：`internal/conversion/simple_json_buffer.go`

**新增方法**：
```go
func (b *SimpleJSONBuffer) AppendFragmentWithFix(fragment string, toolName string)
func (b *SimpleJSONBuffer) tryFixPythonStyle()
func (b *SimpleJSONBuffer) GetFixedBufferedContent() string
```

**增强逻辑**：
- 实时检测缓冲内容中的格式问题
- 针对 TodoWrite 工具进行特殊处理
- 保持增量输出的正确性

### 第二阶段：集成和配置 (中优先级)

#### 2.1 修改事件处理器

**修改文件**：`internal/conversion/response_converter_events.go`

**修改位置**：第 165 行

**修改内容**：
```go
// 原来：state.JSONBuffer.AppendFragment(tc.Function.Arguments)
// 修改为：state.JSONBuffer.AppendFragmentWithFix(tc.Function.Arguments, state.Name)
```

#### 2.2 添加配置支持

**修改文件**：`internal/config/types.go`

**新增配置结构**：
```go
type PythonJSONFixingConfig struct {
    Enabled       bool     `yaml:"enabled" json:"enabled"`
    TargetTools   []string `yaml:"target_tools" json:"target_tools"`
    DebugLogging  bool     `yaml:"debug_logging" json:"debug_logging"`
    MaxAttempts   int      `yaml:"max_attempts" json:"max_attempts"`
}

type ConversionConfig struct {
    // ... 现有字段
    PythonJSONFixing PythonJSONFixingConfig `yaml:"python_json_fixing" json:"python_json_fixing"`
}
```

### 第三阶段：测试和验证 (中优先级)

#### 3.1 单元测试

**创建文件**：`internal/conversion/python_json_fixer_test.go`

**测试覆盖**：
- 基本的 Python 字典转换
- TodoWrite 特定格式转换
- 嵌套结构处理
- 边界条件和错误情况
- 性能测试

#### 3.2 集成测试

**创建文件**：`internal/conversion/integration_fix_test.go`

**测试场景**：
- 完整的 SSE 流处理
- 实际的工具调用场景
- 混合格式处理
- 错误恢复机制

### 第四阶段：监控和优化 (低优先级)

#### 4.1 性能监控

**新增文件**：`internal/conversion/fixing_metrics.go`

**监控指标**：
- 修正尝试次数
- 成功/失败比率
- 处理时间统计
- 影响的请求数量

#### 4.2 管理界面集成

**修改文件**：`internal/web/admin.go`

**新增功能**：
- 格式修正统计显示
- 实时修正日志查看
- 配置动态调整界面

## 风险评估和缓解措施

### 高风险项

1. **数据损坏风险**
   - **缓解**：严格的 JSON 验证，修正失败时回退到原数据
   - **测试**：大量边界条件测试

2. **性能影响**
   - **缓解**：按需触发，仅对失败的 JSON 解析进行修正
   - **监控**：详细的性能指标记录

### 中风险项

1. **兼容性问题**
   - **缓解**：保持配置开关，允许禁用功能
   - **测试**：多模型兼容性测试

2. **误修正**
   - **缓解**：智能检测算法，避免修改正确的 JSON
   - **验证**：双重验证机制

## 实施时间表

### 第 1 周：核心功能开发
- 天 1-2：PythonJSONFixer 开发和单元测试
- 天 3-4：SSE 解析器增强
- 天 5-6：SimpleJSONBuffer 增强
- 天 7：初步集成测试

### 第 2 周：集成和配置
- 天 1-2：事件处理器修改
- 天 3-4：配置系统集成
- 天 5-6：全面集成测试
- 天 7：性能测试和优化

### 第 3 周：测试和部署
- 天 1-3：完整测试套件开发
- 天 4-5：文档完善
- 天 6-7：生产环境部署准备

## 成功标准

### 功能标准
- [ ] 能正确检测并修正 Python 风格的 TodoWrite 工具调用
- [ ] 修正后的 JSON 格式完全兼容现有系统
- [ ] 不影响其他正常工具调用的处理
- [ ] 提供详细的日志和监控信息

### 性能标准
- [ ] 修正操作延迟小于 5ms
- [ ] 内存开销增加小于 10%
- [ ] CPU 开销增加小于 5%
- [ ] 99% 的修正成功率

### 质量标准
- [ ] 单元测试覆盖率 > 90%
- [ ] 集成测试覆盖主要场景
- [ ] 错误处理机制完善
- [ ] 详细的操作文档

## 后续改进方向

1. **机器学习增强**：基于历史数据训练更智能的格式检测模型
2. **多格式支持**：扩展到其他非标准 JSON 格式的修正
3. **实时优化**：根据运行时数据动态调整修正策略
4. **可视化工具**：提供格式修正的可视化调试界面

## 总结

这个实现方案通过最小侵入性的修改来解决 Python 风格 JSON 格式问题，确保：

1. **高可靠性**：多重回退机制和严格验证
2. **高性能**：按需触发和增量处理
3. **高兼容性**：保持现有功能不受影响
4. **高可维护性**：清晰的代码结构和详细文档

该方案已经准备好进行实施，建议按照上述时间表逐步推进。