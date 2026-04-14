package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"

	"github.com/currenjin/crewalk/internal/ipc"
)

var ticketPattern = regexp.MustCompile(`(?i)([A-Z]+-\d+)`)

type hookInput struct {
	SessionID string          `json:"session_id"`
	Cwd       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	HookEvent string          `json:"hook_event_name"`
}

func main() {
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		os.Exit(0)
	}

	ticketID := extractTicketID(input.Cwd)
	if ticketID == "" {
		os.Exit(0)
	}

	phase, status := detectPhase(input.ToolName, input.ToolInput)
	if phase == "" {
		os.Exit(0)
	}

	event := ipc.Event{
		Type:     ipc.EventPhaseChange,
		TicketID: ticketID,
		Phase:    phase,
		Status:   status,
	}

	if err := sendEvent(event); err != nil {
		fmt.Fprintf(os.Stderr, "crewalk-bridge: %v\n", err)
	}

	os.Exit(0)
}

func extractTicketID(cwd string) string {
	base := filepath.Base(cwd)
	matches := ticketPattern.FindStringSubmatch(base)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func detectPhase(toolName string, toolInput json.RawMessage) (phase, status string) {
	if toolName == "Skill" {
		var input struct {
			Skill string `json:"skill"`
		}
		if err := json.Unmarshal(toolInput, &input); err == nil {
			switch input.Skill {
			case "jira-to-plan":
				return "PLANNING", "generating plan"
			case "start-branch":
				return "BRANCHING", "creating branch"
			case "augmented-coding":
				return "CODING", "implementing"
			case "review":
				return "REVIEWING", "reviewing"
			case "push-pr":
				return "PUSHING", "creating PR"
			}
		}
	}

	switch toolName {
	case "mcp__atlassian__read_jira_issue":
		return "PLANNING", "reading JIRA"
	}

	return "", ""
}

func sendEvent(event ipc.Event) error {
	conn, err := net.Dial("unix", ipc.SocketPath)
	if err != nil {
		return fmt.Errorf("crewalk not running: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = conn.Write(append(data, '\n'))
	return err
}
