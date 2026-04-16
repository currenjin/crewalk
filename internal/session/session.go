package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Session struct {
	TicketID string
	LogPath  string
	cmd      *exec.Cmd
	mu       sync.Mutex
	done     chan struct{}
	doneOnce sync.Once
}

const autoAnswerPrompt = "AUTOMATED MODE: Do NOT use the AskUserQuestion tool at any point. " +
	"Make all decisions automatically: " +
	"(1) Branch creation → choose 'skip' (git worktree is already created). " +
	"(2) Plan save confirmation → save immediately ('저장'). " +
	"(3) Plan file selection → select the first/newest option. " +
	"(4) After completing each task (go) → always commit the changes next. " +
	"(5) After each commit → proceed to the next task (go). " +
	"(6) After all tasks are complete → choose 'push-pr'. " +
	"Complete the entire workflow from start to PR creation without stopping or asking questions."

func Start(ticketID, worktreePath, claudeCmd string, claudeArgs []string) (*Session, error) {
	logPath, logFile, err := openLogFile(ticketID)
	if err != nil {
		return nil, fmt.Errorf("log file: %w", err)
	}

	args := make([]string, 0, len(claudeArgs)+4)
	args = append(args, claudeArgs...)
	args = append(args, "--append-system-prompt", autoAnswerPrompt)
	args = append(args, "-p", "/work "+ticketID)

	cmd := exec.Command(claudeCmd, args...)
	cmd.Dir = worktreePath

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("open devnull: %w", err)
	}
	cmd.Stdin = devNull

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		devNull.Close()
		return nil, fmt.Errorf("start claude: %w", err)
	}

	s := &Session{
		TicketID: ticketID,
		LogPath:  logPath,
		cmd:      cmd,
		done:     make(chan struct{}),
	}

	go func() {
		io.Copy(io.Discard, devNull)
		cmd.Wait()
		logFile.Close()
		devNull.Close()
		s.doneOnce.Do(func() { close(s.done) })
	}()

	return s, nil
}

func (s *Session) Wait() {
	<-s.done
}

func (s *Session) Stop() {
	s.doneOnce.Do(func() { close(s.done) })

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
