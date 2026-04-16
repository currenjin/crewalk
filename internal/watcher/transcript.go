package watcher

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type PhaseEvent struct {
	TicketID  string
	Phase     string
	Status    string
	JSONLPath string
}

type QuestionEvent struct {
	TicketID string
	Text     string
	Options  []string
}

type jsonlEntry struct {
	Type    string        `json:"type"`
	Cwd     string        `json:"cwd"`
	Message *assistantMsg `json:"message,omitempty"`
	Result  string        `json:"result,omitempty"`
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
	fileTickets map[string]string // JSONL path → last known ticketID
	mu          sync.Mutex
	stop        chan struct{}
}

func New(projectsDir string) *Watcher {
	return &Watcher{
		projectsDir: projectsDir,
		events:      make(chan PhaseEvent, 100),
		questions:   make(chan QuestionEvent, 20),
		offsets:     make(map[string]int64),
		fileTickets: make(map[string]string),
		stop:        make(chan struct{}),
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

func (w *Watcher) Stop() {
	close(w.stop)
}

func (w *Watcher) poll() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer close(w.events)
	defer close(w.questions)

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.scanAll()
		}
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

	w.mu.Lock()
	offset := w.offsets[path]
	if info.Size() < offset {
		offset = 0
	}
	w.mu.Unlock()

	if info.Size() == offset {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		w.parseLine(path, scanner.Text())
	}

	w.mu.Lock()
	w.offsets[path] = info.Size()
	w.mu.Unlock()
}

func (w *Watcher) parseLine(path, line string) {
	var entry jsonlEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return
	}

	ticketID := extractTicketID(entry.Cwd)
	if ticketID != "" {
		w.mu.Lock()
		w.fileTickets[path] = ticketID
		w.mu.Unlock()
	} else {
		w.mu.Lock()
		ticketID = w.fileTickets[path]
		w.mu.Unlock()
	}
	if ticketID == "" {
		return
	}

	if entry.Type == "result" {
		select {
		case w.events <- PhaseEvent{TicketID: ticketID, Phase: "DONE", Status: "done", JSONLPath: path}:
		case <-w.stop:
		}
		return
	}

	if entry.Type != "assistant" || entry.Message == nil {
		return
	}

	for _, content := range entry.Message.Content {
		if content.Type != "tool_use" {
			continue
		}

		if content.Name == "AskUserQuestion" {
			text, options := extractQuestion(content.Input)
			select {
			case w.questions <- QuestionEvent{TicketID: ticketID, Text: text, Options: options}:
			case <-w.stop:
				return
			}
			continue
		}

		phase, status := detectPhase(content.Name, content.Input)
		if phase == "" {
			continue
		}
		select {
		case w.events <- PhaseEvent{TicketID: ticketID, Phase: phase, Status: status, JSONLPath: path}:
		case <-w.stop:
			return
		}
	}
}

func extractQuestion(raw json.RawMessage) (text string, options []string) {
	var input struct {
		Question  string `json:"question"`
		Prompt    string `json:"prompt"`
		Questions []struct {
			Question string `json:"question"`
			Options  []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return "?", nil
	}

	if len(input.Questions) > 0 {
		q := input.Questions[0]
		text = q.Question
		for _, opt := range q.Options {
			label := opt.Label
			if opt.Description != "" {
				label += "  " + opt.Description
			}
			options = append(options, label)
		}
		return text, options
	}

	if input.Question != "" {
		return input.Question, nil
	}
	return input.Prompt, nil
}

func extractTicketID(cwd string) string {
	base := filepath.Base(cwd)
	matches := ticketPattern.FindStringSubmatch(strings.ToUpper(base))
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

var sourceExts = map[string]bool{
	".kt": true, ".java": true, ".ts": true, ".tsx": true,
	".js": true, ".jsx": true, ".py": true, ".go": true,
	".rb": true, ".rs": true, ".swift": true, ".cs": true,
	".cpp": true, ".c": true, ".h": true,
}

func isSourceFile(p string) bool {
	ext := filepath.Ext(p)
	return sourceExts[ext]
}

func detectPhase(toolName string, toolInput json.RawMessage) (phase, status string) {
	switch toolName {
	case "Skill":
		var input struct {
			Skill string `json:"skill"`
		}
		if err := json.Unmarshal(toolInput, &input); err == nil {
			switch input.Skill {
			case "start-branch":
				return "BRANCHING", "creating branch"
			case "jira-to-plan":
				return "PLANNING", "generating plan"
			case "augmented-coding":
				return "CODING", "implementing"
			case "push-pr":
				return "PUSHING", "creating PR"
			}
		}

	case "Read":
		var input struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal(toolInput, &input); err == nil {
			p := input.FilePath
			switch {
			case strings.Contains(p, "start-branch/SKILL"):
				return "BRANCHING", "creating branch"
			case strings.Contains(p, "jira-to-plan/SKILL"):
				return "PLANNING", "generating plan"
			case strings.Contains(p, "augmented-coding/SKILL"):
				return "CODING", "implementing"
			case strings.Contains(p, "push-pr/SKILL"):
				return "PUSHING", "creating PR"
			}
		}

	case "Write", "Edit", "NotebookEdit":
		var input struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal(toolInput, &input); err == nil {
			p := input.FilePath
			if strings.Contains(p, "augmented-coding") && strings.HasSuffix(p, "-plan.md") {
				return "PLANNING", "saving plan"
			}
			if isSourceFile(p) {
				return "CODING", "implementing"
			}
		}

	case "Bash":
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(toolInput, &input); err == nil {
			cmd := input.Command
			switch {
			case (strings.Contains(cmd, "git checkout") && strings.Contains(cmd, "-b")) ||
				(strings.Contains(cmd, "git switch") && strings.Contains(cmd, "-c")):
				return "BRANCHING", "creating branch"
			case strings.Contains(cmd, "gradlew") || strings.Contains(cmd, "mvn") ||
				strings.Contains(cmd, "git commit"):
				return "CODING", "implementing"
			case strings.Contains(cmd, "git push") || strings.Contains(cmd, "gh pr"):
				return "PUSHING", "creating PR"
			}
		}

	case "mcp__atlassian__read_jira_issue",
		"mcp__claude_ai_Atlassian__getJiraIssue",
		"mcp__claude_ai_Atlassian__search":
		return "PLANNING", "reading JIRA"
	}

	return "", ""
}
