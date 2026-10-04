package usage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/skzv/ccmux/internal/agent"
)

// writeMakiSession writes one maki session .jsonl under
// $HOME/.local/state/maki/sessions/ with a user prompt and a final
// cumulative meta token_usage.
func writeMakiSession(t *testing.T, home, name string, updatedAt, in, out int64) {
	t.Helper()
	path := filepath.Join((agent.Maki{}).TranscriptsRoot(home), name+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"t":"header","v":2,"id":"C1","model":"prov/model","cwd":"/p","created_at":` + itoa(updatedAt-100) + `}` + "\n" +
		`{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"hello"}]}}` + "\n" +
		`{"t":"meta","token_usage":{"input_tokens":` + itoa(in) + `,"output_tokens":` + itoa(out) + `},"updated_at":` + itoa(updatedAt) + "}"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// TestWalkOthers_IncludesMakiRow — a maki transcript in the fake home
// must surface as a "maki" row in WalkOthers. Maki is deliberately not
// in genericWalkAgents (its token_usage key doesn't match the generic
// walker's shapes), so this pins the bespoke-walker wiring.
func TestWalkOthers_IncludesMakiRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("APPDATA", "")
	writeMakiSession(t, home, "C1", time.Now().Add(-time.Hour).Unix(), 100, 50)

	rows := WalkOthers(5 * time.Hour)
	for _, r := range rows {
		if r.Agent != "maki" {
			continue
		}
		s := r.Summary
		if !s.HasData {
			t.Fatal("maki row has HasData=false")
		}
		if s.Prompts != 1 || s.InputTokens != 100 || s.OutputTokens != 50 {
			t.Errorf("maki summary = %+v, want prompts=1 in=100 out=50", s)
		}
		return
	}
	t.Fatalf("WalkOthers returned no maki row: %+v", rows)
}

// TestWalkOthers_MakiStaleSessionOmitted — a session whose last meta
// predates the window must not produce a maki row (WalkOthers only
// returns rows for agents that actually have usage in the window).
func TestWalkOthers_MakiStaleSessionOmitted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("APPDATA", "")
	writeMakiSession(t, home, "C1", time.Now().Add(-24*time.Hour).Unix(), 100, 50)

	for _, r := range WalkOthers(5 * time.Hour) {
		if r.Agent == "maki" {
			t.Errorf("stale maki session surfaced a row: %+v", r)
		}
	}
}
