package session

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const tmuxPrefix = "crewalk-"

type Session struct {
	TicketID    string
	tmuxSession string
}

func Start(ticketID, worktreePath, claudeCmd string, claudeArgs []string) (*Session, error) {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, fmt.Errorf("tmux가 필요합니다: brew install tmux")
	}

	tmuxSession := tmuxPrefix + ticketID

	exec.Command("tmux", "kill-session", "-t", tmuxSession).Run()

	if out, err := exec.Command("tmux", "new-session", "-d", "-s", tmuxSession, "-x", "220", "-y", "50").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tmux 세션 생성 실패: %w\n%s", err, out)
	}

	args := make([]string, 0, len(claudeArgs)+2)
	args = append(args, claudeArgs...)
	args = append(args, "--append-system-prompt",
		"The git worktree has already been created for this ticket's branch. "+
			"For the branch creation question, automatically choose 'skip'.",
	)

	scriptPath := fmt.Sprintf("/tmp/crewalk_%s.sh", ticketID)
	if err := os.WriteFile(scriptPath, []byte(buildLaunchScript(worktreePath, claudeCmd, args)), 0755); err != nil {
		exec.Command("tmux", "kill-session", "-t", tmuxSession).Run()
		return nil, fmt.Errorf("launch script 작성 실패: %w", err)
	}

	if out, err := exec.Command("tmux", "send-keys", "-t", tmuxSession, scriptPath, "Enter").CombinedOutput(); err != nil {
		exec.Command("tmux", "kill-session", "-t", tmuxSession).Run()
		os.Remove(scriptPath)
		return nil, fmt.Errorf("tmux 명령 전송 실패: %w\n%s", err, out)
	}

	go func() {
		time.Sleep(5 * time.Second)
		os.Remove(scriptPath)
		exec.Command("tmux", "send-keys", "-t", tmuxSession, "/work "+ticketID, "Enter").Run()
	}()

	return &Session{TicketID: ticketID, tmuxSession: tmuxSession}, nil
}

func (s *Session) WriteText(answer string) error {
	return exec.Command("tmux", "send-keys", "-t", s.tmuxSession, answer, "Enter").Run()
}

func (s *Session) WriteSelectIndex(index int) error {
	for i := 0; i < index; i++ {
		if err := exec.Command("tmux", "send-keys", "-t", s.tmuxSession, "Down").Run(); err != nil {
			return err
		}
		time.Sleep(80 * time.Millisecond)
	}
	return exec.Command("tmux", "send-keys", "-t", s.tmuxSession, "Enter").Run()
}

func (s *Session) Stop() {
	exec.Command("tmux", "kill-session", "-t", s.tmuxSession).Run()
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
