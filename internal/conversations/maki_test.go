package conversations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/skzv/ccmux/internal/agent"
)

// makiSession is a real-shaped maki transcript: header, a slash-command
// user turn, a cancelled user turn, a real prompt, an assistant reply,
// two meta events (the second is the final updated_at), and an
// "out" event we must ignore. Also includes an unparseable line.
const makiSession = `{"t":"header","v":2,"id":"CfgxdrpL5JtPGPbHvDtTm","model":"openai/gpt-5","cwd":"/Users/skz/Projects/demo","created_at":1791106090}
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"/resume"}]}}
not json at all
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"[Cancelled by user]"}],"display_text":""}}
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"fix  the\n\tflaky test"}]}}
{"t":"msg","d":{"role":"assistant","content":[{"type":"text","text":"On it."}]}}
{"t":"meta","title":"fix the flaky test","token_usage":{"input_tokens":10},"updated_at":1791106299,"mode":"build"}
{"t":"out","v":1,"data":"noise"}
{"t":"meta","title":"fix the flaky test","token_usage":{"input_tokens":20},"updated_at":1791106400,"mode":"build"}
`

func TestListMaki_HappyPath(t *testing.T) {
	home := makiHome(t)
	writeFile(t, filepath.Join((agent.Maki{}).TranscriptsRoot(home), "CfgxdrpL5JtPGPbHvDtTm.jsonl"), makiSession)

	got, err := ListMaki(home)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListMaki = %v, %v", got, err)
	}
	c := got[0]
	if c.ID != "CfgxdrpL5JtPGPbHvDtTm" {
		t.Errorf("ID = %q", c.ID)
	}
	if c.Agent != agent.IDMaki {
		t.Errorf("Agent = %q", c.Agent)
	}
	if c.Project != "/Users/skz/Projects/demo" {
		t.Errorf("Project = %q", c.Project)
	}
	if c.Preview != "fix the flaky test" {
		t.Errorf("Preview = %q, want the first real user prompt", c.Preview)
	}
	if c.Entrypoint != "" || c.IsHeadless() {
		t.Errorf("maki must never be headless: entrypoint %q, headless %v", c.Entrypoint, c.IsHeadless())
	}
	if c.Path == "" {
		t.Error("Path is empty")
	}
	if want := time.Unix(1791106400, 0).UTC(); !c.LastActivity.Equal(want) {
		t.Errorf("LastActivity = %v, want the last meta updated_at %v", c.LastActivity, want)
	}
}

func TestListMaki_HeaderIDOverridesFilename(t *testing.T) {
	home := makiHome(t)
	// Header id deliberately differs from the filename stem.
	writeFile(t, filepath.Join((agent.Maki{}).TranscriptsRoot(home), "otherid.jsonl"),
		`{"t":"header","v":2,"id":"CfgxdrpL5JtPGPbHvDtTm","cwd":"/p","created_at":1}
{"t":"meta","updated_at":100}
`)
	got, err := ListMaki(home)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListMaki = %v, %v", got, err)
	}
	if got[0].ID != "CfgxdrpL5JtPGPbHvDtTm" {
		t.Errorf("ID = %q, want the header id", got[0].ID)
	}
}

func TestListMaki_ArchiveSkipped(t *testing.T) {
	home := makiHome(t)
	root := (agent.Maki{}).TranscriptsRoot(home)
	writeFile(t, filepath.Join(root, "liveid.jsonl"), makiSession)
	writeFile(t, filepath.Join(root, "archive", "deadid.jsonl"), makiSession)

	got, err := ListMaki(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "CfgxdrpL5JtPGPbHvDtTm" {
		t.Fatalf("archived sessions resurfaced: %+v", got)
	}
}

func TestListMaki_EmptyDirAndMissingRoot(t *testing.T) {
	empty := makiHome(t)
	if got, err := ListMaki(filepath.Join(empty, "nowhere")); err != nil || got != nil {
		t.Errorf("missing root: got %v, %v; want nil, nil", got, err)
	}
	if got, err := ListMaki(empty); err != nil || got != nil {
		t.Errorf("empty root: got %v, %v; want nil, nil", got, err)
	}
}

func TestListMaki_UnparseableFileTolerated(t *testing.T) {
	home := makiHome(t)
	root := (agent.Maki{}).TranscriptsRoot(home)
	// A file without a header cannot be resumed by Maki.
	writeFile(t, filepath.Join(root, "bareid.jsonl"), "garbage\nlines\n")

	got, err := ListMaki(home)
	if err != nil || len(got) != 0 {
		t.Fatalf("invalid transcript listed: %v, %v", got, err)
	}
}

func TestRecentMessages_Maki(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CfgxdrpL5JtPGPbHvDtTm.jsonl")
	writeFile(t, path, makiSession)
	c := Conversation{Agent: agent.IDMaki, ID: "CfgxdrpL5JtPGPbHvDtTm", Path: path}

	msgs, err := RecentMessages(c, 50)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, m := range msgs {
		got = append(got, m.Role+": "+m.Content)
	}
	want := []string{
		"user: /resume",
		"user: fix  the\n\tflaky test",
		"assistant: On it.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RecentMessages = %v, want %v", got, want)
	}

	if n, err := CountMessages(c); err != nil || n != 3 {
		t.Fatalf("CountMessages = %d, %v; want 3", n, err)
	}

	// Limit keeps only the most recent tail.
	msgs, err = RecentMessages(c, 1)
	if err != nil || len(msgs) != 1 || msgs[0].Role != "assistant" {
		t.Fatalf("tail = %v, %v", msgs, err)
	}
}

