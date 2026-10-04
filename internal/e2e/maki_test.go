//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skzv/ccmux/internal/agent"
	"github.com/skzv/ccmux/internal/conversations"
)

func TestMakiConversationLifecycle(t *testing.T) {
	e := newEnv(t)
	t.Setenv("XDG_STATE_HOME", filepath.Join(e.Home, "custom-state"))
	proj := filepath.Join(e.Root, "maki-proj")
	mkdirAll(t, proj)
	const id = "CfgxdrpL5JtPGPbHvDtTm"
	root := agent.ByID(agent.IDMaki).TranscriptsRoot(e.Home)
	path := filepath.Join(root, id+".jsonl")
	header := `{"t":"header","v":2,"id":"` + id + `","cwd":"` + proj + `"}`
	writeFile(t, path, header+"\n"+
		`{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"hidden API context"}],"display_text":""}}`+"\n"+
		`{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"fix the Maki test"}]}}`+"\n")
	writeFile(t, filepath.Join(root, "archive", "snapshot.jsonl"), header+"\n")
	list := e.listConversationsJSON()
	if len(list) != 1 || list[0].Agent != agent.IDMaki || list[0].ID != id ||
		list[0].Project != proj || list[0].Preview != "fix the Maki test" {
		t.Fatalf("Maki listing = %+v", list)
	}

	_, _, _ = e.ccmux("resume", id)
	session := conversations.ResumeSessionName(id)
	if !waitFor(3*time.Second, func() bool {
		pane, _ := e.tmux("capture-pane", "-p", "-t", session)
		return strings.Contains(pane, "ccmux-stub-agent=maki") &&
			strings.Contains(pane, "ccmux-stub-args=--resume "+id) &&
			strings.Contains(pane, "ccmux-stub-cwd="+proj)
	}) {
		pane, _ := e.tmux("capture-pane", "-p", "-t", session)
		t.Fatalf("Maki resume did not use the selected ID and project: %s", pane)
	}
	if _, stderr, err := e.ccmux("delete-conversation", id, "--force"); err != nil {
		t.Fatalf("delete Maki conversation: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deleted transcript still exists: %v", err)
	}
	if list := e.listConversationsJSON(); len(list) != 0 {
		t.Fatalf("deleted Maki conversation still listed: %+v", list)
	}
}
