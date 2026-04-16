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
	Options  []string
	Response chan QuestionResponse
}

type Ticket struct {
	ID        string
	Phase     Phase
	Status    string
	PosX      float64
	TargetX   float64
	IsMoving  bool
	WalkFrame int
	JSONLPath string
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
	modeConfirmRemove
)

type Model struct {
	tickets          []*Ticket
	questionQueue    []*Ticket
	inputBuffer      string
	mode             inputMode
	statusMsg        string
	width            int
	height           int
	tick             int
	selectedIdx      int
	logLines         []string
	questionSelected  int  // highlighted option index
	questionTextMode  bool // true when typing custom text
	confirmRemoveIdx  int

	OnStartTicket  func(ticketID string)
	OnRemoveTicket func(ticketID string)
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
			t := m.tickets[m.selectedIdx]
			if t.JSONLPath != "" {
				m.logLines = readLastLines(t.JSONLPath, 40)
			}
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
					t := m.tickets[m.selectedIdx]
					if t.JSONLPath != "" {
						m.logLines = readLastLines(t.JSONLPath, 40)
					}
				}
			case tea.KeyRunes:
				switch msg.String() {
				case "n":
					m.mode = modeNewTicket
					m.inputBuffer = ""
					m.statusMsg = ""
				case "d":
					if len(m.tickets) > 0 && m.selectedIdx < len(m.tickets) {
						m.confirmRemoveIdx = m.selectedIdx
						m.mode = modeConfirmRemove
					}
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
			if len(m.questionQueue) == 0 {
				m.mode = modeNormal
				break
			}
			asking := m.questionQueue[0]
			hasOptions := asking.Question != nil && len(asking.Question.Options) > 0

			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				if m.questionTextMode {
					m.questionTextMode = false
					m.inputBuffer = ""
				}
			case tea.KeyUp:
				if !m.questionTextMode && hasOptions && m.questionSelected > 0 {
					m.questionSelected--
				}
			case tea.KeyDown:
				if !m.questionTextMode && hasOptions && m.questionSelected < len(asking.Question.Options)-1 {
					m.questionSelected++
				}
			case tea.KeyEnter:
				if asking.Question != nil {
					var resp QuestionResponse
					if m.questionTextMode || !hasOptions {
						resp = QuestionResponse{OptionIndex: -1, Text: m.inputBuffer}
					} else {
						resp = QuestionResponse{OptionIndex: m.questionSelected, Text: asking.Question.Options[m.questionSelected]}
					}
					asking.Question.Response <- resp
					asking.IsAsking = false
					asking.Question = nil
					m.questionQueue = m.questionQueue[1:]
					m.inputBuffer = ""
					m.questionSelected = 0
					m.questionTextMode = false
					if len(m.questionQueue) > 0 {
						m.questionQueue[0].IsAsking = true
					} else {
						m.mode = modeNormal
					}
				}
			case tea.KeyBackspace:
				if m.questionTextMode && len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				}
			case tea.KeyTab:
				if hasOptions {
					m.questionTextMode = !m.questionTextMode
					m.inputBuffer = ""
				}
			default:
				if hasOptions && !m.questionTextMode {
					m.questionTextMode = true
					m.inputBuffer = msg.String()
				} else {
					m.inputBuffer += msg.String()
				}
			}
		case modeConfirmRemove:
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.mode = modeNormal
			case tea.KeyEnter:
				m.selectedIdx = m.confirmRemoveIdx
				m.removeSelectedTicket()
				m.mode = modeNormal
			case tea.KeyRunes:
				switch msg.String() {
				case "y", "Y":
					m.selectedIdx = m.confirmRemoveIdx
					m.removeSelectedTicket()
					m.mode = modeNormal
				case "n", "N":
					m.mode = modeNormal
				}
			}
		}

	case AskQuestionMsg:
		ticket := m.findTicket(msg.TicketID)
		if ticket == nil {
			msg.Response <- QuestionResponse{OptionIndex: 0}
			break
		}
		ticket.Question = &Question{Text: msg.Text, Options: msg.Options, Response: msg.Response}
		ticket.IsAsking = true
		m.questionQueue = append(m.questionQueue, ticket)
		if len(m.questionQueue) == 1 {
			m.mode = modeQuestion
			m.questionSelected = 0
			m.questionTextMode = false
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
		if ticket != nil && msg.Phase >= ticket.Phase {
			ticket.Phase = msg.Phase
			ticket.Status = msg.Status
			ticket.TargetX = float64(msg.Phase)
			ticket.IsMoving = true
			if msg.JSONLPath != "" {
				ticket.JSONLPath = msg.JSONLPath
			}
		} else if ticket != nil && msg.JSONLPath != "" {
			ticket.JSONLPath = msg.JSONLPath
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

func (m *Model) removeSelectedTicket() {
	t := m.tickets[m.selectedIdx]

	for i, qt := range m.questionQueue {
		if qt.ID == t.ID {
			if qt.Question != nil {
				qt.Question.Response <- QuestionResponse{OptionIndex: -1, Text: ""}
			}
			m.questionQueue = append(m.questionQueue[:i], m.questionQueue[i+1:]...)
			break
		}
	}

	m.tickets = append(m.tickets[:m.selectedIdx], m.tickets[m.selectedIdx+1:]...)
	if m.selectedIdx >= len(m.tickets) && m.selectedIdx > 0 {
		m.selectedIdx--
	}
	if len(m.questionQueue) == 0 && m.mode == modeQuestion {
		m.mode = modeNormal
	}

	if m.OnRemoveTicket != nil {
		go m.OnRemoveTicket(t.ID)
	}
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
		m.renderConfirmRemove(),
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

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 3).
			Width(60)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Bold(true)

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

	asking := m.questionQueue[0]
	if asking.Question == nil {
		return ""
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("🧑 %s", ticketNameStyle.Render(asking.ID)))
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("? "+asking.Question.Text))
	lines = append(lines, "")

	if len(asking.Question.Options) > 0 && !m.questionTextMode {
		for i, opt := range asking.Question.Options {
			if i == m.questionSelected {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render("❯ "+opt))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  "+opt))
			}
		}
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[↑↓] 이동  [Enter] 선택  [Tab] 직접입력"))
	} else {
		lines = append(lines, inputStyle.Render("> "+m.inputBuffer+"_"))
		lines = append(lines, "")
		if len(asking.Question.Options) > 0 {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[Enter] 전송  [Tab] 목록으로"))
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[Enter] 전송"))
		}
	}

	if len(m.questionQueue) > 1 {
		waiting := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
			fmt.Sprintf("  (대기 중: %d개)", len(m.questionQueue)-1),
		)
		lines = append(lines, waiting)
	}

	return "\n" + inputBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderConfirmRemove() string {
	if m.mode != modeConfirmRemove {
		return ""
	}
	if m.confirmRemoveIdx >= len(m.tickets) {
		return ""
	}
	t := m.tickets[m.confirmRemoveIdx]
	content := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("Remove ticket") +
		"\n\n" +
		fmt.Sprintf("티켓 %s을 제거할까요? tmux 세션과 worktree가 삭제됩니다.\n", ticketNameStyle.Render(t.ID)) +
		"\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[y / Enter] 제거  [n / Esc] 취소")

	return "\n" + inputBoxStyle.Render(content)
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

	return "\n" + inputBoxStyle.Render(content)
}

func (m Model) renderStatusBar() string {
	var parts []string

	if m.statusMsg != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(m.statusMsg))
	}

	if m.mode == modeNormal && len(m.questionQueue) == 0 {
		hint := "[n] new ticket  [ctrl+c] quit"
		if len(m.tickets) > 0 {
			hint = "[n] new ticket  [tab] select  [enter] detail  [d] remove  [ctrl+c] quit"
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
		msg := "waiting for activity..."
		if t.JSONLPath == "" {
			msg = "Claude is starting in Terminal..."
		}
		logText = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).Render(msg)
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
