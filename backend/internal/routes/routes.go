package routes

import (
	"os"

	"github.com/autolog/backend/internal/controllers"
	"github.com/autolog/backend/internal/middleware"
	"github.com/autolog/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRoutes configures all application routes
func SetupRoutes(r *gin.Engine, db *gorm.DB, stopChan <-chan struct{}) {
	// Initialize services
	llmService := services.NewLLMService(
		os.Getenv("OLLAMA_URL"),
		os.Getenv("OLLAMA_MODEL"),
	)

	// Initialize services
	parsingRuleService := services.NewParsingRuleService(db)
	learningService := services.NewLearningService(db, llmService)

	// Initialize controllers
	authController := controllers.NewAuthController(db)
	userController := controllers.NewUserController(db)
	logController := controllers.NewLogController(db, llmService, stopChan)
	parsingRuleController := controllers.NewParsingRuleController(parsingRuleService)
	learningController := controllers.NewLearningController(db, learningService)

	// API routes
	api := r.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/login", authController.Login)
			auth.POST("/register", authController.Register)
			auth.POST("/refresh", authController.RefreshToken)
		}

		// Protected routes
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			// Users
			users := protected.Group("/users")
			{
				users.GET("/me", userController.GetCurrentUser)
				users.PUT("/me", userController.UpdateCurrentUser)
				users.GET("", userController.GetUsers)
			}

			// Logs
			logs := protected.Group("/logs")
			{
				logs.POST("/upload", logController.UploadLogFile)
				logs.GET("/:id", logController.GetLogFile)
				logs.POST("/:id/analyze", logController.AnalyzeLogFile)
				logs.GET("/:id/analyses", logController.GetLogAnalyses)
				logs.GET("/:id/error-analysis", logController.GetDetailedErrorAnalysis)
				logs.GET("/:id/rca-results", logController.GetRCAResults)
				logs.DELETE("/:id", logController.DeleteLogFile)
				logs.GET("", logController.GetLogFiles)
				r.GET("/logs/:logFileId/jobs", logController.GetAllRCAJobs)
			}

			// RCA Jobs
			jobs := protected.Group("/jobs")
			{
				jobs.GET("/:jobId/status", logController.GetRCAJobStatus)
			}

			// Admin routes
			admin := protected.Group("/admin")
			{
				admin.GET("/logs", logController.GetAdminLogs)
				admin.GET("/llm-api-calls", logController.GetLLMAPICalls)
				admin.DELETE("/llm-api-calls", logController.ClearLLMAPICalls)
				admin.GET("/log-files/:id", logController.GetLogFileDetails)
				admin.GET("/jobs/:id", logController.GetJobDetails)
			}

			// Log Analysis Memory Feedback
			analyses := protected.Group("/analyses")
			{
				analyses.POST(":id/feedback", logController.AddFeedback)
				analyses.GET(":id/feedback", logController.GetFeedbackForAnalysis)
				analyses.GET("export/all", logController.ExportAllFeedback)
			}

			// LLM Status endpoint
			llm := protected.Group("/llm")
			{
				llm.GET("/status", logController.GetLLMStatus)
				llm.GET("/api-calls", logController.GetLLMAPICalls)
				llm.DELETE("/api-calls", logController.ClearLLMAPICalls)
			}

			// Parsing Rules
			parsingRules := protected.Group("/parsing-rules")
			{
				parsingRules.GET("", parsingRuleController.GetUserParsingRules)
				parsingRules.POST("", parsingRuleController.CreateParsingRule)
				parsingRules.GET("/:id", parsingRuleController.GetParsingRule)
				parsingRules.PUT("/:id", parsingRuleController.UpdateParsingRule)
				parsingRules.DELETE("/:id", parsingRuleController.DeleteParsingRule)
				parsingRules.POST("/test", parsingRuleController.TestParsingRule)
				parsingRules.GET("/active", parsingRuleController.GetActiveParsingRules)
			}

			// Learning and AI Insights
			learning := protected.Group("/learning")
			{
				learning.GET("/insights/:logFileID", learningController.GetLearningInsights)
				learning.GET("/patterns", learningController.GetPatterns)
				learning.GET("/patterns/:patternID", learningController.GetPattern)
				learning.DELETE("/patterns/:patternID", learningController.DeletePattern)
				learning.GET("/metrics", learningController.GetLearningMetrics)
				learning.GET("/similar-incidents/:logFileID", learningController.GetSimilarIncidents)
			}
		}
	}
}
