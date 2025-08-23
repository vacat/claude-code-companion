package logger

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
	"claude-code-companion/internal/i18n"
)

// ValidateGORMCompatibility 验证GORM与modernc.org/sqlite的兼容性
func ValidateGORMCompatibility() error {
	// 首先使用标准sql包测试modernc.org/sqlite连接
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf(i18n.T("sqlite_connection_failed", "modernc.org/sqlite连接失败: %v"), err)
	}
	defer sqlDB.Close()
	
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf(i18n.T("sqlite_ping_failed", "modernc.org/sqlite ping失败: %v"), err)
	}
	
	// 使用已有连接创建GORM实例
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        ":memory:",
	}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf(i18n.T("gorm_connection_failed", "GORM连接失败: %v"), err)
	}
	
	// 验证底层驱动
	gormSqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf(i18n.T("get_underlying_db_failed", "获取底层数据库失败: %v"), err)
	}
	defer gormSqlDB.Close()
	
	// 测试基础SQL操作
	if err := gormSqlDB.Ping(); err != nil {
		return fmt.Errorf(i18n.T("db_connection_test_failed", "数据库连接测试失败: %v"), err)
	}
	
	// 测试自动迁移
	if err := db.AutoMigrate(&GormRequestLog{}); err != nil {
		return fmt.Errorf(i18n.T("auto_migration_failed", "自动迁移失败: %v"), err)
	}
	
	// 测试基础CRUD操作
	testLog := &GormRequestLog{
		RequestID: "test-compatibility-123",
		Endpoint:  "test-endpoint",
		Method:    "GET",
		Path:      "/test",
	}
	
	// 测试创建
	if err := db.Create(testLog).Error; err != nil {
		return fmt.Errorf(i18n.T("create_test_record_failed", "创建测试记录失败: %v"), err)
	}
	
	// 测试查询
	var foundLog GormRequestLog
	if err := db.Where("request_id = ?", "test-compatibility-123").First(&foundLog).Error; err != nil {
		return fmt.Errorf(i18n.T("query_test_record_failed", "查询测试记录失败: %v"), err)
	}
	
	// 测试删除
	if err := db.Delete(&foundLog).Error; err != nil {
		return fmt.Errorf(i18n.T("delete_test_record_failed", "删除测试记录失败: %v"), err)
	}
	
	fmt.Println(i18n.T("gorm_compatibility_validation_passed", "✅ GORM与modernc.org/sqlite兼容性验证通过"))
	return nil
}