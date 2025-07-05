package main

import (
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	// Test with a sample log line
	testLine := `{"timestamp": "2025-07-04T10:00:00Z", "level": "INFO", "message": "Application 'FusionFlow' starting up...", "metadata": {"app_version": "2.1.0", "build_id": "b-7890"}}`

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(testLine), &parsed); err != nil {
		log.Fatal("JSON parse error:", err)
	}

	fmt.Printf("Parsed JSON: %+v\n", parsed)

	// Test field extraction
	timestamp := getString(parsed, "timestamp", "ts", "time", "date", "datetime", "@timestamp")
	level := getString(parsed, "level", "severity", "log_level", "lvl", "priority")
	message := getString(parsed, "message", "msg", "log", "log_message", "text", "body")

	fmt.Printf("Extracted fields:\n")
	fmt.Printf("  timestamp: %q\n", timestamp)
	fmt.Printf("  level: %q\n", level)
	fmt.Printf("  message: %q\n", message)

	// Test metadata extraction
	var metadata map[string]interface{}
	if meta, ok := parsed["metadata"].(map[string]interface{}); ok {
		metadata = meta
		fmt.Printf("  metadata: %+v\n", metadata)
	} else {
		fmt.Printf("  metadata: nil\n")
	}
}

func getString(parsed map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := parsed[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
