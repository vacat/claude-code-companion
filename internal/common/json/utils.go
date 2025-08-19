package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// SafeMarshal 安全的JSON序列化，提供更好的错误信息
func SafeMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %T: %w", v, err)
	}
	return data, nil
}

// SafeUnmarshal 安全的JSON反序列化，提供更好的错误信息
func SafeUnmarshal(data []byte, v interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("cannot unmarshal empty data into %T", v)
	}
	
	err := json.Unmarshal(data, v)
	if err != nil {
		return fmt.Errorf("failed to unmarshal into %T: %w", v, err)
	}
	return nil
}

// SafeUnmarshalWithDefault 安全的JSON反序列化，失败时使用默认值
func SafeUnmarshalWithDefault[T any](data []byte, defaultValue T) T {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return defaultValue
	}
	return result
}

// ExtractField 提取JSON中的指定字段（泛型版本）
func ExtractField[T any](data []byte, field string) (T, error) {
	var zero T
	if len(data) == 0 {
		return zero, nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return zero, fmt.Errorf("failed to parse JSON: %w", err)
	}

	value, exists := parsed[field]
	if !exists {
		return zero, nil
	}

	// 类型转换
	if converted, ok := value.(T); ok {
		return converted, nil
	}

	// 尝试通过JSON重新序列化和反序列化进行类型转换
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal field value: %w", err)
	}

	var result T
	if err := json.Unmarshal(valueBytes, &result); err != nil {
		return zero, fmt.Errorf("failed to convert field %s to %T: %w", field, result, err)
	}

	return result, nil
}

// ExtractNestedField 提取嵌套字段（泛型版本）
func ExtractNestedField[T any](data []byte, path []string) (T, error) {
	var zero T
	if len(data) == 0 || len(path) == 0 {
		return zero, nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return zero, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 导航到嵌套路径
	current := parsed
	for _, key := range path[:len(path)-1] {
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return zero, nil
		}
	}

	// 获取最终字段
	finalKey := path[len(path)-1]
	value, exists := current[finalKey]
	if !exists {
		return zero, nil
	}

	// 类型转换
	if converted, ok := value.(T); ok {
		return converted, nil
	}

	// 尝试通过JSON重新序列化进行类型转换
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal nested field value: %w", err)
	}

	var result T
	if err := json.Unmarshal(valueBytes, &result); err != nil {
		return zero, fmt.Errorf("failed to convert nested field %s to %T: %w", 
			fmt.Sprintf("%v.%s", path[:len(path)-1], finalKey), result, err)
	}

	return result, nil
}

// ValidateJSON 验证JSON字符串是否有效
func ValidateJSON(data []byte) error {
	var temp interface{}
	return json.Unmarshal(data, &temp)
}

// IsValidJSON 检查数据是否为有效JSON
func IsValidJSON(data []byte) bool {
	return ValidateJSON(data) == nil
}

// PrettyPrint 美化打印JSON
func PrettyPrint(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to pretty print %T: %w", v, err)
	}
	return string(data), nil
}

// CompactJSON 压缩JSON字符串
func CompactJSON(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, data); err != nil {
		return nil, fmt.Errorf("failed to compact JSON: %w", err)
	}
	return buffer.Bytes(), nil
}

// MergeJSONObjects 合并两个JSON对象
func MergeJSONObjects(obj1, obj2 []byte) ([]byte, error) {
	var map1, map2 map[string]interface{}
	
	if err := json.Unmarshal(obj1, &map1); err != nil {
		return nil, fmt.Errorf("failed to unmarshal first object: %w", err)
	}
	
	if err := json.Unmarshal(obj2, &map2); err != nil {
		return nil, fmt.Errorf("failed to unmarshal second object: %w", err)
	}
	
	// 合并map2到map1
	for k, v := range map2 {
		map1[k] = v
	}
	
	return json.Marshal(map1)
}

// CloneViaJSON 通过JSON序列化/反序列化进行深度克隆
func CloneViaJSON[T any](src T) (T, error) {
	var dst T
	data, err := json.Marshal(src)
	if err != nil {
		return dst, fmt.Errorf("failed to marshal source: %w", err)
	}
	
	err = json.Unmarshal(data, &dst)
	if err != nil {
		return dst, fmt.Errorf("failed to unmarshal to destination: %w", err)
	}
	
	return dst, nil
}

// GetJSONType 获取JSON值的类型
func GetJSONType(data []byte) (string, error) {
	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	
	switch temp.(type) {
	case map[string]interface{}:
		return "object", nil
	case []interface{}:
		return "array", nil
	case string:
		return "string", nil
	case float64:
		return "number", nil
	case bool:
		return "boolean", nil
	case nil:
		return "null", nil
	default:
		return reflect.TypeOf(temp).String(), nil
	}
}