package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/currenjin/crewalk/internal/tui"
)

type demoStep struct {
	delay    time.Duration
	ticketID string
	phase    tui.Phase
	status   string
	add      bool
	question string
}

var demoScript = []demoStep{
	{delay: 800 * time.Millisecond, ticketID: "RP-1234", add: true, status: "reading ticket..."},
	{delay: 1800 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhaseBranching, status: "creating branch..."},
	{delay: 1500 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhaseCoding, status: "implementing..."},
	{delay: 1200 * time.Millisecond, ticketID: "RP-5678", add: true, status: "reading ticket..."},
	{delay: 2000 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhaseReviewing, status: "running tests..."},
	{delay: 1000 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhaseBranching, status: "creating branch..."},
	{delay: 1500 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhasePushing, status: "opening PR..."},
	{delay: 800 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhaseCoding, status: "implementing..."},
	{delay: 1200 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhaseDone, status: "done"},
	{delay: 1500 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhaseReviewing, status: "running tests...", question: "Should I squash commits before merging?"},
	{delay: 3000 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhasePushing, status: "opening PR..."},
	{delay: 1500 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhaseDone, status: "done"},
}

func runDemo(p *tea.Program) {
	for _, step := range demoScript {
		time.Sleep(step.delay)
		if step.add {
			p.Send(tui.AddTicketMsg{TicketID: step.ticketID, Status: step.status})
			continue
		}
		if step.question != "" {
			responseCh := make(chan string, 1)
			p.Send(tui.AskQuestionMsg{
				TicketID: step.ticketID,
				Text:     step.question,
				Response: responseCh,
			})
			go func(ch chan string, ticketID string, phase tui.Phase, status string, delay time.Duration) {
				time.Sleep(2500 * time.Millisecond)
				ch <- "yes, squash them"
				time.Sleep(delay)
				p.Send(tui.PhaseChangeMsg{TicketID: ticketID, Phase: phase, Status: status})
			}(responseCh, step.ticketID, step.phase, step.status, step.delay)
			continue
		}
		p.Send(tui.PhaseChangeMsg{
			TicketID: step.ticketID,
			Phase:    step.phase,
			Status:   step.status,
		})
	}
}
