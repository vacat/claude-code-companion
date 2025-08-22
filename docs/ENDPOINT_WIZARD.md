# 端点添加向导设计文档

## 概述

端点添加向导是一个简化的端点配置流程，通过预设配置模板帮助用户快速添加常用的API端点，而无需手动填写所有配置参数。

## 设计目标

1. **简化用户体验** - 将复杂的端点配置过程简化为3步向导流程
2. **预设配置管理** - 通过配置文件管理常用端点的预设信息
3. **电子表格友好** - 支持从在线电子表格导出预设配置文件
4. **与现有系统集成** - 与现有端点管理功能无缝集成

## 配置文件设计

### 文件位置
```
endpoint_profiles.yaml
```

### 配置格式
```yaml
profiles:
  - profile_id: "anthropic"           # 预设标识符
    display_name: "Anthropic (Claude)" # 显示名称
    url: "https://api.anthropic.com"   # API地址
    endpoint_type: "anthropic"         # 端点类型
    auth_type: "api_key"              # 认证方式
    path_prefix: ""                   # 路径前缀
    override_max_tokens: null         # 强制max_tokens值
    require_default_model: false      # 是否必须填写默认模型
    default_model_options: ""         # 预定义模型选项(逗号分隔)
```

### 字段说明
- `profile_id`: 预设的唯一标识符，用于程序引用
- `display_name`: 用户界面显示的友好名称，用于自动生成端点名称
- `url`: API端点地址，配置文件中必须提供
- `endpoint_type`: `anthropic` 或 `openai`
- `auth_type`: `api_key` 或 `auth_token`
- `path_prefix`: API路径前缀，配置文件中必须提供
- `override_max_tokens`: 某些端点(如DeepSeek)需要强制设置的max_tokens值
- `require_default_model`: 是否要求用户必须填写默认模型
- `default_model_options`: 预定义的模型选项，逗号分隔，空值表示无预设选项

## 向导流程设计

### 第一步：预设选择
- **标题**: "选择端点类型"
- **内容**: 显示所有可用的预设配置下拉选择框
- **交互**: 用户从下拉框中选择一个预设配置
- **UI**: 对话框中央的下拉选择框，显示预设的`display_name`

### 第二步：基本配置
- **标题**: "配置端点信息"
- **内容**: 根据选中的预设动态显示需要用户填写的字段
- **字段逻辑**:
  - **端点名称**: 根据预设的`display_name`自动生成，检查与现有端点重复时自动添加后缀`(1)`、`(2)`等，允许用户修改
  - **认证信息**: 
    - 当`auth_type`为`api_key`时，显示"API Key"标签和相应提示
    - 当`auth_type`为`auth_token`时，显示"API Token"标签和相应提示
  - **URL**: 从预设配置自动填充，用户可修改（某些场景下可能需要自定义域名）
  - **默认模型**: 根据预设的`require_default_model`决定是否显示
    - 使用可编辑下拉框（combobox）形式，既可以从预设选项中选择，也可以手动输入
    - 如果`default_model_options`不为空，预填充下拉选项
    - 用户始终可以手动输入自定义模型名称
  - **路径前缀**: 从预设配置自动填充，用户可修改

### 第三步：确认配置
- **标题**: "确认端点配置"
- **内容**: 显示最终的端点配置信息供用户确认
- **操作**: 用户确认后保存端点配置

## 技术实现方案

### 后端实现

#### 1. 配置文件处理
```go
// internal/config/profiles.go

//go:embed endpoint_profiles.yaml
var embeddedProfiles []byte

type EndpointProfile struct {
    ProfileID            string `yaml:"profile_id"`
    DisplayName          string `yaml:"display_name"`
    URL                  string `yaml:"url"`
    EndpointType         string `yaml:"endpoint_type"`
    AuthType             string `yaml:"auth_type"`
    PathPrefix           string `yaml:"path_prefix"`
    OverrideMaxTokens    *int   `yaml:"override_max_tokens"`
    RequireDefaultModel  bool   `yaml:"require_default_model"`
    DefaultModelOptions  string `yaml:"default_model_options"`
}

type ProfilesConfig struct {
    Profiles []EndpointProfile `yaml:"profiles"`
}

func LoadEmbeddedEndpointProfiles() (*ProfilesConfig, error) {
    var config ProfilesConfig
    err := yaml.Unmarshal(embeddedProfiles, &config)
    return &config, err
}

func (p *EndpointProfile) ToEndpointConfig(name, authValue, defaultModel, url string) config.EndpointConfig
```

