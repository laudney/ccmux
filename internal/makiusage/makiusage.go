// Package makiusage reads cumulative Maki usage snapshots. Deltas between
// snapshots prevent a resumed session from billing its full history again.
package makiusage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skzv/ccmux/internal/jsonl"
)

type Summary struct {
	HasData      bool
	Window       time.Duration
	Prompts      int
	InputTokens  int
	OutputTokens int
}

type tokenUsage struct {
	Input  int `json:"input_tokens"`
	Output int `json:"output_tokens"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type msgData struct {
	Role        string          `json:"role"`
	Content     []content       `json:"content"`
	DisplayText *string         `json:"display_text"`
	Kind        json.RawMessage `json:"kind"`
}

type line struct {
	T          string      `json:"t"`
	ID         string      `json:"id"`
	CreatedAt  int64       `json:"created_at"`
	D          *msgData    `json:"d"`
	TokenUsage *tokenUsage `json:"token_usage"`
	UpdatedAt  int64       `json:"updated_at"`
}

// Walk visits only native top-level sessions. Archived snapshots, lock
// directories and duplicate legacy JSON files never contribute usage.
func Walk(root string, window time.Duration) (Summary, error) {
	sum := Summary{Window: window}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return sum, nil
	}
	if err != nil {
		return sum, err
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names[e.Name()] = true
		}
	}
	cutoff := time.Now().Add(-window)
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".jsonl" && ext != ".json") {
			continue
		}
		if ext == ".json" && names[strings.TrimSuffix(e.Name(), ext)+".jsonl"] {
			continue
		}
		scanFile(filepath.Join(root, e.Name()), cutoff, &sum)
	}
	sum.HasData = sum.Prompts > 0 || sum.InputTokens > 0 || sum.OutputTokens > 0
	return sum, nil
}

func scanFile(path string, cutoff time.Time, sum *Summary) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.ModTime().Before(cutoff) {
		return
	}
	if filepath.Ext(path) == ".json" {
		// A legacy file has one snapshot. Its totals can only be placed
		// in the window when the session itself was created in the window.
		var s struct {
			Version    int         `json:"version"`
			ID         string      `json:"id"`
			CreatedAt  int64       `json:"created_at"`
			UpdatedAt  int64       `json:"updated_at"`
			TokenUsage *tokenUsage `json:"token_usage"`
			Messages   []msgData   `json:"messages"`
		}
		if json.NewDecoder(f).Decode(&s) != nil || s.Version != 1 || s.ID == "" ||
			time.Unix(s.CreatedAt, 0).Before(cutoff) || time.Unix(s.UpdatedAt, 0).Before(cutoff) {
			return
		}
		if s.TokenUsage != nil {
			sum.InputTokens += max(s.TokenUsage.Input, 0)
			sum.OutputTokens += max(s.TokenUsage.Output, 0)
		}
		for i := range s.Messages {
			if isUserPrompt(&s.Messages[i]) {
				sum.Prompts++
			}
		}
		return
	}

	var previous tokenUsage
	havePrevious := false
	header, newSession := false, false
	pending := 0
	scanned := Summary{}
	sc := jsonl.NewScanner(f, 8*1024*1024)
	for sc.Scan() {
		var l line
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		switch l.T {
		case "header":
			header = l.ID != ""
			newSession = l.CreatedAt > 0 && !time.Unix(l.CreatedAt, 0).Before(cutoff)
		case "msg":
			if isUserPrompt(l.D) {
				pending++
			}
		case "meta":
			inWindow := l.UpdatedAt > 0 && !time.Unix(l.UpdatedAt, 0).Before(cutoff)
			// A rewrite keeps only the final snapshot. For an old
			// session its first surviving totals have no known time;
			// use them as a baseline instead of billing all old turns.
			if inWindow && (havePrevious || newSession) {
				scanned.Prompts += pending
				if l.TokenUsage != nil {
					scanned.InputTokens += max(l.TokenUsage.Input-previous.Input, 0)
					scanned.OutputTokens += max(l.TokenUsage.Output-previous.Output, 0)
				}
			}
			pending = 0
			if l.TokenUsage != nil {
				previous, havePrevious = *l.TokenUsage, true
			}
		}
	}
	if !header || sc.Err() != nil {
		return
	}
	// A new file can be captured while the first prompt is still running.
	if newSession {
		scanned.Prompts += pending
	}
	sum.Prompts += scanned.Prompts
	sum.InputTokens += scanned.InputTokens
	sum.OutputTokens += scanned.OutputTokens
}

func isUserPrompt(d *msgData) bool {
	if d == nil || d.Role != "user" || (len(d.Kind) != 0 && string(d.Kind) != `"turn"`) {
		return false
	}
	if d.DisplayText != nil {
		return realPromptText(*d.DisplayText)
	}
	for _, c := range d.Content {
		if c.Type == "text" && realPromptText(c.Text) {
			return true
		}
	}
	return false
}

func realPromptText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || text == "[Cancelled by user]" {
		return false
	}
	fields := strings.Fields(text)
	return len(fields) != 1 || !strings.HasPrefix(fields[0], "/")
}
