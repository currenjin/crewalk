package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/currenjin/crewalk/internal/ipc"
	"github.com/currenjin/crewalk/internal/tui"
)

func main() {
	server := ipc.NewServer()
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start IPC server: %v\n", err)
		os.Exit(1)
	}
	defer server.Stop()

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	go forwardIPCEvents(server, p)
	go simulateWork(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func forwardIPCEvents(server *ipc.Server, p *tea.Program) {
	for event := range server.Events() {
		switch event.Type {
		case ipc.EventPhaseChange:
			p.Send(tui.PhaseChangeMsg{
				TicketID: event.TicketID,
				Phase:    tui.PhaseFromString(event.Phase),
				Status:   event.Status,
			})
		case ipc.EventAskQuestion:
			responseCh := make(chan string, 1)
			p.Send(tui.AskQuestionMsg{
				TicketID: event.TicketID,
				Text:     event.Text,
				Response: responseCh,
			})
		}
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
