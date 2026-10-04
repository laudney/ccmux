package conversations

import (
	"path/filepath"
	"reflect"
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
{"t":"meta","title":"fix the flaky test","token_usage":{"input":10},"updated_at":1791106299,"mode":"build"}
{"t":"out","v":1,"data":"noise"}
{"t":"meta","title":"fix the flaky test","token_usage":{"input":20},"updated_at":1791106400,"mode":"build"}
`

func TestListMaki_HappyPath(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".local/state/maki/sessions/CfgxdrpL5JtPGPbHvDtTm.jsonl"), makiSession)

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
	home := t.TempDir()
	// Header id deliberately differs from the filename stem.
	writeFile(t, filepath.Join(home, ".local/state/maki/sessions/otherid.jsonl"),
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
	home := t.TempDir()
	root := filepath.Join(home, ".local/state/maki/sessions")
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
	empty := t.TempDir()
	if got, err := ListMaki(filepath.Join(empty, "nowhere")); err != nil || got != nil {
		t.Errorf("missing root: got %v, %v; want nil, nil", got, err)
	}
	if got, err := ListMaki(empty); err != nil || got != nil {
		t.Errorf("empty root: got %v, %v; want nil, nil", got, err)
	}
}

func TestListMaki_UnparseableFileTolerated(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".local/state/maki/sessions")
	// No header at all: the file still lists, ID falling back to the
	// filename stem and LastActivity to the mtime.
	writeFile(t, filepath.Join(root, "bareid.jsonl"), "garbage\nlines\n")

	got, err := ListMaki(home)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListMaki = %v, %v", got, err)
	}
	c := got[0]
	if c.ID != "bareid" {
		t.Errorf("ID = %q, want the filename stem", c.ID)
	}
	if c.Preview != "" {
		t.Errorf("Preview = %q, want empty", c.Preview)
	}
	if c.LastActivity.IsZero() {
		t.Error("LastActivity = zero, want the file mtime fallback")
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
		"user: [Cancelled by user]",
		"user: fix  the\n\tflaky test",
		"assistant: On it.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RecentMessages = %v, want %v", got, want)
	}

	if n, err := CountMessages(c); err != nil || n != 4 {
		t.Fatalf("CountMessages = %d, %v; want 4", n, err)
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
	home := t.TempDir()
	root := filepath.Join(home, ".local/state/maki/sessions")
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
