package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/incident-sage/backend/internal/database"
	"github.com/incident-sage/backend/internal/models"
)

type CreateIncidentRequest struct {
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description" binding:"required"`
	Priority    string   `json:"priority" binding:"required"`
	Severity    string   `json:"severity" binding:"required"`
	AssigneeID  *uint    `json:"assigneeId"`
	Tags        []string `json:"tags"`
}

type UpdateIncidentRequest struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Status      *string   `json:"status"`
	Priority    *string   `json:"priority"`
	Severity    *string   `json:"severity"`
	AssigneeID  *uint     `json:"assigneeId"`
	Tags        *[]string `json:"tags"`
}

func GetIncidents(c *gin.Context) {
	var incidents []models.Incident

	query := database.DB.Preload("Reporter").Preload("Assignee")

	// Add filters
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if priority := c.Query("priority"); priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if assigneeID := c.Query("assigneeId"); assigneeID != "" {
		query = query.Where("assignee_id = ?", assigneeID)
	}

	// Add limit
	limit := c.DefaultQuery("limit", "50")
	if limitInt, err := strconv.Atoi(limit); err == nil && limitInt > 0 {
		query = query.Limit(limitInt)
	}

	if err := query.Order("created_at desc").Find(&incidents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch incidents",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    incidents,
	})
}

func GetIncident(c *gin.Context) {
	id := c.Param("id")

	var incident models.Incident
	if err := database.DB.Preload("Reporter").Preload("Assignee").First(&incident, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Incident not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    incident,
	})
}

func CreateIncident(c *gin.Context) {
	var req CreateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
			"errors":  err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")

	// Parse priority and severity
	var priority models.IncidentPriority
	var severity models.IncidentSeverity

	switch req.Priority {
	case "LOW":
		priority = models.PriorityLow
	case "MEDIUM":
		priority = models.PriorityMedium
	case "HIGH":
		priority = models.PriorityHigh
	case "CRITICAL":
		priority = models.PriorityCritical
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid priority value",
		})
		return
	}

	switch req.Severity {
	case "MINOR":
		severity = models.SeverityMinor
	case "MODERATE":
		severity = models.SeverityModerate
	case "MAJOR":
		severity = models.SeverityMajor
	case "CRITICAL":
		severity = models.SeverityCritical
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid severity value",
		})
		return
	}

	incident := models.Incident{
		Title:       req.Title,
		Description: req.Description,
		Status:      models.StatusOpen,
		Priority:    priority,
		Severity:    severity,
		ReporterID:  userID.(uint),
		AssigneeID:  req.AssigneeID,
		Tags:        req.Tags,
	}

	if err := database.DB.Create(&incident).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create incident",
		})
		return
	}

	// Load relationships
	database.DB.Preload("Reporter").Preload("Assignee").First(&incident, incident.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    incident,
	})
}

func UpdateIncident(c *gin.Context) {
	id := c.Param("id")

	var incident models.Incident
	if err := database.DB.First(&incident, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Incident not found",
		})
		return
	}

	var req UpdateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// Update fields
	if req.Title != nil {
		incident.Title = *req.Title
	}
	if req.Description != nil {
		incident.Description = *req.Description
	}
	if req.Status != nil {
		switch *req.Status {
		case "OPEN":
			incident.Status = models.StatusOpen
		case "IN_PROGRESS":
			incident.Status = models.StatusInProgress
		case "RESOLVED":
			incident.Status = models.StatusResolved
		case "CLOSED":
			incident.Status = models.StatusClosed
		case "CANCELLED":
			incident.Status = models.StatusCancelled
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid status value",
			})
			return
		}
	}
	if req.Priority != nil {
		switch *req.Priority {
		case "LOW":
			incident.Priority = models.PriorityLow
		case "MEDIUM":
			incident.Priority = models.PriorityMedium
		case "HIGH":
			incident.Priority = models.PriorityHigh
		case "CRITICAL":
			incident.Priority = models.PriorityCritical
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid priority value",
			})
			return
		}
	}
	if req.Severity != nil {
		switch *req.Severity {
		case "MINOR":
			incident.Severity = models.SeverityMinor
		case "MODERATE":
			incident.Severity = models.SeverityModerate
		case "MAJOR":
			incident.Severity = models.SeverityMajor
		case "CRITICAL":
			incident.Severity = models.SeverityCritical
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid severity value",
			})
			return
		}
	}
	if req.AssigneeID != nil {
		incident.AssigneeID = req.AssigneeID
	}
	if req.Tags != nil {
		incident.Tags = *req.Tags
	}

	if err := database.DB.Save(&incident).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update incident",
		})
		return
	}

	// Load relationships
	database.DB.Preload("Reporter").Preload("Assignee").First(&incident, incident.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    incident,
	})
}

func DeleteIncident(c *gin.Context) {
	id := c.Param("id")

	var incident models.Incident
	if err := database.DB.First(&incident, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Incident not found",
		})
		return
	}

	if err := database.DB.Delete(&incident).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete incident",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Incident deleted successfully",
	})
}

func GetIncidentUpdates(c *gin.Context) {
	incidentID := c.Param("id")

	var updates []models.IncidentUpdate
	if err := database.DB.Preload("User").Where("incident_id = ?", incidentID).Order("created_at desc").Find(&updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch incident updates",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updates,
	})
}

func CreateIncidentUpdate(c *gin.Context) {
	incidentID := c.Param("id")
	userID, _ := c.Get("user_id")

	var req struct {
		Content string `json:"content" binding:"required"`
		Type    string `json:"type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var updateType models.UpdateType
	switch req.Type {
	case "COMMENT":
		updateType = models.UpdateTypeComment
	case "STATUS_CHANGE":
		updateType = models.UpdateTypeStatusChange
	case "ASSIGNMENT":
		updateType = models.UpdateTypeAssignment
	case "PRIORITY_CHANGE":
		updateType = models.UpdateTypePriorityChange
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid update type",
		})
		return
	}

	update := models.IncidentUpdate{
		IncidentID: uint(incidentID),
		UserID:     userID.(uint),
		Content:    req.Content,
		Type:       updateType,
	}

	if err := database.DB.Create(&update).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create incident update",
		})
		return
	}

	// Load user relationship
	database.DB.Preload("User").First(&update, update.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    update,
	})
}
