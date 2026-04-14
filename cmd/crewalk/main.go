package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/currenjin/crewalk/internal/tui"
	"github.com/currenjin/crewalk/internal/watcher"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get home dir: %v\n", err)
		os.Exit(1)
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	w := watcher.New(projectsDir)
	w.Start()

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	go forwardWatcherEvents(w, p)
	go simulateWork(p)

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

func simulateWork(p *tea.Program) {
	time.Sleep(2 * time.Second)

	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-5678",
		Phase:    tui.PhaseBranching,
		Status:   "creating branch",
	})

	time.Sleep(3 * time.Second)

	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-5678",
		Phase:    tui.PhaseCoding,
		Status:   "implementing",
	})

	time.Sleep(2 * time.Second)

	responseCh := make(chan string, 1)
	p.Send(tui.AskQuestionMsg{
		TicketID: "RP-1234",
		Text:     "How should we create the branch?\n  [ yes ]  [ worktree ]  [ skip ]",
		Response: responseCh,
	})

	time.Sleep(500 * time.Millisecond)

	responseCh2 := make(chan string, 1)
	p.Send(tui.AskQuestionMsg{
		TicketID: "RP-9012",
		Text:     "Use develop as PR base branch?\n  [ yes ]  [ no ]",
		Response: responseCh2,
	})

	<-responseCh
	<-responseCh2

	time.Sleep(1 * time.Second)
	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-1234",
		Phase:    tui.PhasePushing,
		Status:   "creating PR",
	})

	time.Sleep(3 * time.Second)
	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-1234",
		Phase:    tui.PhaseDone,
		Status:   "done",
	})
}
