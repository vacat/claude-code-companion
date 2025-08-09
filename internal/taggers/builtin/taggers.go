package builtin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"claude-proxy/internal/interfaces"
)

// wildcardMatch 统一的通配符匹配函数，支持更直观的通配符语义
// * 匹配任意字符序列
// ? 匹配单个字符
func wildcardMatch(pattern, str string) (bool, error) {
	// 将通配符模式转换为正则表达式
	regexPattern := wildcardToRegex(pattern)
	
	// 编译正则表达式
	regex, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		return false, fmt.Errorf("invalid pattern '%s': %v", pattern, err)
	}
	
	return regex.MatchString(str), nil
}

// wildcardToRegex 将通配符模式转换为正则表达式
func wildcardToRegex(pattern string) string {
	// 转义正则表达式特殊字符，但保留我们的通配符
	escaped := regexp.QuoteMeta(pattern)
	
	// 将转义后的通配符还原并转换为正则表达式
	// \* -> .* (匹配任意字符序列)
	// \? -> . (匹配单个字符)
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	
	return escaped
}

// BaseTagger 内置tagger的基础结构
type BaseTagger struct {
	name string
	tag  string
}

func (bt *BaseTagger) Name() string { return bt.name }
func (bt *BaseTagger) Tag() string  { return bt.tag }

// PathTagger 路径匹配tagger
type PathTagger struct {
	BaseTagger
	pathPattern string
}

// NewPathTagger 创建路径匹配tagger
func NewPathTagger(name, tag string, config map[string]interface{}) (interfaces.Tagger, error) {
	pathPattern, ok := config["path_pattern"].(string)
	if !ok || pathPattern == "" {
		return nil, fmt.Errorf("path tagger requires 'path_pattern' in config")
	}

	return &PathTagger{
		BaseTagger:  BaseTagger{name: name, tag: tag},
		pathPattern: pathPattern,
	}, nil
}

func (pt *PathTagger) ShouldTag(request *http.Request) (bool, error) {
	// 使用统一的通配符匹配函数
	return wildcardMatch(pt.pathPattern, request.URL.Path)
}

// HeaderTagger 请求头匹配tagger
type HeaderTagger struct {
	BaseTagger
	headerName    string
	expectedValue string
}

// NewHeaderTagger 创建请求头匹配tagger
func NewHeaderTagger(name, tag string, config map[string]interface{}) (interfaces.Tagger, error) {
	headerName, ok := config["header_name"].(string)
	if !ok || headerName == "" {
		return nil, fmt.Errorf("header tagger requires 'header_name' in config")
	}

	expectedValue, ok := config["expected_value"].(string)
	if !ok || expectedValue == "" {
		return nil, fmt.Errorf("header tagger requires 'expected_value' in config")
	}

	return &HeaderTagger{
		BaseTagger:    BaseTagger{name: name, tag: tag},
		headerName:    headerName,
		expectedValue: expectedValue,
	}, nil
}

func (ht *HeaderTagger) ShouldTag(request *http.Request) (bool, error) {
	headerValue := request.Header.Get(ht.headerName)
	if headerValue == "" {
		return false, nil
	}

	// 使用统一的通配符匹配函数
	return wildcardMatch(ht.expectedValue, headerValue)
}

// MethodTagger HTTP方法匹配tagger
type MethodTagger struct {
	BaseTagger
	methods []string
}

// NewMethodTagger 创建HTTP方法匹配tagger
func NewMethodTagger(name, tag string, config map[string]interface{}) (interfaces.Tagger, error) {
	methodsInterface, ok := config["methods"]
	if !ok {
		return nil, fmt.Errorf("method tagger requires 'methods' in config")
	}

	var methods []string
	switch v := methodsInterface.(type) {
	case []interface{}:
		methods = make([]string, len(v))
		for i, method := range v {
			if str, ok := method.(string); ok {
				methods[i] = strings.ToUpper(str)
			} else {
				return nil, fmt.Errorf("method tagger 'methods' must be array of strings")
			}
		}
	case []string:
		methods = make([]string, len(v))
		for i, method := range v {
			methods[i] = strings.ToUpper(method)
		}
	case string:
		methods = []string{strings.ToUpper(v)}
	default:
		return nil, fmt.Errorf("method tagger 'methods' must be string or array of strings")
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("method tagger requires at least one method")
	}

	return &MethodTagger{
		BaseTagger: BaseTagger{name: name, tag: tag},
		methods:    methods,
	}, nil
}

