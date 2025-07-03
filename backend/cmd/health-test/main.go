package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	Services  struct {
		Database struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		} `json:"database"`
	} `json:"services"`
}

func main() {
	url := "http://localhost:8080/health"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	fmt.Printf("🔍 Testing health endpoint: %s\n", url)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("❌ Error connecting to health endpoint: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Error reading response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📊 Response Status: %s\n", resp.Status)
	fmt.Printf("📄 Response Body: %s\n", string(body))

	if resp.StatusCode != 200 {
		fmt.Printf("❌ Health check failed with status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var health HealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		fmt.Printf("❌ Error parsing JSON response: %v\n", err)
		os.Exit(1)
	}

	if health.Status != "ok" {
		fmt.Printf("❌ Health status is not 'ok': %s\n", health.Status)
		os.Exit(1)
	}

	if health.Services.Database.Status != "ok" {
		fmt.Printf("❌ Database status is not 'ok': %s\n", health.Services.Database.Status)
		if health.Services.Database.Error != "" {
			fmt.Printf("   Database error: %s\n", health.Services.Database.Error)
		}
		os.Exit(1)
	}

	fmt.Printf("✅ Health check passed!\n")
	fmt.Printf("   Status: %s\n", health.Status)
	fmt.Printf("   Version: %s\n", health.Version)
	fmt.Printf("   Database: %s\n", health.Services.Database.Status)
	fmt.Printf("   Timestamp: %s\n", health.Timestamp)
}
