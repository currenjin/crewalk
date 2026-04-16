package main

import (
	"context"
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
	if len(os.Args) > 1 && os.Args[1] == "--demo" {
		runDemoMode()
		return
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var p *tea.Program

	m := tui.New()
	m.OnStartTicket = func(ticketID string) {
		if err := sessionMgr.StartTicket(ticketID); err != nil {
			p.Send(tui.TicketErrorMsg{TicketID: ticketID, Err: err})
			return
		}
		p.Send(tui.StatusMsg{Text: ticketID + " started"})
	}

	p = tea.NewProgram(m, tea.WithAltScreen())

	go forwardPhaseEvents(ctx, w, p)
	go forwardQuestionEvents(ctx, w, p, sessionMgr)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cancel()
	w.Stop()
}

func runDemoMode() {
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	go runDemo(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func forwardPhaseEvents(ctx context.Context, w *watcher.Watcher, p *tea.Program) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.Events():
			if !ok {
				return
			}
			p.Send(tui.PhaseChangeMsg{
				TicketID:  event.TicketID,
				Phase:     tui.PhaseFromString(event.Phase),
				Status:    event.Status,
				JSONLPath: event.JSONLPath,
			})
		}
	}
}

func forwardQuestionEvents(ctx context.Context, w *watcher.Watcher, p *tea.Program, mgr *session.Manager) {
	for {
		select {
		case <-ctx.Done():
			return
		case qEvent, ok := <-w.Questions():
			if !ok {
				return
			}
			responseCh := make(chan tui.QuestionResponse, 1)
			p.Send(tui.AskQuestionMsg{
				TicketID: qEvent.TicketID,
				Text:     qEvent.Text,
				Options:  qEvent.Options,
				Response: responseCh,
			})
			go func(ticketID string, ch <-chan tui.QuestionResponse) {
				select {
				case resp := <-ch:
					if resp.OptionIndex >= 0 {
						mgr.WriteSelectToSession(ticketID, resp.OptionIndex)
					} else {
						mgr.WriteTextToSession(ticketID, resp.Text)
					}
				case <-ctx.Done():
				}
			}(qEvent.TicketID, responseCh)
		}
	}
}
