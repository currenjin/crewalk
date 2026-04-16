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
	offsets     map[string]int64
	mu          sync.Mutex
	stop        chan struct{}
}

func New(projectsDir string) *Watcher {
	return &Watcher{
		projectsDir: projectsDir,
		events:      make(chan PhaseEvent, 100),
		offsets:     make(map[string]int64),
		stop:        make(chan struct{}),
	}
}

func (w *Watcher) Events() <-chan PhaseEvent {
	return w.events
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
	}

	switch toolName {
	case "mcp__atlassian__read_jira_issue":
		return "PLANNING", "reading JIRA"
	}

	return "", ""
}
