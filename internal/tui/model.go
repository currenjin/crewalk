package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Phase int

const (
	PhaseBranching Phase = iota
	PhasePlanning
	PhaseCoding
	PhasePushing
	PhaseDone
)

func PhaseFromString(s string) Phase {
	switch s {
	case "BRANCHING":
		return PhaseBranching
	case "PLANNING":
		return PhasePlanning
	case "CODING":
		return PhaseCoding
	case "PUSHING":
		return PhasePushing
	case "DONE":
		return PhaseDone
	}
	return PhaseBranching
}

func (p Phase) String() string {
	switch p {
	case PhaseBranching:
		return "BRANCHING"
	case PhasePlanning:
		return "PLANNING"
	case PhaseCoding:
		return "CODING"
	case PhasePushing:
		return "PUSH/PR"
	case PhaseDone:
		return "DONE"
	}
	return "UNKNOWN"
}

var phaseOrder = []Phase{
	PhaseBranching, PhasePlanning, PhaseCoding, PhasePushing, PhaseDone,
}

var walkFrames = []string{"🚶", "🚶", "🧍", "🧍"}

type Question struct {
	Text     string
	Response chan string
}

type Ticket struct {
	ID        string
	Phase     Phase
	Status    string
	PosX      float64
	TargetX   float64
	IsMoving  bool
	WalkFrame int
	Question  *Question
	IsAsking  bool
}

type tickMsg time.Time

type inputMode int

const (
	modeNormal inputMode = iota
	modeNewTicket
	modeQuestion
	modeDetail
)

type Model struct {
	tickets       []*Ticket
	questionQueue []*Ticket
	inputBuffer   string
	mode          inputMode
	statusMsg     string
	width         int
	height        int
	tick          int
	selectedIdx   int
	logLines      []string

	OnStartTicket func(ticketID string)
}

