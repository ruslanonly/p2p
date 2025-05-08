package threats

import (
	"fmt"
	"os"
)

func Pipename() string {
	agentID := os.Getenv("AGENT_ID")
	if agentID == "" {
		return "ipc-threats"

	}

	return fmt.Sprintf("ipc-threats-%s", agentID)
}