#### 2. API接口
```go
// internal/web/endpoint_wizard.go

// GET /admin/api/endpoint-profiles
func (s *AdminServer) handleGetEndpointProfiles(c *gin.Context)

// POST /admin/api/endpoints/from-wizard
func (s *AdminServer) handleCreateEndpointFromWizard(c *gin.Context)
```

#### 3. 请求/响应格式
```go
// 获取预设列表响应
type GetProfilesResponse struct {
    Profiles []EndpointProfile `json:"profiles"`
}

// 从向导创建端点请求
type CreateFromWizardRequest struct {
    ProfileID      string `json:"profile_id" binding:"required"`
    Name          string `json:"name" binding:"required"`
    AuthValue     string `json:"auth_value" binding:"required"`
    URL          string `json:"url" binding:"required"`      // 从预设填充，用户可修改
    DefaultModel  string `json:"default_model,omitempty"`
}

// 端点名称生成和验证
func GenerateUniqueEndpointName(displayName string, existingNames []string) string
func ValidateAndGenerateEndpointName(baseName string, existingNames []string) string
```

### 前端实现

#### 1. 向导模态框
```html
<!-- web/templates/endpoint-wizard-modal.html -->
<div class="modal fade" id="endpointWizardModal" tabindex="-1">
    <div class="modal-dialog modal-lg">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title" id="wizardModalTitle">添加端点向导</h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div class="modal-body">
                <!-- 步骤指示器 -->
                <div class="wizard-steps">
                    <div class="step active" data-step="1">选择类型</div>
                    <div class="step" data-step="2">配置信息</div>
                    <div class="step" data-step="3">确认保存</div>
                </div>
                
                <!-- 步骤内容 -->
                <div id="wizard-step-1" class="wizard-step active">
                    <!-- 预设选择界面 -->
                    <div class="mb-4">
                        <label for="profile-select" class="form-label">选择端点类型</label>
                        <select class="form-select" id="profile-select" required>
                            <option value="">请选择端点类型...</option>
                            <!-- 动态填充预设选项 -->
                        </select>
                    </div>
                </div>
                <div id="wizard-step-2" class="wizard-step">
                    <!-- 配置填写界面 -->
                    <form id="wizard-config-form">
                        <div class="mb-3">
                            <label for="endpoint-name-wizard" class="form-label">端点名称 <span class="text-danger">*</span></label>
                            <input type="text" class="form-control" id="endpoint-name-wizard" required>
                            <small class="form-text text-muted">根据预设自动生成，可修改</small>
                        </div>
                        <div class="mb-3">
                            <label for="auth-value-wizard" class="form-label" id="auth-label-wizard">认证信息 <span class="text-danger">*</span></label>
                            <div class="input-group">
                                <input type="password" class="form-control" id="auth-value-wizard" required>
                                <button class="btn btn-outline-secondary" type="button" id="toggle-auth-wizard">
                                    <i class="fas fa-eye"></i>
                                </button>
                            </div>
                            <small class="form-text text-muted" id="auth-help-wizard">请输入您的认证信息</small>
                        </div>
                        <div class="mb-3">
                            <label for="url-wizard" class="form-label">API地址 <span class="text-danger">*</span></label>
                            <input type="url" class="form-control" id="url-wizard" required>
                            <small class="form-text text-muted">可根据需要修改为自定义域名</small>
                        </div>
                        <!-- 默认模型字段，根据预设动态显示 -->
                        <div class="mb-3 d-none" id="default-model-group-wizard">
                            <label for="default-model-wizard" class="form-label">默认模型 <span class="text-danger">*</span></label>
                            <div class="position-relative">
                                <input type="text" class="form-control" id="default-model-input-wizard" 
                                       placeholder="请输入或选择模型名称" list="model-options">
                                <datalist id="model-options">
                                    <!-- 动态填充预设模型选项 -->
                                </datalist>
                            </div>
                            <small class="form-text text-muted">可以选择预设选项或手动输入模型名称</small>
                        </div>
                    </form>
                </div>
                <div id="wizard-step-3" class="wizard-step">
                    <!-- 确认界面 -->
                </div>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" id="wizard-prev-btn">上一步</button>
                <button type="button" class="btn btn-primary" id="wizard-next-btn">下一步</button>
                <button type="button" class="btn btn-success d-none" id="wizard-save-btn">保存端点</button>
            </div>
        </div>
    </div>
</div>
```

