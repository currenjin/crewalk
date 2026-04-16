package session

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Session struct {
	TicketID string
}

func Start(ticketID, worktreePath, claudeCmd string, claudeArgs []string) (*Session, error) {
	args := make([]string, 0, len(claudeArgs)+4)
	args = append(args, claudeArgs...)
	args = append(args, "--append-system-prompt",
		"The git worktree has already been created for this ticket's branch. "+
			"For the branch creation question, automatically choose 'skip'. "+
			"For the PR creation question at the end, automatically choose 'push-pr'.",
	)
	args = append(args, "-p", "/work "+ticketID)

	scriptPath := fmt.Sprintf("/tmp/crewalk_%s.sh", ticketID)
	if err := os.WriteFile(scriptPath, []byte(buildLaunchScript(worktreePath, claudeCmd, args)), 0755); err != nil {
		return nil, fmt.Errorf("write launch script: %w", err)
	}

	appleScript := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, scriptPath)

	if err := exec.Command("osascript", "-e", appleScript).Run(); err != nil {
		os.Remove(scriptPath)
		return nil, fmt.Errorf("open terminal: %w", err)
	}

	go func() {
		time.Sleep(30 * time.Second)
		os.Remove(scriptPath)
	}()

	return &Session{TicketID: ticketID}, nil
}

func buildLaunchScript(worktreePath, claudeCmd string, args []string) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/zsh\n")
	sb.WriteString(fmt.Sprintf("cd %q\n", worktreePath))
	sb.WriteString(claudeCmd)
	for _, arg := range args {
		sb.WriteByte(' ')
		sb.WriteString(fmt.Sprintf("%q", arg))
	}
	sb.WriteString("\n")
	return sb.String()
}
