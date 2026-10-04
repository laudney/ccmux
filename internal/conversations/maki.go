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

// ListMaki walks ~/.local/state/maki/sessions/<id>.jsonl (maki is the
// Rust terminal coding agent, tontinton/maki). The sessions directory
// is flat — one file per conversation — and holds an `archive/`
// SUBDIRECTORY of user-deleted sessions; deleted conversations must
// not resurface, so we read only the top level and never descend.
//
// The conversation ID comes from the header line's `id` (falling back
// to the filename stem), and the project label is the header `cwd` —
// the same convention as pi's session header (see ListPi).
//
// Launch mode: maki transcripts carry NO headless marker (SessionMeta
// only has mode build/plan, and `maki -p` headless writes the same
// shape), so Entrypoint stays "" and IsHeadless reports false for
// every maki row — the Antigravity precedent.
func ListMaki(home string) ([]Conversation, error) {
	root := filepath.Join(home, ".local", "state", "maki", "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", root, err)
	}
	var out []Conversation
	for _, e := range entries {
		// e.IsDir() skips archive/ (deleted sessions) and anything
		// else maki parks in this directory; we only read flat files.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(root, e.Name())
		c := readMakiTranscript(path)
		if c.ID != "" {
			out = append(out, c)
		}
	}
	return out, nil
}

// makiEvent models one line of a maki session file. maki tags every
// line with a top-level `t` ("header" once per file, "msg" for turns,
// "meta" for session metadata, plus "out"/"frame" shapes we ignore).
// The header carries `id` + `cwd`; the meta events carry `updated_at`
// (epoch seconds) and the LAST one is authoritative.
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
	DisplayText string          `json:"display_text"`
}

// content returns the human-readable text of a maki message. Content
// is an array of typed blocks (text blocks carry "text");
// messagePartsContent already handles that shape (and a bare string,
// for older sessions), so we delegate.
func (m *makiMessage) content() string {
	if m == nil || len(m.Content) == 0 {
		return ""
	}
	return messagePartsContent(m.Content)
}

// makiVisibleBody is visibleTurn for a maki msg event.
func makiVisibleBody(ev makiEvent) (role, body string, ok bool) {
	if ev.Type != "msg" || ev.Data == nil {
		return "", "", false
	}
	role = ev.Data.Role
	body, ok = visibleTurn(role, ev.Data.content())
	return role, body, ok
}

// readMakiTranscript reads one maki session file end-to-end, returning
// a Conversation with ID, Project, Preview, and LastActivity populated.
func readMakiTranscript(path string) Conversation {
	c := Conversation{Agent: agent.IDMaki, Path: path}
	if info, err := os.Stat(path); err == nil {
		c.LastActivity = info.ModTime()
	}
	// Fallback ID from the filename: <id>.jsonl → id.
	c.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")

	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	var latestEvent time.Time
	sc := jsonl.NewScanner(f, 4*1024*1024)
	for sc.Scan() {
		var ev makiEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "header":
			// Header line: authoritative id + cwd.
			if ev.ID != "" {
				c.ID = ev.ID
			}
			if ev.CWD != "" {
				c.Project = ev.CWD
			}
		case "msg":
			if ev.Data != nil && ev.Data.Role == "user" && c.Preview == "" {
				c.Preview = makiPreviewText(ev.Data)
			}
		case "meta":
			// One or many meta events per file; the LAST one has the
			// final updated_at, but any later one wins here since
			// they are written in order.
			if ev.UpdatedAt > 0 {
				if ts := time.Unix(ev.UpdatedAt, 0).UTC(); ts.After(latestEvent) {
					latestEvent = ts
				}
			}
		}
	}
	if !latestEvent.IsZero() {
		c.LastActivity = latestEvent
	}
	return c
}

// makiPreviewText picks the text a maki user message shows as the
// conversation preview, and "" for turns that shouldn't: the
// "[Cancelled by user]" interrupt marker (some of which also carry an
// empty display_text), and messages that are only a slash command
// like "/resume" — those are control input, not a prompt. Everything
// else is trimmed and flattened to one line, truncated to ~100 runes,
// like the other agents' preview rules.
func makiPreviewText(m *makiMessage) string {
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