// Note: agent.Commands has no per-agent override field for maki yet,
// so ResumeArgs always uses the default "maki" binary.
func TestResumeArgs_Maki(t *testing.T) {
	c := Conversation{Agent: agent.IDMaki, ID: "CfgxdrpL5JtPGPbHvDtTm"}
	got := c.ResumeArgsWithCommands(agent.Commands{})
	want := []string{"maki", "--resume", "CfgxdrpL5JtPGPbHvDtTm"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResumeArgsWithCommands = %v, want %v", got, want)
	}
}

func TestGuardTranscriptPath_Maki(t *testing.T) {
	home := makiHome(t)
	root := (agent.Maki{}).TranscriptsRoot(home)
	// The guard resolves symlinks on the parent dir, so it must exist.
	writeFile(t, filepath.Join(root, "someid.jsonl"), makiSession)
	writeFile(t, filepath.Join(root, "archive", "deadid.jsonl"), makiSession)
	if err := guardTranscriptPath(home, agent.IDMaki, filepath.Join(root, "someid.jsonl")); err != nil {
		t.Errorf("legit maki transcript refused: %v", err)
	}
	// The guard only checks root containment + extension (same as
	// Claude's subagents/ nesting); keeping archive/ out of the list
	// is ListMaki's job, covered by TestListMaki_ArchiveSkipped.
	if err := guardTranscriptPath(home, agent.IDMaki, filepath.Join(root, "archive", "deadid.jsonl")); err != nil {
		t.Errorf("contained maki transcript refused: %v", err)
	}
	if err := guardTranscriptPath(home, agent.IDMaki, filepath.Join(home, "etc/passwd")); err == nil {
		t.Error("non-transcript path accepted for delete")
	}
}

func makiHome(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("APPDATA", "")
	return t.TempDir()
}

func TestMakiDisplayTextAndHostMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	writeFile(t, path, `{"t":"header","id":"Cabc","cwd":"/p"}
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"API-only compaction prompt"}],"display_text":""}}
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"untrusted host observation"}],"kind":"observation"}}
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"full changed system facts"}],"kind":{"context_update":{}},"display_text":"Model changed"}}
{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"expanded slash command"}],"display_text":"Fix the bug"}}
{"t":"msg","d":{"role":"assistant","content":[{"type":"text","text":"Done\u001b]52;c;c2VjcmV0\u0007"}]}}
`)
	c := readMakiTranscript(path)
	if c.Preview != "Fix the bug" {
		t.Fatalf("Preview = %q", c.Preview)
	}
	msgs, err := RecentMessages(c, 20)
	if err != nil {
		t.Fatal(err)
	}
	want := []Message{{Role: "user", Content: "Model changed"}, {Role: "user", Content: "Fix the bug"}, {Role: "assistant", Content: "Done"}}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("Messages = %+v, want %+v", msgs, want)
	}
	if n, err := CountMessages(c); err != nil || n != len(want) {
		t.Fatalf("CountMessages = %d, %v", n, err)
	}
}

func TestListMakiLegacyAndXDG(t *testing.T) {
	home := makiHome(t)
	root := filepath.Join(t.TempDir(), "maki", "sessions")
	t.Setenv("XDG_STATE_HOME", filepath.Dir(filepath.Dir(root)))
	t.Setenv("APPDATA", filepath.Dir(filepath.Dir(root)))
	writeFile(t, filepath.Join(root, "Clegacy.json"), `{"version":1,"id":"Clegacy","cwd":"/old","created_at":1,"updated_at":10,"messages":[{"role":"user","content":[{"type":"text","text":"legacy prompt"}]}]}`)
	writeFile(t, filepath.Join(root, "scan_cache.json"), `{"entries":[]}`)
	got, err := ListMaki(home)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListMaki = %+v, %v", got, err)
	}
	c := got[0]
	if c.ID != "Clegacy" || c.Preview != "legacy prompt" || c.Project != "/old" {
		t.Fatalf("legacy = %+v", c)
	}
	if n, err := CountMessages(c); err != nil || n != 1 {
		t.Fatalf("CountMessages = %d, %v", n, err)
	}
	if err := guardTranscriptPath(home, agent.IDMaki, c.Path); err != nil {
		t.Fatal(err)
	}
	msgs, err := RecentMessages(c, 1)
	if err != nil || len(msgs) != 1 || msgs[0].Content != "legacy prompt" {
		t.Fatalf("legacy messages = %+v, %v", msgs, err)
	}
	writeFile(t, filepath.Join(root, "Clegacy.jsonl"), makiSession)
	got, err = ListMaki(home)
	if err != nil || len(got) != 1 || filepath.Ext(got[0].Path) != ".jsonl" {
		t.Fatalf("duplicate legacy session = %+v, %v", got, err)
	}
}

func TestMakiDeleteWithXDGRoot(t *testing.T) {
	home := makiHome(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("APPDATA", os.Getenv("XDG_STATE_HOME"))
	root := agent.ByID(agent.IDMaki).TranscriptsRoot(home)
	path := filepath.Join(root, "Cabc.jsonl")
	writeFile(t, path, makiSession)
	c := readMakiTranscript(path)
	if err := Delete(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deleted path stat = %v", err)
	}
}

func TestMakiSkipsOversizedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Cabc.jsonl")
	raw, err := json.Marshal(strings.Repeat("x", 4*1024*1024+1))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `{"t":"header","id":"Cabc","cwd":"/p"}`+"\n"+string(raw)+"\n"+`{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"after large output"}]}}`+"\n")
	c := readMakiTranscript(path)
	if c.Preview != "after large output" {
		t.Fatalf("Preview = %q", c.Preview)
	}
}
