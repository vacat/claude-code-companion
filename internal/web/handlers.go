package web

import (
	"fmt"
	"net/http"
	"strconv"

	"claude-proxy/internal/config"
	"claude-proxy/internal/endpoint"

	"github.com/gin-gonic/gin"
)

func (s *AdminServer) handleDashboard(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	
	totalRequests := 0
	successRequests := 0
	activeEndpoints := 0
	
	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}
	
	endpointStats := make([]EndpointStats, 0)
	
	for _, ep := range endpoints {
		totalRequests += ep.TotalRequests
		successRequests += ep.SuccessRequests
		if ep.Status == endpoint.StatusActive {
			activeEndpoints++
		}
		
		successRate := "N/A"
		if ep.TotalRequests > 0 {
			rate := float64(ep.SuccessRequests) / float64(ep.TotalRequests) * 100.0
			successRate = fmt.Sprintf("%.1f%%", rate)
		}
		
		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}
	
	overallSuccessRate := "N/A"
	if totalRequests > 0 {
		rate := float64(successRequests) / float64(totalRequests) * 100.0
		overallSuccessRate = fmt.Sprintf("%.1f%%", rate)
	}
	
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Title":             "Claude Proxy Dashboard",
		"TotalEndpoints":    len(endpoints),
		"ActiveEndpoints":   activeEndpoints,
		"TotalRequests":     totalRequests,
		"SuccessRequests":   successRequests,
		"OverallSuccessRate": overallSuccessRate,
		"Endpoints":         endpointStats,
	})
}

func (s *AdminServer) handleEndpointsPage(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	
	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}
	
	endpointStats := make([]EndpointStats, 0)
	
	for _, ep := range endpoints {
		successRate := "N/A"
		if ep.TotalRequests > 0 {
			rate := float64(ep.SuccessRequests) / float64(ep.TotalRequests) * 100.0
			successRate = fmt.Sprintf("%.1f%%", rate)
		}
		
		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}
	
	c.HTML(http.StatusOK, "endpoints.html", gin.H{
		"Title":     "Endpoints Configuration",
		"Endpoints": endpointStats,
	})
}

func (s *AdminServer) handleLogsPage(c *gin.Context) {
	logs, total, _ := s.logger.GetLogs(50, 0, false)
	c.HTML(http.StatusOK, "logs.html", gin.H{
		"Title":     "Request Logs",
		"Logs":      logs,
		"Total":     total,
	})
}

func (s *AdminServer) handleSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"Title":  "Settings",
		"Config": s.config,
	})
}

func (s *AdminServer) handleGetEndpoints(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	c.JSON(http.StatusOK, gin.H{
		"endpoints": endpoints,
	})
}

func (s *AdminServer) handleUpdateEndpoints(c *gin.Context) {
	var request struct {
		Endpoints []config.EndpointConfig `json:"endpoints"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	s.endpointManager.UpdateEndpoints(request.Endpoints)
	c.JSON(http.StatusOK, gin.H{"message": "Endpoints updated successfully"})
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