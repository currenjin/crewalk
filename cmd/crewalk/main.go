package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/currenjin/crewalk/internal/tui"
)

func main() {
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	go simulateWork(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "오류: %v\n", err)
		os.Exit(1)
	}
}

func simulateWork(p *tea.Program) {
	time.Sleep(2 * time.Second)

	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-5678",
		Phase:    tui.PhaseBranching,
		Status:   "브랜치 생성 중",
	})

	time.Sleep(3 * time.Second)

	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-5678",
		Phase:    tui.PhaseCoding,
		Status:   "go 루프 중",
	})

	time.Sleep(2 * time.Second)

	responseCh := make(chan string, 1)
	p.Send(tui.AskQuestionMsg{
		TicketID: "RP-1234",
		Text:     "브랜치를 어떻게 만들까요?\n  [ yes ]  [ worktree ]  [ skip ]",
		Response: responseCh,
	})

	time.Sleep(500 * time.Millisecond)

	responseCh2 := make(chan string, 1)
	p.Send(tui.AskQuestionMsg{
		TicketID: "RP-9012",
		Text:     "PR 베이스를 develop으로 할까요?\n  [ yes ]  [ no ]",
		Response: responseCh2,
	})

	<-responseCh
	<-responseCh2

	time.Sleep(1 * time.Second)
	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-1234",
		Phase:    tui.PhasePushing,
		Status:   "PR 생성 중",
	})

	time.Sleep(3 * time.Second)
	p.Send(tui.PhaseChangeMsg{
		TicketID: "RP-1234",
		Phase:    tui.PhaseDone,
		Status:   "완료",
	})
}
