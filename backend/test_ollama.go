package main

import (
	"fmt"
	"log"
	"time"

	"github.com/autolog/backend/internal/models"
	"github.com/autolog/backend/internal/services"
)

func main() {
	fmt.Println("Testing Ollama connection...")

	// Create LLM service
	llmService := services.NewLLMService("http://localhost:11434", "codellama:7b")

	// Test health check
	fmt.Println("1. Testing health check...")
	if err := llmService.CheckLLMStatus(); err != nil {
		log.Printf("Health check failed: %v", err)
	} else {
		fmt.Println("✅ Health check passed")
	}

	// Test available models
	fmt.Println("2. Testing available models...")
	availableModels, err := llmService.GetAvailableModels()
	if err != nil {
		log.Printf("Failed to get models: %v", err)
	} else {
		fmt.Printf("✅ Available models: %v\n", availableModels)
	}

	// Test simple generation
	fmt.Println("3. Testing simple generation...")
	startTime := time.Now()

	// Create a simple test log file and entries for analysis
	testLogFile := models.LogFile{
		ID:         1,
		Filename:   "test.log",
		ErrorCount: 1,
	}

	testEntries := []models.LogEntry{
		{
			ID:        1,
			LogFileID: 1,
			Level:     "ERROR",
			Message:   "Test error message",
			Timestamp: time.Now(),
		},
	}

	// Test AI analysis
	response, err := llmService.AnalyzeLogsWithAI(&testLogFile, testEntries, nil) // No job ID for test
	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Printf("AI analysis failed: %v\n", err)
		return
	}

	if err != nil {
		log.Printf("Analysis failed after %v: %v", elapsed, err)
	} else {
		fmt.Printf("✅ Analysis successful in %v: %s\n", elapsed, response.Summary)
	}

	fmt.Println("Test completed!")
}