func New() Model {
	return Model{}
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
		if m.mode == modeDetail && len(m.tickets) > 0 && m.selectedIdx < len(m.tickets) {
			m.logLines = readLastLines(ticketLogPath(m.tickets[m.selectedIdx].ID), 40)
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch m.mode {
		case modeNormal:
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyTab:
				if len(m.tickets) > 0 {
					m.selectedIdx = (m.selectedIdx + 1) % len(m.tickets)
				}
			case tea.KeyEnter:
				if len(m.tickets) > 0 {
					if m.selectedIdx >= len(m.tickets) {
						m.selectedIdx = 0
					}
					m.mode = modeDetail
					m.logLines = readLastLines(ticketLogPath(m.tickets[m.selectedIdx].ID), 40)
				}
			case tea.KeyRunes:
				if msg.String() == "n" {
					m.mode = modeNewTicket
					m.inputBuffer = ""
					m.statusMsg = ""
				}
			}

		case modeDetail:
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.mode = modeNormal
			case tea.KeyRunes:
				if msg.String() == "q" {
					m.mode = modeNormal
				}
			}

		case modeNewTicket:
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.mode = modeNormal
				m.inputBuffer = ""
			case tea.KeyEnter:
				ticketID := strings.ToUpper(strings.TrimSpace(m.inputBuffer))
				if ticketID != "" {
					m.mode = modeNormal
					m.inputBuffer = ""
					m.tickets = append(m.tickets, &Ticket{
						ID:      ticketID,
						Phase:   PhaseBranching,
						Status:  "starting...",
						PosX:    0,
						TargetX: 0,
					})
					if m.OnStartTicket != nil {
						go m.OnStartTicket(ticketID)
					}
				}
			case tea.KeyBackspace:
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				}
			default:
				m.inputBuffer += msg.String()
			}

		case modeQuestion:
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
					if len(m.questionQueue) == 0 {
						m.mode = modeNormal
					}
				}
			case tea.KeyBackspace:
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				}
			default:
				m.inputBuffer += msg.String()
			}
		}

	case AskQuestionMsg:
		ticket := m.findTicket(msg.TicketID)
		if ticket == nil {
			msg.Response <- ""
			break
		}
		ticket.Question = &Question{Text: msg.Text, Response: msg.Response}
		m.questionQueue = append(m.questionQueue, ticket)
		if len(m.questionQueue) == 1 {
			ticket.IsAsking = true
			m.mode = modeQuestion
		}

	case AddTicketMsg:
		m.tickets = append(m.tickets, &Ticket{
			ID:      msg.TicketID,
			Phase:   PhaseBranching,
			Status:  msg.Status,
			PosX:    0,
			TargetX: 0,
		})

	case PhaseChangeMsg:
		ticket := m.findTicket(msg.TicketID)
		if ticket != nil {
			ticket.Phase = msg.Phase
			ticket.Status = msg.Status
			ticket.TargetX = float64(msg.Phase)
			ticket.IsMoving = true
		}

	case StatusMsg:
		m.statusMsg = msg.Text

	case TicketErrorMsg:
		m.statusMsg = fmt.Sprintf("error starting %s: %v", msg.TicketID, msg.Err)
		for i, t := range m.tickets {
			if t.ID == msg.TicketID {
				m.tickets = append(m.tickets[:i], m.tickets[i+1:]...)
				if m.selectedIdx >= len(m.tickets) && m.selectedIdx > 0 {
					m.selectedIdx--
				}
				break
			}
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
		return "loading..."
	}
	if m.mode == modeDetail {
		return m.renderDetail()
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.renderRooms(),
		m.renderCorridor(),
		m.renderQuestionArea(),
		m.renderNewTicketInput(),
		m.renderStatusBar(),
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

	selectedRoomStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")).
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

	detailBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(1, 2)

	logLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

func (m Model) renderHeader() string {
	now := time.Now().Format("2006-01-02 15:04:05")
	title := "🚶 CREWALK"
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
	selectedTicket := m.selectedTicket()

	for _, t := range m.tickets {
		if t.Phase == phase && !t.IsMoving && !t.IsAsking {
			sprite := "🧑"
			if phase == PhaseDone {
				sprite = "✅"
			}
			name := ticketNameStyle.Render(t.ID)
			if selectedTicket != nil && t.ID == selectedTicket.ID {
				name = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render("▶ " + t.ID)
			}
			ticketsInRoom = append(ticketsInRoom,
				name+"\n  "+sprite+"\n"+statusStyle.Render(t.Status),
			)
		}
	}

	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render(phase.String())
	if len(ticketsInRoom) > 0 {
		content += "\n\n" + strings.Join(ticketsInRoom, "\n\n")
	}

	style := roomStyle
	if selectedTicket != nil && selectedTicket.Phase == phase && !selectedTicket.IsMoving {
		style = selectedRoomStyle
	} else if len(ticketsInRoom) > 0 {
		style = activeRoomStyle
	}
	if phase == PhaseDone {
		style = doneRoomStyle
	}

	return style.Render(content)
}

func (m Model) selectedTicket() *Ticket {
	if len(m.tickets) == 0 || m.selectedIdx >= len(m.tickets) {
		return nil
	}
	return m.tickets[m.selectedIdx]
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

func (m Model) renderNewTicketInput() string {
	if m.mode != modeNewTicket {
		return ""
	}
	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("New ticket") +
		"\n\n" +
		"Ticket ID (e.g. RP-1234):\n" +
		inputStyle.Render("> "+strings.ToUpper(m.inputBuffer)+"_") +
		"\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[Enter] start  [Esc] cancel")

	return "\n" + questionBoxStyle.Render(content)
}

func (m Model) renderStatusBar() string {
	var parts []string

	if m.statusMsg != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(m.statusMsg))
	}

	if m.mode == modeNormal && len(m.questionQueue) == 0 {
		hint := "[n] new ticket  [ctrl+c] quit"
		if len(m.tickets) > 0 {
			hint = "[n] new ticket  [tab] select  [enter] enter room  [ctrl+c] quit"
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(hint))
	}

	if len(parts) == 0 {
		return ""
	}
	return "\n" + strings.Join(parts, "  ")
}

