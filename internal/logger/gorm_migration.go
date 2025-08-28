package logger

import (
	"fmt"
	"strings"
	"gorm.io/gorm"
)

// createOptimizedIndexes 创建基于现有查询模式的优化索引
func createOptimizedIndexes(db *gorm.DB) error {
	// 基于现有查询模式分析的索引优化策略
	// 这些是对现有索引的补充优化，不会破坏现有结构
	indexes := []string{
		// 复合索引优化（基于 GetLogs 方法的查询模式）
		"CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp_status_opt ON request_logs(timestamp DESC, status_code)",
		
		// 支持分页查询的覆盖索引
		"CREATE INDEX IF NOT EXISTS idx_request_logs_pagination_opt ON request_logs(timestamp DESC, id)",
		
		// 端点特定查询优化
		"CREATE INDEX IF NOT EXISTS idx_request_logs_endpoint_time_opt ON request_logs(endpoint, timestamp DESC)",
		
		// 请求ID查询优化（GetAllLogsByRequestID方法）
		"CREATE INDEX IF NOT EXISTS idx_request_logs_request_id_time ON request_logs(request_id, timestamp ASC)",
		
		// 基于模型的查询优化
		"CREATE INDEX IF NOT EXISTS idx_request_logs_model_time ON request_logs(model, timestamp DESC)",
		
		// 失败日志查询优化
		"CREATE INDEX IF NOT EXISTS idx_request_logs_status_code_time ON request_logs(status_code, timestamp DESC)",
		
		// 错误字段索引
		"CREATE INDEX IF NOT EXISTS idx_request_logs_error_time ON request_logs(timestamp DESC) WHERE error != ''",
	}
	
	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			// 忽略已存在的索引错误，但记录其他错误
			if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate") {
				return fmt.Errorf("failed to create index: %v", err)
			}
		}
	}
	return nil
}

// validateTableCompatibility 验证现有表结构兼容性并自动添加缺失的列
func validateTableCompatibility(db *gorm.DB) error {
	// 检查表是否存在
	if !db.Migrator().HasTable(&GormRequestLog{}) {
		return fmt.Errorf("request_logs table does not exist")
	}
	
	// 检查关键字段是否存在
	requiredColumns := []string{
		"timestamp", "request_id", "endpoint", "method", "path",
		"status_code", "duration_ms", "request_headers", "response_headers",
		"request_body", "response_body", "thinking_enabled",
		"original_model", "rewritten_model", "model_rewrite_applied",
		"attempt_number", "thinking_budget_tokens",
	}
	
	for _, column := range requiredColumns {
		if !db.Migrator().HasColumn(&GormRequestLog{}, column) {
			return fmt.Errorf("required column %s does not exist", column)
		}
	}
	
	// 检查并添加新增的可选列
	optionalColumns := map[string]string{
		"session_id": "session_id VARCHAR(100) DEFAULT ''",
		"blacklist_causing_request_ids": "blacklist_causing_request_ids TEXT DEFAULT '[]'",
		"endpoint_blacklisted_at": "endpoint_blacklisted_at DATETIME",
		"endpoint_blacklist_reason": "endpoint_blacklist_reason TEXT DEFAULT ''",
	}
	
	for column, definition := range optionalColumns {
		if !db.Migrator().HasColumn(&GormRequestLog{}, column) {
			// 添加缺失的列
			sql := fmt.Sprintf("ALTER TABLE request_logs ADD COLUMN %s", definition)
			if err := db.Exec(sql).Error; err != nil {
				return fmt.Errorf("failed to add column %s: %v", column, err)
			}
			fmt.Printf("Added column %s to request_logs table\n", column)
		}
	}
	
	return nil
}