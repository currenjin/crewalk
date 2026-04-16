package session

import (
	"fmt"
	"os"
	"sync"

	"github.com/currenjin/crewalk/internal/config"
	"github.com/currenjin/crewalk/internal/git"
)

type Manager struct {
	cfg      *config.Config
	git      *git.Manager
	sessions map[string]*Session
	mu       sync.Mutex
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:      cfg,
		git:      git.NewManager(cfg.Project.Path, cfg.Project.WorktreeBase),
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) StartTicket(ticketID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[ticketID]; exists {
		return fmt.Errorf("ticket %s is already running", ticketID)
	}

	worktreePath := m.cfg.WorktreePath(ticketID)

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if err := m.git.CreateWorktree(ticketID, worktreePath); err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
	}

	s, err := Start(ticketID, worktreePath, m.cfg.Claude.Command, m.cfg.Claude.Args)
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}

	m.sessions[ticketID] = s
	return nil
}

func (m *Manager) WriteSelectToSession(ticketID string, index int) error {
	m.mu.Lock()
	s, exists := m.sessions[ticketID]
	m.mu.Unlock()
	if !exists {
		return fmt.Errorf("no session for ticket %s", ticketID)
	}
	return s.WriteSelectIndex(index)
}

func (m *Manager) WriteTextToSession(ticketID, text string) error {
	m.mu.Lock()
	s, exists := m.sessions[ticketID]
	m.mu.Unlock()
	if !exists {
		return fmt.Errorf("no session for ticket %s", ticketID)
	}
	return s.WriteText(text)
}

func (m *Manager) StopTicket(ticketID string) {
	m.mu.Lock()
	s, exists := m.sessions[ticketID]
	delete(m.sessions, ticketID)
	m.mu.Unlock()

	if exists {
		s.Stop()
	}

	worktreePath := m.cfg.WorktreePath(ticketID)
	m.git.RemoveWorktree(worktreePath)
}
