package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Phase int

const (
	PhasePlanning Phase = iota
	PhaseBranching
	PhaseCoding
	PhaseReviewing
	PhasePushing
	PhaseDone
)

func (p Phase) String() string {
	switch p {
	case PhasePlanning:
		return "PLANNING"
	case PhaseBranching:
		return "BRANCHING"
	case PhaseCoding:
		return "CODING"
	case PhaseReviewing:
		return "REVIEWING"
	case PhasePushing:
		return "PUSH/PR"
	case PhaseDone:
		return "DONE"
	}
	return "UNKNOWN"
}

var phaseOrder = []Phase{
	PhasePlanning, PhaseBranching, PhaseCoding, PhaseReviewing, PhasePushing, PhaseDone,
}

var walkFrames = []string{"🚶", "🚶", "🧍", "🧍"}

type Question struct {
	Text     string
	Response chan string
}

type Ticket struct {
	ID          string
	Phase       Phase
	Status      string
	PosX        float64
	TargetX     float64
	IsMoving    bool
	WalkFrame   int
	Question    *Question
	IsAsking    bool
}

type tickMsg time.Time

type Model struct {
	tickets      []*Ticket
	questionQueue []*Ticket
	inputBuffer  string
	width        int
	height       int
	tick         int
}

func New() Model {
	return Model{
		tickets: []*Ticket{
			{ID: "RP-1234", Phase: PhaseCoding, Status: "go 루프 중", PosX: 2, TargetX: 2},
			{ID: "RP-5678", Phase: PhasePlanning, Status: "계획 생성 중", PosX: 0, TargetX: 0},
			{ID: "RP-9012", Phase: PhaseReviewing, Status: "리뷰 중", PosX: 3, TargetX: 3},
			{ID: "RP-0001", Phase: PhaseDone, Status: "완료", PosX: 5, TargetX: 5},
		},
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.tick++
		for _, t := range m.tickets {
			if t.IsMoving {
				diff := t.TargetX - t.PosX
				if diff > 0.08 {
					t.PosX += 0.08
				} else if diff < -0.08 {
					t.PosX -= 0.08
				} else {
					t.PosX = t.TargetX
					t.IsMoving = false
					t.WalkFrame = 0
				}
				t.WalkFrame = (t.WalkFrame + 1) % len(walkFrames)
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.questionQueue) > 0 {
				asking := m.questionQueue[0]
				if asking.Question != nil {
					asking.Question.Response <- m.inputBuffer
					asking.IsAsking = false
					asking.Question = nil
					m.questionQueue = m.questionQueue[1:]
					if len(m.questionQueue) > 0 {
						m.questionQueue[0].IsAsking = true
					}
				}
				m.inputBuffer = ""
			}
		case tea.KeyBackspace:
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
		default:
			if len(m.questionQueue) > 0 {
				m.inputBuffer += msg.String()
			}
		}

	case AskQuestionMsg:
		ticket := m.findTicket(msg.TicketID)
		if ticket != nil {
			ticket.Question = &Question{Text: msg.Text, Response: msg.Response}
			m.questionQueue = append(m.questionQueue, ticket)
			if len(m.questionQueue) == 1 {
				ticket.IsAsking = true
			}
		}

	case PhaseChangeMsg:
		ticket := m.findTicket(msg.TicketID)
		if ticket != nil {
			ticket.Phase = msg.Phase
			ticket.Status = msg.Status
			ticket.TargetX = float64(msg.Phase)
			ticket.IsMoving = true
		}
	}

	return m, nil
}

func (m Model) findTicket(id string) *Ticket {
	for _, t := range m.tickets {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "로딩 중..."
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.renderRooms(),
		m.renderCorridor(),
		m.renderQuestionArea(),
	)
}

var (
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Padding(0, 2)

	roomStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2).
			Width(16).
			Height(7)

	activeRoomStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(1, 2).
			Width(16).
			Height(7)

	doneRoomStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("46")).
			Padding(1, 2).
			Width(16).
			Height(7)

	ticketNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)

	corridorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Height(3)

	questionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")).
				Padding(1, 3).
				Width(60)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Bold(true)

	waitingCharStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

func (m Model) renderHeader() string {
	now := time.Now().Format("2006-01-02 15:04:05")
	title := "🏢 ROOUTY WORK DASHBOARD"
	padding := m.width - len(title) - len(now) - 4
	if padding < 1 {
		padding = 1
	}
	return headerStyle.Width(m.width).Render(
		title + strings.Repeat(" ", padding) + now,
	)
}

func (m Model) renderRooms() string {
	rooms := make([]string, len(phaseOrder))
	for i, phase := range phaseOrder {
		rooms[i] = m.renderRoom(phase)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rooms...)
}

func (m Model) renderRoom(phase Phase) string {
	var ticketsInRoom []string
	for _, t := range m.tickets {
		if t.Phase == phase && !t.IsMoving && !t.IsAsking {
			sprite := "🧑"
			if phase == PhaseDone {
				sprite = "✅"
			}
			ticketsInRoom = append(ticketsInRoom,
				ticketNameStyle.Render(t.ID)+"\n  "+sprite+"\n"+statusStyle.Render(t.Status),
			)
		}
	}

	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render(phase.String())
	if len(ticketsInRoom) > 0 {
		content += "\n\n" + strings.Join(ticketsInRoom, "\n\n")
	}

	style := roomStyle
	if len(ticketsInRoom) > 0 {
		style = activeRoomStyle
	}
	if phase == PhaseDone {
		style = doneRoomStyle
	}

	return style.Render(content)
}

func (m Model) renderCorridor() string {
	width := m.width
	corridor := strings.Repeat("─", width)

	for _, t := range m.tickets {
		if t.IsMoving {
			roomWidth := 18
			posX := int(t.PosX*float64(roomWidth)) + 2
			if posX < 0 {
				posX = 0
			}
			if posX >= width-4 {
				posX = width - 5
			}
			frame := walkFrames[t.WalkFrame%len(walkFrames)]
			label := fmt.Sprintf("%s%s", t.ID, frame)
			before := posX
			after := width - before - len([]rune(label))
			if after < 0 {
				after = 0
			}
			corridor = strings.Repeat(" ", before) + label + strings.Repeat(" ", after)
		}
	}

	return corridorStyle.Render(corridor)
}

func (m Model) renderQuestionArea() string {
	if len(m.questionQueue) == 0 {
		return ""
	}

	var lines []string

	waitingLine := ""
	for i, t := range m.questionQueue {
		if i == 0 {
			lines = append(lines, fmt.Sprintf("🧑 %s", ticketNameStyle.Render(t.ID)))
		} else {
			waitingLine += waitingCharStyle.Render(fmt.Sprintf("  🧍 %s (대기)", t.ID))
		}
	}
	if waitingLine != "" {
		lines = append(lines, waitingLine)
	}

	asking := m.questionQueue[0]
	if asking.Question != nil {
		lines = append(lines, "")
		lines = append(lines, asking.Question.Text)
		lines = append(lines, "")
		lines = append(lines, inputStyle.Render("> "+m.inputBuffer+"_"))
	}

	return "\n" + questionBoxStyle.Render(strings.Join(lines, "\n"))
}
