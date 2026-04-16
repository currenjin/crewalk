package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	logs     []string
}

var demoScript = []demoStep{
	{
		delay: 800 * time.Millisecond, ticketID: "RP-1234", add: true, status: "starting...",
		logs: []string{
			"> /work RP-1234",
			"Reading JIRA ticket RP-1234...",
		},
	},
	{
		delay: 1500 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhasePlanning, status: "generating plan...",
		logs: []string{
			"Generating plan for RP-1234...",
			"  - Analyzing requirements",
			"  - Checking existing code",
			"  - Writing plan to .claude/skills/augmented-coding/RP-1234-plan.md",
			"Plan created. Committing...",
		},
	},
	{
		delay: 1800 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhaseCoding, status: "implementing...",
		logs: []string{
			"Starting implementation...",
			"  Creating OrderSaver.kt",
			"  Creating OrderSaverTest.kt",
			"  Running tests...",
			"  ✓ OrderSaverTest: 주문을 저장한다 (23ms)",
			"  Committing: feat: RP-1234 주문 저장 기능 추가",
		},
	},
	{
		delay: 1200 * time.Millisecond, ticketID: "RP-5678", add: true, status: "starting...",
		logs: []string{
			"> /work RP-5678",
			"Reading JIRA ticket RP-5678...",
		},
	},
	{
		delay: 1000 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhasePlanning, status: "generating plan...",
		logs: []string{
			"Generating plan for RP-5678...",
		},
	},
	{
		delay: 2000 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhasePushing, status: "opening PR...",
		logs: []string{
			"Pushing branch feature/RP-1234...",
			"Creating pull request...",
			"PR created: https://github.com/org/repo/pull/42",
		},
	},
	{
		delay: 800 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhaseCoding, status: "implementing...",
		logs: []string{
			"Starting implementation...",
		},
	},
	{
		delay: 1200 * time.Millisecond, ticketID: "RP-1234", phase: tui.PhaseDone, status: "done",
		logs:  []string{"✓ All done."},
	},
	{
		delay: 1500 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhasePushing, status: "opening PR...",
		logs: []string{
			"Pushing branch feature/RP-5678...",
		},
	},
	{
		delay: 3000 * time.Millisecond, ticketID: "RP-5678", phase: tui.PhaseDone, status: "done",
		logs:  []string{"✓ All done."},
	},
}

func runDemo(p *tea.Program) {
	for _, step := range demoScript {
		time.Sleep(step.delay)

		if len(step.logs) > 0 {
			appendDemoLog(step.ticketID, step.logs)
		}

		if step.add {
			p.Send(tui.AddTicketMsg{TicketID: step.ticketID, Status: step.status})
			continue
		}
		p.Send(tui.PhaseChangeMsg{
			TicketID: step.ticketID,
			Phase:    step.phase,
			Status:   step.status,
		})
	}
}

func appendDemoLog(ticketID string, lines []string) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".crewalk", "logs")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, ticketID+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	for _, l := range lines {
		fmt.Fprintln(f, l)
	}
}
