package watcher

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type PhaseEvent struct {
	TicketID string
	Phase    string
	Status   string
}

type QuestionEvent struct {
	TicketID string
	Text     string
}

type jsonlEntry struct {
	Type    string        `json:"type"`
	Cwd     string        `json:"cwd"`
	Message *assistantMsg `json:"message,omitempty"`
}

type assistantMsg struct {
	Role    string        `json:"role"`
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

var ticketPattern = regexp.MustCompile(`(?i)((?:RP|TECH|DEV)-\d+)`)

type Watcher struct {
	projectsDir string
	events      chan PhaseEvent
	questions   chan QuestionEvent
	offsets     map[string]int64
}

func New(projectsDir string) *Watcher {
	return &Watcher{
		projectsDir: projectsDir,
		events:      make(chan PhaseEvent, 100),
		questions:   make(chan QuestionEvent, 20),
		offsets:     make(map[string]int64),
	}
}

func (w *Watcher) Events() <-chan PhaseEvent {
	return w.events
}

func (w *Watcher) Questions() <-chan QuestionEvent {
	return w.questions
}

func (w *Watcher) Start() {
	go w.poll()
}

func (w *Watcher) poll() {
	for {
		w.scanAll()
		time.Sleep(500 * time.Millisecond)
	}
}

func (w *Watcher) scanAll() {
	pattern := filepath.Join(w.projectsDir, "*", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, file := range files {
		w.scanFile(file)
	}
}

func (w *Watcher) scanFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	offset := w.offsets[path]
	if info.Size() <= offset {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	if offset > 0 {
		f.Seek(offset, 0)
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		w.parseLine(scanner.Text())
	}

	w.offsets[path] = info.Size()
}

func (w *Watcher) parseLine(line string) {
	var entry jsonlEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return
	}

	if entry.Type != "assistant" || entry.Message == nil {
		return
	}

	ticketID := extractTicketID(entry.Cwd)
	if ticketID == "" {
		return
	}

	for _, content := range entry.Message.Content {
		if content.Type != "tool_use" {
			continue
		}

		if content.Name == "AskUserQuestion" {
			text := extractQuestionText(content.Input)
			w.questions <- QuestionEvent{
				TicketID: ticketID,
				Text:     text,
			}
			continue
		}

		phase, status := detectPhase(content.Name, content.Input)
		if phase == "" {
			continue
		}
		w.events <- PhaseEvent{
			TicketID: ticketID,
			Phase:    phase,
			Status:   status,
		}
	}
}

func extractQuestionText(raw json.RawMessage) string {
	var input struct {
		Question string `json:"question"`
		Prompt   string `json:"prompt"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return "?"
	}
	if input.Question != "" {
		return input.Question
	}
	return input.Prompt
}

func extractTicketID(cwd string) string {
	base := filepath.Base(cwd)
	matches := ticketPattern.FindStringSubmatch(strings.ToUpper(base))
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func detectPhase(toolName string, toolInput json.RawMessage) (phase, status string) {
	if toolName == "Skill" {
		var input struct {
			Skill string `json:"skill"`
		}
		if err := json.Unmarshal(toolInput, &input); err == nil {
			switch input.Skill {
			case "jira-to-plan":
				return "PLANNING", "generating plan"
			case "start-branch":
				return "BRANCHING", "creating branch"
			case "augmented-coding":
				return "CODING", "implementing"
			case "review":
				return "REVIEWING", "reviewing"
			case "push-pr":
				return "PUSHING", "creating PR"
			}
		}
	}

	switch toolName {
	case "mcp__atlassian__read_jira_issue":
		return "PLANNING", "reading JIRA"
	}

	return "", ""
}
