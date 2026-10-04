// Package makiusage aggregates token usage and prompt counts from the
// Maki CLI's session JSONL files under ~/.local/state/maki/sessions/.
// Local-only parse — no API calls. Powers the dashboard's per-agent
// Maki row so users see "tokens used in the last 5h" the same way the
// Claude panel does.
//
// Maki gets its own walker instead of riding the generic
// internal/agentusage one because its transcripts don't match either
// recognized usage shape: token totals live under a `token_usage` key
// inside `{"t":"meta"}` events, not a `usage` object or top-level
// token fields, so the generic walker silently yields HasData=false.
//
// File shape: one flat <id>.jsonl per session:
//
//   - line 1: {"t":"header","v":2,"id":"C…","model":"provider/model",
//     "cwd":"/path","created_at":<epoch seconds>}
//   - {"t":"msg","d":{"role":"user"|"assistant",
//     "content":[{"type":"text","text":"…"}]}} — user/assistant turns
//   - {"t":"meta","title":"…","token_usage":{"input_tokens":N,
//     "output_tokens":M,"cache_creation_input_tokens":C,
//     "cache_read_input_tokens":R},"updated_at":<epoch seconds>,
//     "mode":"build","usage_by_model":{…}} — one per state snapshot,
//     many per file
//
// Semantics: token_usage is a CUMULATIVE session total with
// Anthropic-style field names, so only the LAST meta event in a file
// contributes (summing every meta would multiply-count). token_usage
// can be null in old files — that contributes zero tokens, but the
// file's user turns still count. The optional per-model `usage_by_model`
// cost data is ignored: AgentSummary keeps only the fields that mean
// the same thing across every agent, and cache tokens are excluded for
// the same reason the generic walker reads only input/output.
package makiusage

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skzv/ccmux/internal/jsonl"
)

// Summary is the result of one Walk: token totals + user-turn count
// for the window. HasData is false when no usage or user turns were
// found. Shape-matched to agentusage.Summary so internal/usage renders
// it identically.
type Summary struct {
	HasData      bool
	Window       time.Duration
	Prompts      int
	InputTokens  int
	OutputTokens int
}

// tokenUsage is the cumulative per-session breakdown a meta event
// carries. Cache tokens are parsed but deliberately unused — the
// cross-agent summary reads only billed input/output.
type tokenUsage struct {
	Input  int `json:"input_tokens"`
	Output int `json:"output_tokens"`

	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
}

// content is one block of a msg event's content array.
type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// msgData is the `d` payload of a msg event.
type msgData struct {
	Role    string    `json:"role"`
	Content []content `json:"content"`
}

// line is the union of every event shape we care about. All optional —
// a line matching none contributes nothing.
type line struct {
	T          string      `json:"t"`
	D          *msgData    `json:"d"`
	TokenUsage *tokenUsage `json:"token_usage"`
	UpdatedAt  int64       `json:"updated_at"`
}

// cancelledMarker is the text maki puts on the synthetic user message
// it records when a turn is interrupted. It isn't a turn the user
// typed, so it doesn't count as a prompt.
const cancelledMarker = "[Cancelled by user]"

// Walk scans every *.jsonl directly under root (the flat sessions dir;
// the archive/ subdirectory holds user-deleted sessions and is
// skipped) and aggregates usage for sessions whose LAST meta
// updated_at falls inside the window, falling back to the file mtime
// for files with no meta events. The gate is per-file because
// token_usage is cumulative — a session's final totals belong to the
// moment it was last active, not to each historical meta event.
//
// Returns HasData=false (not an error) when root is missing or nothing
// matched — callers render that as the placeholder row.
func Walk(root string, window time.Duration) (Summary, error) {
	cutoff := time.Now().Add(-window)
	sum := Summary{Window: window}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A file/dir removed while we walk (maki rotates
			// sessions) shouldn't sink the whole walk.
			if os.IsNotExist(err) || os.IsPermission(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			// archive/ holds user-deleted sessions; skip it
			// (and everything under it) rather than counting
			// sessions the user explicitly deleted.
			if d.Name() == "archive" && filepath.Dir(path) == root {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		scanFile(path, d, cutoff, &sum)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return Summary{Window: window}, nil
		}
		return Summary{Window: window}, err
	}
	sum.HasData = sum.Prompts > 0 || sum.InputTokens > 0 || sum.OutputTokens > 0
	return sum, nil
}

// scanFile reads one session file and accumulates its final totals
// into sum. Unparseable lines are skipped silently — transcripts may
// interleave partial lines, and old files carry shapes we don't know.
func scanFile(path string, d fs.DirEntry, cutoff time.Time, sum *Summary) {
	// Cheap pre-filter by mtime, same as the generic walker: a file
	// last written before the window can't have an in-window final
	// meta. (Refined below by the last meta's updated_at.)
	if info, err := d.Info(); err == nil && info.ModTime().Before(cutoff) {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	// Transcript lines can be long (a full assistant message); allow
	// up to 8 MiB, and skip — rather than stop at — anything longer.
	sc := jsonl.NewScanner(f, 8*1024*1024)
	prompts := 0
	var lastMeta *tokenUsage
	lastUpdated := int64(0)
	for sc.Scan() {
		text := sc.Bytes()
		if len(text) == 0 || text[0] != '{' {
			continue
		}
		var l line
		if err := json.Unmarshal(text, &l); err != nil {
			continue
		}
		switch l.T {
		case "msg":
			if isUserPrompt(l.D) {
				prompts++
			}
		case "meta":
			// token_usage is cumulative: the last meta carries
			// the final totals. A null token_usage (old files)
			// leaves lastMeta pointing at the previous meta, but
			// updated_at still advances — it's the fresher
			// activity timestamp either way.
			if l.TokenUsage != nil {
				tu := *l.TokenUsage
				lastMeta = &tu
			}
			if l.UpdatedAt > lastUpdated {
				lastUpdated = l.UpdatedAt
			}
		}
	}
	// Window gate: the file's last meta updated_at (epoch seconds)
	// is when the session was last active; fall back to the mtime —
	// already checked above — for files with no meta events.
	if lastUpdated > 0 {
		when := time.Unix(lastUpdated, 0)
		if when.Before(cutoff) {
			return
		}
	}
	var in, out int
	if lastMeta != nil {
		in, out = lastMeta.Input, lastMeta.Output
	}
	if prompts == 0 && in == 0 && out == 0 {
		return
	}
	sum.Prompts += prompts
	sum.InputTokens += in
	sum.OutputTokens += out
}

// isUserPrompt reports whether a msg event is a real user-initiated
// turn: role "user" with non-empty text content, excluding maki's
// "[Cancelled by user]" interruption marker.
func isUserPrompt(d *msgData) bool {
	if d == nil || !strings.EqualFold(strings.TrimSpace(d.Role), "user") {
		return false
	}
	for _, c := range d.Content {
		if c.Type != "text" {
			continue
		}
		t := strings.TrimSpace(c.Text)
		if t == "" || t == cancelledMarker {
			continue
		}
		return true
	}
	return false
}
