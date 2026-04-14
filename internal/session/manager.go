package session

import (
	"fmt"
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

	if err := m.git.CreateWorktree(ticketID, worktreePath); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	s, err := Start(ticketID, worktreePath, m.cfg.Claude.Command, m.cfg.Claude.Args)
	if err != nil {
		m.git.RemoveWorktree(worktreePath)
		return fmt.Errorf("start session: %w", err)
	}

	m.sessions[ticketID] = s

	go m.waitAndCleanup(ticketID, worktreePath)

	return nil
}

func (m *Manager) WriteToSession(ticketID, input string) error {
	m.mu.Lock()
	s, exists := m.sessions[ticketID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("no session for ticket %s", ticketID)
	}
	return s.Write(input)
}

func (m *Manager) StopTicket(ticketID string) {
	m.mu.Lock()
	s, exists := m.sessions[ticketID]
	if exists {
		delete(m.sessions, ticketID)
	}
	m.mu.Unlock()

	if exists {
		s.Stop()
		m.git.RemoveWorktree(m.cfg.WorktreePath(ticketID))
	}
}

func (m *Manager) waitAndCleanup(ticketID, worktreePath string) {
	s := m.sessions[ticketID]
	if s == nil {
		return
	}
	s.Wait()

	m.mu.Lock()
	delete(m.sessions, ticketID)
	m.mu.Unlock()

	m.git.RemoveWorktree(worktreePath)
}
