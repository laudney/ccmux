package conversations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skzv/ccmux/internal/agent"
	"github.com/skzv/ccmux/internal/jsonl"
	"github.com/skzv/ccmux/internal/termsafe"
)

// ListMaki reads the native flat session store. archive/ contains old
// snapshots, not additional conversations. JSONL takes precedence over
// legacy JSON, as it does in Maki's loader.
func ListMaki(home string) ([]Conversation, error) {
	root := agent.ByID(agent.IDMaki).TranscriptsRoot(home)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", root, err)
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names[e.Name()] = true
		}
	}
	var out []Conversation
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".jsonl" && ext != ".json") {
			continue
		}
		if ext == ".json" && names[strings.TrimSuffix(e.Name(), ext)+".jsonl"] {
			continue
		}
		c := readMakiTranscript(filepath.Join(root, e.Name()))
		if c.ID != "" {
			out = append(out, c)
		}
	}
	return out, nil
}

type makiEvent struct {
	Type      string       `json:"t"`
	ID        string       `json:"id"`
	CWD       string       `json:"cwd"`
	UpdatedAt int64        `json:"updated_at"`
	Data      *makiMessage `json:"d"`
}

type makiMessage struct {
	Role        string          `json:"role"`
	Content     json.RawMessage `json:"content"`
	DisplayText *string         `json:"display_text"`
	Kind        json.RawMessage `json:"kind"`
}

// content follows Maki's display_text contract: an empty override hides
// synthetic API messages, and a non-empty one replaces their API text.
func (m *makiMessage) content() string {
	if m == nil || string(m.Kind) == `"observation"` {
		return ""
	}
	if m.DisplayText != nil {
		return *m.DisplayText
	}
	return messagePartsContent(m.Content)
}

func makiVisibleBody(ev makiEvent) (role, body string, ok bool) {
	if ev.Type != "msg" || ev.Data == nil {
		return "", "", false
	}
	role = ev.Data.Role
	body, ok = visibleTurn(role, ev.Data.content())
	return role, body, ok
}

type makiLegacySession struct {
	Version   int           `json:"version"`
	ID        string        `json:"id"`
	CWD       string        `json:"cwd"`
	UpdatedAt int64         `json:"updated_at"`
	Messages  []makiMessage `json:"messages"`
}

// scanMaki visits records without retaining tool outputs or all messages.
func scanMaki(path string, visit func(makiEvent)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if filepath.Ext(path) == ".json" {
		var s makiLegacySession
		if err := json.NewDecoder(f).Decode(&s); err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if s.Version != 1 || s.ID == "" {
			return fmt.Errorf("invalid Maki session %s", path)
		}
		visit(makiEvent{Type: "header", ID: s.ID, CWD: s.CWD})
		for i := range s.Messages {
			visit(makiEvent{Type: "msg", Data: &s.Messages[i]})
		}
		visit(makiEvent{Type: "meta", UpdatedAt: s.UpdatedAt})
		return nil
	}
	sc := jsonl.NewScanner(f, 4*1024*1024)
	for sc.Scan() {
		var ev makiEvent
		if json.Unmarshal(sc.Bytes(), &ev) == nil {
			visit(ev)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func readMakiTranscript(path string) Conversation {
	c := Conversation{Agent: agent.IDMaki, Path: path}
	if info, err := os.Stat(path); err == nil {
		c.LastActivity = info.ModTime()
	}
	err := scanMaki(path, func(ev makiEvent) {
		switch ev.Type {
		case "header":
			c.ID, c.Project = ev.ID, ev.CWD
		case "msg":
			if ev.Data != nil && ev.Data.Role == "user" && c.Preview == "" {
				c.Preview = makiPreviewText(ev.Data)
			}
		case "meta":
			if ev.UpdatedAt > 0 {
				c.LastActivity = time.Unix(ev.UpdatedAt, 0).UTC()
			}
		}
	})
	if err != nil {
		return Conversation{}
	}
	return c
}

func makiPreviewText(m *makiMessage) string {
	// Host observations and context updates are not user prompts.
	if len(m.Kind) != 0 && string(m.Kind) != `"turn"` {
		return ""
	}
	text := strings.Join(strings.Fields(termsafe.String(m.content())), " ")
	if text == "" || text == "[Cancelled by user]" {
		return ""
	}
	if fields := strings.Fields(text); len(fields) == 1 && strings.HasPrefix(fields[0], "/") {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 100 {
		return string(runes[:100]) + "…"
	}
	return text
}

func readMakiMessages(path string, limit int) ([]Message, error) {
	var all []Message
	err := scanMaki(path, func(ev makiEvent) {
		if role, body, ok := makiVisibleBody(ev); ok {
			all = append(all, Message{Role: role, Content: body})
		}
	})
	if err != nil {
		return nil, err
	}
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}

func countMakiMessages(path string) (int, error) {
	n := 0
	err := scanMaki(path, func(ev makiEvent) {
		if _, _, ok := makiVisibleBody(ev); ok {
			n++
		}
	})
	return n, err
}
