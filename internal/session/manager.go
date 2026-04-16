package session

import (
	"fmt"
	"log"
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

func (m *Manager) StartTicket(ticketID string) (logPath string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[ticketID]; exists {
		return "", fmt.Errorf("ticket %s is already running", ticketID)
	}

	worktreePath := m.cfg.WorktreePath(ticketID)

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if err := m.git.CreateWorktree(ticketID, worktreePath); err != nil {
			return "", fmt.Errorf("create worktree: %w", err)
		}
	}

	s, err := Start(ticketID, worktreePath, m.cfg.Claude.Command, m.cfg.Claude.Args)
	if err != nil {
		return "", fmt.Errorf("start session: %w", err)
	}

	m.sessions[ticketID] = s
	go m.waitAndCleanup(ticketID, s)

	return s.LogPath, nil
}

func (m *Manager) waitAndCleanup(ticketID string, s *Session) {
	s.Wait()

	m.mu.Lock()
	delete(m.sessions, ticketID)
	m.mu.Unlock()

	log.Printf("session finished: %s", ticketID)
}
