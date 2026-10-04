package makiusage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// writeSession writes one .jsonl session file, one line per element.
func writeSession(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf []byte
	for _, l := range lines {
		buf = append(buf, l+"\n"...)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWalk — table-driven coverage of the walker against fixture
// sessions written to a temp root: cumulative token_usage across meta
// events, null token_usage, prompt filtering, the archive/ skip, and
// the window gate on last-meta updated_at (with an mtime fallback).
func TestWalk(t *testing.T) {
	now := time.Now()
	inWindow := now.Add(-1 * time.Hour).Unix()
	stale := now.Add(-6 * time.Hour).Unix()
	window := 5 * time.Hour

	header := `{"t":"header","v":2,"id":"Cabc","model":"prov/model","cwd":"/p","created_at":` + itoa(inWindow-100) + `}`
	userMsg := `{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"fix the bug"}]}}`
	cancelledMsg := `{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"[Cancelled by user]"}]}}`
	emptyMsg := `{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"  "}]}}`
	assistantMsg := `{"t":"msg","d":{"role":"assistant","content":[{"type":"text","text":"on it"}]}}`

	tests := []struct {
		name  string
		files map[string][]string // path under root → lines
		// touchOld marks files whose mtime should be pushed before
		// the window (tests the mtime fallback).
		touchOld map[string]bool
		want     Summary
	}{
		{
			name: "basic session counts last meta and user prompts",
			files: map[string][]string{
				"C1.jsonl": {
					header,
					userMsg,
					assistantMsg,
					`{"t":"meta","title":"t","token_usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":9,"cache_read_input_tokens":7},"updated_at":` + itoa(inWindow) + `,"mode":"build"}`,
				},
			},
			want: Summary{HasData: true, Window: window, Prompts: 1, InputTokens: 100, OutputTokens: 50},
		},
		{
			name: "cumulative token_usage counts only the last meta",
			files: map[string][]string{
				"C1.jsonl": {
					header,
					userMsg,
					`{"t":"meta","token_usage":{"input_tokens":10,"output_tokens":5},"updated_at":` + itoa(inWindow-50) + `}`,
					assistantMsg,
					`{"t":"meta","token_usage":{"input_tokens":100,"output_tokens":50},"updated_at":` + itoa(inWindow) + `}`,
				},
			},
			want: Summary{HasData: true, Window: window, Prompts: 1, InputTokens: 100, OutputTokens: 50},
		},
		{
			name: "null token_usage still counts prompts",
			files: map[string][]string{
				"C1.jsonl": {
					header,
					userMsg,
					`{"t":"meta","token_usage":null,"updated_at":` + itoa(inWindow) + `}`,
				},
			},
			want: Summary{HasData: true, Window: window, Prompts: 1},
		},
		{
			name: "zero tokens with prompts is still HasData",
			files: map[string][]string{
				"C1.jsonl": {
					header,
					userMsg,
					`{"t":"meta","token_usage":{"input_tokens":0,"output_tokens":0},"updated_at":` + itoa(inWindow) + `}`,
				},
			},
			want: Summary{HasData: true, Window: window, Prompts: 1},
		},
		{
			name: "cancelled and empty user msgs are not prompts",
			files: map[string][]string{
				"C1.jsonl": {
					header,
					cancelledMsg,
					emptyMsg,
					`{"t":"meta","token_usage":{"input_tokens":3,"output_tokens":2},"updated_at":` + itoa(inWindow) + `}`,
				},
			},
			want: Summary{HasData: true, Window: window, InputTokens: 3, OutputTokens: 2},
		},
		{
			name: "multiple sessions aggregate",
			files: map[string][]string{
				"C1.jsonl": {
					header, userMsg,
					`{"t":"meta","token_usage":{"input_tokens":10,"output_tokens":20},"updated_at":` + itoa(inWindow) + `}`,
				},
				"C2.jsonl": {
					header, userMsg, userMsg,
					`{"t":"meta","token_usage":{"input_tokens":1,"output_tokens":2},"updated_at":` + itoa(inWindow) + `}`,
				},
			},
			want: Summary{HasData: true, Window: window, Prompts: 3, InputTokens: 11, OutputTokens: 22},
		},
		{
			name: "archive subdir is skipped",
			files: map[string][]string{
				"archive/C1.jsonl": {
					header, userMsg,
					`{"t":"meta","token_usage":{"input_tokens":999,"output_tokens":999},"updated_at":` + itoa(inWindow) + `}`,
				},
			},
			want: Summary{Window: window},
		},
		{
			name: "non-jsonl files are ignored",
			files: map[string][]string{
				"C1.json": {userMsg},
			},
			want: Summary{Window: window},
		},
		{
			name: "session with stale last meta is excluded",
			files: map[string][]string{
				"old.jsonl": {
					header, userMsg,
					`{"t":"meta","token_usage":{"input_tokens":500,"output_tokens":500},"updated_at":` + itoa(stale) + `}`,
				},
			},
			want: Summary{Window: window},
		},
		{
			name: "file with no meta falls back to mtime",
			files: map[string][]string{
				"nometa.jsonl": {header, userMsg},
			},
			touchOld: map[string]bool{"nometa.jsonl": true},
			want:     Summary{Window: window},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, lines := range tt.files {
				writeSession(t, filepath.Join(root, rel), lines...)
			}
			for rel := range tt.touchOld {
				path := filepath.Join(root, rel)
				old := now.Add(-6 * time.Hour)
				if err := os.Chtimes(path, old, old); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Walk(root, window)
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			if got != tt.want {
				t.Errorf("Walk = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestWalk_EmptyAndMissingRoots — an empty sessions dir and a missing
// root both return HasData=false with no error (the dashboard renders
// the install-hint placeholder, same contract as agentusage).
func TestWalk_EmptyAndMissingRoots(t *testing.T) {
	for _, tc := range []struct {
		name string
		root string // "" → create-and-use an empty dir
	}{
		{name: "empty dir"},
		{name: "missing root", root: filepath.Join(t.TempDir(), "does", "not", "exist")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.root
			if root == "" {
				root = t.TempDir()
			}
			got, err := Walk(root, 5*time.Hour)
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			want := Summary{Window: 5 * time.Hour}
			if got != want {
				t.Errorf("Walk = %+v, want %+v", got, want)
			}
		})
	}
}

// itoa keeps fixture lines readable inline.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