func (m Model) renderDetail() string {
	if len(m.tickets) == 0 || m.selectedIdx >= len(m.tickets) {
		m.mode = modeNormal
		return ""
	}
	t := m.tickets[m.selectedIdx]

	sprite := detailSprite(t.Phase, m.tick)

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		sprite+"  ",
		ticketNameStyle.Render(t.ID),
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  ·  "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true).Render(t.Phase.String()),
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).Render("  "+t.Status),
	)

	var logText string
	if len(m.logLines) == 0 {
		logText = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).Render("no output yet...")
	} else {
		colored := make([]string, len(m.logLines))
		for i, l := range m.logLines {
			colored[i] = logLineStyle.Render(l)
		}
		logText = strings.Join(colored, "\n")
	}

	innerH := m.height - 8
	if innerH < 5 {
		innerH = 5
	}

	content := header + "\n\n" + logText

	box := detailBoxStyle.
		Width(m.width - 4).
		Height(innerH).
		Render(content)

	statusBar := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
		Render("[q / esc] exit room  [ctrl+c] quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		"\n"+box,
		"\n"+statusBar,
	)
}

func friendlyToolName(name string) string {
	if strings.HasPrefix(name, "mcp__atlassian__") {
		action := strings.TrimPrefix(name, "mcp__atlassian__")
		switch {
		case strings.Contains(action, "jira_issue"):
			return "Jira"
		case strings.Contains(action, "confluence"):
			return "Confluence"
		default:
			return "Atlassian: " + action
		}
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.SplitN(strings.TrimPrefix(name, "mcp__"), "__", 2)
		if len(parts) == 2 {
			return parts[0] + ": " + parts[1]
		}
	}
	return name
}

func detailSprite(phase Phase, tick int) string {
	if phase == PhaseDone {
		return "🧍"
	}
	frames := []string{"🚶", "🚶‍", "🧍", "🧍"}
	return frames[tick%len(frames)]
}

func ticketLogPath(ticketID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".crewalk", "logs", ticketID+".log")
}

func readLastLines(path string, n int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var lines []string
	for _, line := range raw {
		if parsed := parseStreamJsonLine(line); parsed != "" {
			lines = append(lines, strings.Split(parsed, "\n")...)
		}
	}
	if len(lines) == 0 && len(raw) > 0 && raw[0] != "" {
		lines = raw
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func parseStreamJsonLine(line string) string {
	if line == "" {
		return ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return line
	}

	var entryType string
	if err := json.Unmarshal(obj["type"], &entryType); err != nil {
		return ""
	}

	switch entryType {
	case "assistant":
		var msg struct {
			Message struct {
				Content []struct {
					Type  string `json:"type"`
					Text  string `json:"text"`
					Name  string `json:"name"`
					Input struct {
						Command  string `json:"command"`
						Skill    string `json:"skill"`
						FilePath string `json:"file_path"`
						Path     string `json:"path"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return ""
		}
		var parts []string
		for _, item := range msg.Message.Content {
			switch item.Type {
			case "text":
				if t := strings.TrimSpace(item.Text); t != "" {
					parts = append(parts, t)
				}
			case "tool_use":
				switch item.Name {
				case "Bash":
					cmd := item.Input.Command
					if len(cmd) > 60 {
						cmd = cmd[:60] + "..."
					}
					parts = append(parts, "$ "+cmd)
				case "Skill":
					parts = append(parts, "/"+item.Input.Skill)
				case "Edit", "Write":
					p := item.Input.FilePath
					if p == "" {
						p = item.Input.Path
					}
					parts = append(parts, item.Name+": "+filepath.Base(p))
				case "Read":
					p := item.Input.FilePath
					if p == "" {
						p = item.Input.Path
					}
					parts = append(parts, "Read: "+filepath.Base(p))
				default:
					parts = append(parts, friendlyToolName(item.Name))
				}
			}
		}
		return strings.Join(parts, "\n")

	case "system":
		var sys struct {
			Subtype string `json:"subtype"`
			Stdout  string `json:"stdout"`
		}
		if err := json.Unmarshal([]byte(line), &sys); err != nil {
			return ""
		}
		if sys.Subtype == "hook_response" && strings.TrimSpace(sys.Stdout) != "" {
			return strings.TrimRight(sys.Stdout, "\n")
		}
		return ""

	case "result":
		var res struct {
			Result  string `json:"result"`
			IsError bool   `json:"is_error"`
		}
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			return ""
		}
		if res.Result != "" {
			return res.Result
		}
		return ""
	}

	return ""
}
