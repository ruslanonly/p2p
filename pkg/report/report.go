package report

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func Report(report AgentReport) {
	reportURL := os.Getenv("REPORT_URL")
	if reportURL == "" {
		log.Fatalf("Параметр REPORT_URL не указан")
		return
	}

	data, err := json.Marshal(report)
	if err != nil {
		log.Printf("Failed to marshal report: %v", err)
		return
	}

	resp, err := http.Post(reportURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to send report: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Server returned non-OK status: %s", resp.Status)
	}
}
