package web

import (
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
		
		successRate := calculateSuccessRate(ep.SuccessRequests, ep.TotalRequests)
		
		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}
	
	overallSuccessRate := calculateSuccessRate(successRequests, totalRequests)
	
	data := s.mergeTemplateData(c, "dashboard", map[string]interface{}{
		"Title":             "Claude Proxy Dashboard",
		"TotalEndpoints":    len(endpoints),
		"ActiveEndpoints":   activeEndpoints,
		"TotalRequests":     totalRequests,
		"SuccessRequests":   successRequests,
		"OverallSuccessRate": overallSuccessRate,
		"Endpoints":         endpointStats,
	})
	s.renderHTML(c, "dashboard.html", data)
}

func (s *AdminServer) handleEndpointsPage(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	
	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}
	
	endpointStats := make([]EndpointStats, 0)
	
	for _, ep := range endpoints {
		successRate := calculateSuccessRate(ep.SuccessRequests, ep.TotalRequests)
		
		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}
	
	data := s.mergeTemplateData(c, "endpoints", map[string]interface{}{
		"Title":     "Endpoints Configuration",
		"Endpoints": endpointStats,
	})
	s.renderHTML(c, "endpoints.html", data)
}