func (mt *MethodTagger) ShouldTag(request *http.Request) (bool, error) {
	requestMethod := strings.ToUpper(request.Method)
	for _, method := range mt.methods {
		if method == requestMethod {
			return true, nil
		}
	}
	return false, nil
}

// QueryTagger 查询参数匹配tagger
type QueryTagger struct {
	BaseTagger
	paramName     string
	expectedValue string
}

// NewQueryTagger 创建查询参数匹配tagger
func NewQueryTagger(name, tag string, config map[string]interface{}) (interfaces.Tagger, error) {
	paramName, ok := config["param_name"].(string)
	if !ok || paramName == "" {
		return nil, fmt.Errorf("query tagger requires 'param_name' in config")
	}

	expectedValue, ok := config["expected_value"].(string)
	if !ok || expectedValue == "" {
		return nil, fmt.Errorf("query tagger requires 'expected_value' in config")
	}

	return &QueryTagger{
		BaseTagger:    BaseTagger{name: name, tag: tag},
		paramName:     paramName,
		expectedValue: expectedValue,
	}, nil
}

func (qt *QueryTagger) ShouldTag(request *http.Request) (bool, error) {
	paramValue := request.URL.Query().Get(qt.paramName)
	if paramValue == "" {
		return false, nil
	}

	// 使用统一的通配符匹配函数
	return wildcardMatch(qt.expectedValue, paramValue)
}

// BodyJSONTagger JSON请求体字段匹配tagger
type BodyJSONTagger struct {
	BaseTagger
	jsonPath      string
	expectedValue string
}

// NewBodyJSONTagger 创建JSON请求体字段匹配tagger
func NewBodyJSONTagger(name, tag string, config map[string]interface{}) (interfaces.Tagger, error) {
	jsonPath, ok := config["json_path"].(string)
	if !ok || jsonPath == "" {
		return nil, fmt.Errorf("body-json tagger requires 'json_path' in config")
	}

	expectedValue, ok := config["expected_value"].(string)
	if !ok || expectedValue == "" {
		return nil, fmt.Errorf("body-json tagger requires 'expected_value' in config")
	}

	return &BodyJSONTagger{
		BaseTagger:    BaseTagger{name: name, tag: tag},
		jsonPath:      jsonPath,
		expectedValue: expectedValue,
	}, nil
}

func (bt *BodyJSONTagger) ShouldTag(request *http.Request) (bool, error) {
	// 只处理JSON内容类型
	contentType := request.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return false, nil
	}

	// 从请求上下文中获取预处理的请求体数据
	// 这需要在调用tagger之前由pipeline预处理并设置到context中
	bodyContent, ok := request.Context().Value("cached_body").([]byte)
	if !ok || len(bodyContent) == 0 {
		return false, nil
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(bodyContent, &jsonData); err != nil {
		return false, nil // JSON解析失败，不匹配
	}

	// 简单的JSON路径解析（支持如 "model" 或 "data.model" 格式）
	value, err := bt.extractJSONValue(jsonData, bt.jsonPath)
	if err != nil {
		return false, nil
	}

	if strValue, ok := value.(string); ok {
		// 使用统一的通配符匹配函数
		return wildcardMatch(bt.expectedValue, strValue)
	}

	return false, nil
}

// extractJSONValue 从JSON数据中提取指定路径的值
func (bt *BodyJSONTagger) extractJSONValue(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一个部分，返回值
			return current[part], nil
		}

		// 中间部分，继续深入
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, fmt.Errorf("invalid path: %s", path)
		}
	}

	return nil, fmt.Errorf("empty path")
}