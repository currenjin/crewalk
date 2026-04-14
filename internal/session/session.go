package session

import (
	"fmt"
	"io"
	"os/exec"
	"time"
)

type Session struct {
	TicketID string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
}

func Start(ticketID, worktreePath, claudeCmd string, claudeArgs []string) (*Session, error) {
	args := append(claudeArgs, "--cwd", worktreePath)
	cmd := exec.Command(claudeCmd, args...)
	cmd.Dir = worktreePath

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	s := &Session{
		TicketID: ticketID,
		cmd:      cmd,
		stdin:    stdin,
	}

	go s.injectInitialCommand(ticketID)

	return s, nil
}

func (s *Session) injectInitialCommand(ticketID string) {
	time.Sleep(3 * time.Second)
	s.Write(fmt.Sprintf("/work %s\n", ticketID))
}

func (s *Session) Write(input string) error {
	_, err := fmt.Fprint(s.stdin, input)
	return err
}

func (s *Session) Stop() {
	s.stdin.Close()
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}

func (s *Session) Wait() error {
	return s.cmd.Wait()
}
