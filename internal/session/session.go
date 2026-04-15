package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Session struct {
	TicketID string
	LogPath  string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	done     chan struct{}
}

func Start(ticketID, worktreePath, claudeCmd string, claudeArgs []string) (*Session, error) {
	logPath, logFile, err := openLogFile(ticketID)
	if err != nil {
		return nil, fmt.Errorf("log file: %w", err)
	}

	args := make([]string, 0, len(claudeArgs)+9)
	args = append(args, claudeArgs...)
	args = append(args, "-p", fmt.Sprintf("/work %s", ticketID))
	args = append(args, "--output-format", "stream-json")
	args = append(args, "--verbose")
	args = append(args, "--append-system-prompt",
		"The git worktree has already been created for this ticket's branch. "+
			"For the branch creation question, automatically choose 'skip'. "+
			"For the PR creation question at the end, automatically choose 'push-pr'.",
	)

	cmd := exec.Command(claudeCmd, args...)
	cmd.Dir = worktreePath
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start claude: %w", err)
	}

	go func() { cmd.Wait(); logFile.Close() }()

	return &Session{
		TicketID: ticketID,
		LogPath:  logPath,
		cmd:      cmd,
		stdin:    stdin,
		done:     make(chan struct{}),
	}, nil
}

func (s *Session) Write(input string) error {
	_, err := fmt.Fprint(s.stdin, input)
	return err
}

func (s *Session) Stop() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}

	s.stdin.Close()

	if s.cmd.Process == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-ctx.Done():
		s.cmd.Process.Kill()
		s.cmd.Wait()
	case <-done:
	}
}

func (s *Session) Wait() error {
	return s.cmd.Wait()
}

func openLogFile(ticketID string) (path string, f *os.File, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(home, ".crewalk", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, err
	}
	path = filepath.Join(dir, ticketID+".log")
	f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	return path, f, err
}