#### 2. JavaScript实现
```javascript
// web/static/endpoint-wizard.js

class EndpointWizard {
    constructor() {
        this.currentStep = 1;
        this.totalSteps = 3;
        this.selectedProfile = null;
        this.profiles = [];
        this.existingEndpoints = [];
    }

    async init() {
        await this.loadProfiles();
        await this.loadExistingEndpoints();
        this.bindEvents();
        this.renderStep1();
    }

    async loadProfiles() {
        const response = await fetch('/admin/api/endpoint-profiles');
        const data = await response.json();
        this.profiles = data.profiles;
    }

    async loadExistingEndpoints() {
        const response = await fetch('/admin/api/endpoints');
        const data = await response.json();
        this.existingEndpoints = data.endpoints;
    }

    renderStep1() {
        // 填充预设下拉选择框
        const select = document.getElementById('profile-select');
        select.innerHTML = '<option value="">请选择端点类型...</option>';
        
        this.profiles.forEach(profile => {
            const option = document.createElement('option');
            option.value = profile.profile_id;
            option.textContent = profile.display_name;
            select.appendChild(option);
        });
    }

    renderStep2() {
        if (!this.selectedProfile) return;

        // 自动生成端点名称（基于display_name）
        const generatedName = this.generateUniqueEndpointName(this.selectedProfile.display_name);
        document.getElementById('endpoint-name-wizard').value = generatedName;

        // 设置认证信息标签和提示
        const authLabel = document.getElementById('auth-label-wizard');
        const authHelp = document.getElementById('auth-help-wizard');
        
        if (this.selectedProfile.auth_type === 'api_key') {
            authLabel.innerHTML = 'API Key <span class="text-danger">*</span>';
            authHelp.textContent = '请输入您的 API Key（如：sk-ant-api03-...）';
        } else {
            authLabel.innerHTML = 'API Token <span class="text-danger">*</span>';
            authHelp.textContent = '请输入您的 API Token（如：sk-...）';
        }

        // 自动填充URL
        document.getElementById('url-wizard').value = this.selectedProfile.url;

        // 处理默认模型字段
        this.handleDefaultModelField();
    }

    generateUniqueEndpointName(displayName) {
        const existingNames = this.existingEndpoints.map(ep => ep.name);
        // 将显示名称转换为合适的端点名称（移除特殊字符，转小写，空格转横线）
        let baseName = displayName.toLowerCase()
            .replace(/[^\w\s-]/g, '') // 移除特殊字符
            .replace(/\s+/g, '-')     // 空格转横线
            .replace(/-+/g, '-');     // 多个横线合并为一个
        
        let uniqueName = baseName;
        let counter = 1;
        
        while (existingNames.includes(uniqueName)) {
            uniqueName = `${baseName}(${counter})`;
            counter++;
        }
        
        return uniqueName;
    }

    handleDefaultModelField() {
        const modelGroup = document.getElementById('default-model-group-wizard');
        const modelInput = document.getElementById('default-model-input-wizard');
        const datalist = document.getElementById('model-options');

        if (!this.selectedProfile.require_default_model) {
            modelGroup.classList.add('d-none');
            return;
        }

        modelGroup.classList.remove('d-none');

        // 清空datalist
        datalist.innerHTML = '';

        // 如果有预设选项，填充到datalist中
        if (this.selectedProfile.default_model_options) {
            const options = this.selectedProfile.default_model_options.split(',');
            options.forEach(option => {
                const optionElement = document.createElement('option');
                optionElement.value = option.trim();
                datalist.appendChild(optionElement);
            });
        }

        // 清空输入框
        modelInput.value = '';
    }

    renderStep3() {
        // 渲染确认界面
    }

    async saveEndpoint() {
        // 提交端点配置
    }
}
```

