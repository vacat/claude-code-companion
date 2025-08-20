package web

import (
	"fmt"
	"net/http"
	"strconv"

	"claude-code-companion/internal/config"
	"github.com/gin-gonic/gin"
)

func (s *AdminServer) handleLogsPage(c *gin.Context) {
	// 获取参数
	pageStr := c.DefaultQuery("page", strconv.Itoa(config.Default.Pagination.DefaultPage))
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < config.Default.Pagination.DefaultPage {
		page = config.Default.Pagination.DefaultPage
	}
	
	failedOnly, _ := strconv.ParseBool(failedOnlyStr)
	
	// 每页记录数使用统一默认值
	limit := config.Default.Pagination.DefaultLimit
	offset := (page - 1) * limit
	
	logs, total, _ := s.logger.GetLogs(limit, offset, failedOnly)
	
	// 计算分页信息
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = config.Default.Pagination.MaxPages
	}
	
	// 生成分页数组
	var pages []int
	startPage := page - 5
	if startPage < config.Default.Pagination.DefaultPage {
		startPage = config.Default.Pagination.DefaultPage
	}
	endPage := startPage + 9
	if endPage > totalPages {
		endPage = totalPages
		startPage = endPage - 9
		if startPage < config.Default.Pagination.DefaultPage {
			startPage = config.Default.Pagination.DefaultPage
		}
	}
	
	for i := startPage; i <= endPage; i++ {
		pages = append(pages, i)
	}
	
	data := s.mergeTemplateData(c, "logs", map[string]interface{}{
		"Title":       "Request Logs",
		"Logs":        logs,
		"Total":       total,
		"FailedOnly":  failedOnly,
		"Page":        page,
		"TotalPages":  totalPages,
		"Pages":       pages,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"PrevPage":    page - 1,
		"NextPage":    page + 1,
		"Limit":       limit,
	})
	s.renderHTML(c, "logs.html", data)
}

func (s *AdminServer) handleGetLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	requestIDStr := c.DefaultQuery("request_id", "")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	failedOnly, _ := strconv.ParseBool(failedOnlyStr)

	if requestIDStr != "" {
		// 如果指定了request_id，返回该请求的所有尝试记录
		allLogs, _ := s.logger.GetAllLogsByRequestID(requestIDStr)
		c.JSON(http.StatusOK, gin.H{
			"logs":  allLogs,
			"total": len(allLogs),
		})
		return
	}

	logs, total, err := s.logger.GetLogs(limit, offset, failedOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}

// handleCleanupLogs 清理日志
func (s *AdminServer) handleCleanupLogs(c *gin.Context) {
	var request struct {
		Days *int `json:"days" binding:"required,gte=0"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	days := *request.Days

	// 验证days参数 - 支持0表示清除全部，1, 7, 30表示清除指定天数之前的
	if days < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be >= 0 (0 means delete all logs)"})
		return
	}

	// 执行清理
	deletedCount, err := s.logger.CleanupLogsByDays(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup logs: " + err.Error()})
		return
	}

	message := fmt.Sprintf("Successfully cleaned up %d log entries", deletedCount)
	if days == 0 {
		message = fmt.Sprintf("Successfully deleted all %d log entries", deletedCount)
	} else {
		message = fmt.Sprintf("Successfully deleted %d log entries older than %d days", deletedCount, days)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       message,
		"deleted_count": deletedCount,
	})
}

// handleGetLogStats 获取日志统计信息
func (s *AdminServer) handleGetLogStats(c *gin.Context) {
	// SQLite存储提供基本统计信息
	stats := map[string]interface{}{
		"storage_type": "sqlite",
		"message": "SQLite storage active with automatic cleanup (30 days retention)",
		"features": []string{
			"Automatic cleanup of logs older than 30 days",
			"Indexed queries for better performance", 
			"Memory efficient storage",
			"ACID transactions",
		},
	}
	
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}