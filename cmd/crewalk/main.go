package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/currenjin/crewalk/internal/config"
	"github.com/currenjin/crewalk/internal/session"
	"github.com/currenjin/crewalk/internal/tui"
	"github.com/currenjin/crewalk/internal/watcher"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get home dir: %v\n", err)
		os.Exit(1)
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	w := watcher.New(projectsDir)
	w.Start()

	sessionMgr := session.NewManager(cfg)

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	m.OnStartTicket = func(ticketID string) {
		if err := sessionMgr.StartTicket(ticketID); err != nil {
			p.Send(tui.TicketErrorMsg{TicketID: ticketID, Err: err})
		}
	}

	go forwardWatcherEvents(w, p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func forwardWatcherEvents(w *watcher.Watcher, p *tea.Program) {
	for event := range w.Events() {
		p.Send(tui.PhaseChangeMsg{
			TicketID: event.TicketID,
			Phase:    tui.PhaseFromString(event.Phase),
			Status:   event.Status,
		})
	}
}