#### 3. UI集成
在 `endpoints.html` 中添加向导按钮：
```html
<div class="card-header d-flex justify-content-between align-items-center">
    <div>
        <h5 class="mb-1" data-t="endpoint_configuration">端点配置</h5>
        <small class="text-muted" data-t="config_changes_auto_save">配置变更自动保存，实时生效</small>
    </div>
    <div class="btn-group">
        <button class="btn btn-outline-primary" data-action="show-endpoint-wizard">
            <i class="fas fa-magic"></i> <span data-t="wizard_add">向导添加</span>
        </button>
        <button class="btn btn-primary" data-action="show-add-endpoint-modal">
            <i class="fas fa-plus"></i> <span data-t="add_endpoint">添加端点</span>
        </button>
    </div>
</div>
```

## 用户交互流程

### 正常流程
1. 用户点击"向导添加"按钮
2. 显示预设选择界面，用户选择一个预设
3. 显示配置表单，用户填写必要信息：
   - 端点名称
   - API认证信息
   - 默认模型（如果需要）
   - 其他必填信息
4. 显示确认界面，用户检查配置
5. 用户点击"保存"，创建端点

### 特殊处理
- **返回上一步**: 用户可以在任何步骤点击"上一步"返回修改
- **表单验证**: 在步骤2进行实时表单验证
- **错误处理**: 保存失败时显示错误信息，允许用户重试

## 与现有功能的关系

### 共存策略
- 保留原有的"添加端点"按钮，作为高级配置选项
- 新增"向导添加"按钮，作为快速配置选项
- 两个功能使用相同的后端API创建端点

### 功能对比
| 功能 | 向导添加 | 高级添加 |
|------|---------|---------|
| 适用场景 | 常用预设端点 | 自定义配置 |
| 配置复杂度 | 简化 | 完整 |
| 用户门槛 | 低 | 高 |
| 配置选项 | 有限 | 完整 |

## 配置文件维护

### 嵌入式配置管理
预设配置文件使用Go的`embed`功能直接编译到可执行文件中，确保版本一致性和部署简化：

1. 预设配置由开发团队统一维护
2. 配置文件位于项目根目录的`endpoint_profiles.yaml`
3. 使用`//go:embed`指令将配置文件嵌入到可执行文件中
4. 用户无法修改预设配置，避免配置错误和版本不一致问题

### 开发维护流程
1. 开发者在`endpoint_profiles.yaml`中添加或修改预设配置
2. 配置包含9个字段：profile_id, display_name, url, endpoint_type, auth_type, path_prefix, override_max_tokens, require_default_model, default_model_options
3. **重要约束**：
   - `url` 字段必须填写，不能为空
   - `path_prefix` 字段必须填写，Anthropic端点可填空字符串，OpenAI端点必须填写路径
   - `profile_id` 必须唯一，用于程序引用
   - `display_name` 用于生成默认端点名称和用户界面显示
   - `default_model_options` 为空时表示无预设选项，用户完全手动输入
4. 重新编译项目后配置生效

### 验证和测试
- 配置文件修改后需要重新编译
- 建议在测试环境先验证新的预设配置
- 可以添加配置文件语法检查到CI/CD流程中

## 未来扩展

### 可能的增强功能
**推理测试**: 在向导中添加"测试连接"功能，向配置的端点发送一个简单的推理请求来验证配置是否正确