package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/skzv/ccmux/internal/agent"
	"github.com/skzv/ccmux/internal/tui/styles"
)

func TestAgents_MakiConfigFiles(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "xdg", true: "legacy"}[legacy], func(t *testing.T) {
			home, configHome := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("XDG_CONFIG_HOME", configHome)
			t.Setenv("APPDATA", configHome)
			t.Setenv("EDITOR", "maki-test-editor")
			t.Setenv("VISUAL", "")
			if legacy {
				if err := os.Mkdir(filepath.Join(home, ".maki"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			root := (agent.Maki{}).ConfigRoot(home)
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			names := []string{"init.lua", "providers.toml", "permissions.toml", "mcp.toml"}
			for _, name := range names {
				if err := os.WriteFile(filepath.Join(root, name), []byte("config-preview-"+name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			m := newAgents(styles.Default(), DefaultKeymap())
			// Follow the actual sub-tab navigation, without running other
			// agents' background commands while crossing their tabs.
			for range agentConfigSubtabs() {
				m, _ = m.Update(keyMsg("l"))
				if m.active == agent.IDMaki {
					break
				}
			}
			if m.active != agent.IDMaki {
				t.Fatal("Maki config is unreachable through the Agents sub-tabs")
			}
			m.SetSize(120, 35)
			assertPresent(t, m.View(120, 35), "Maki", "init.lua", "providers.toml", "permissions.toml", "mcp.toml")
			for _, name := range names {
				if !strings.Contains(m.View(120, 35), "config-preview-"+name) {
					t.Fatalf("selected %s preview is missing", name)
				}
				next, edit := m.Update(keyMsg("e"))
				m = next
				if edit == nil {
					t.Fatalf("%s has no edit action", name)
				}
				msg, ok := edit().(openEditorMsg)
				if !ok || msg.Path != filepath.Join(root, name) || msg.Source != "agents" || msg.Editor != "maki-test-editor" {
					t.Fatalf("%s edit message = %#v", name, msg)
				}
				m, _ = m.Update(keyMsg("j"))
			}
			m, _ = m.Update(keyMsg("l"))
			if m.active != agent.IDClaude {
				t.Fatalf("after Maki, active = %s, want Claude", m.active)
			}
		})
	}
}

func TestMakiConfig_MissingFilesCanBeEdited(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "custom-config"))
	t.Setenv("APPDATA", filepath.Join(home, "custom-config"))
	m := newMakiConfig(styles.Default())
	if _, err := os.Stat(m.root); !os.IsNotExist(err) {
		t.Fatalf("opening the pane created the config directory: %v", err)
	}
	m, cmd := m.Update(keyMsg("e"))
	if cmd == nil {
		t.Fatal("missing init.lua cannot be edited")
	}
	msg, ok := cmd().(openEditorMsg)
	if !ok || msg.Path != filepath.Join(m.root, "init.lua") {
		t.Fatalf("missing file edit message = %#v", msg)
	}
	if info, err := os.Stat(m.root); err != nil || !info.IsDir() {
		t.Fatalf("edit did not create the config directory: %v", err)
	}
	if _, err := os.Stat(msg.Path); !os.IsNotExist(err) {
		t.Fatalf("edit action wrote the config before the editor ran: %v", err)
	}
}

func TestApp_MakiConfigReloadsAfterEdit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	a := newAppForTest(t)
	a.agentsM.active = agent.IDMaki
	_, cmd := a.agentsM.Update(keyMsg("e"))
	if cmd == nil {
		t.Fatal("missing edit action")
	}
	msg := cmd().(openEditorMsg)
	if err := os.WriteFile(msg.Path, []byte("updated-maki-config\n\x1b]52;c;c2VjcmV0\x07"), 0o600); err != nil {
		t.Fatal(err)
	}
	updated, _ := a.Update(editorReloadMsg(msg.Source))
	a = updated.(App)
	for _, width := range []int{50, 80, 160} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			a.agentsM.SetSize(width, 35)
			out := a.agentsM.View(width, 35)
			assertNoOverflow(t, out, width)
			assertPresent(t, out, "updated-maki-config")
			assertAbsent(t, out, "\x1b]52", "c2VjcmV0")
		})
	}
}

func TestMakiConfig_ConfigDirectoryError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	configHome := filepath.Join(home, "not-a-directory")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)
	if err := os.WriteFile(configHome, []byte("keep-this-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newMakiConfig(styles.Default())
	_, cmd := m.Update(keyMsg("e"))
	if cmd == nil {
		t.Fatal("missing edit action")
	}
	if msg, ok := cmd().(toastMsg); !ok || msg.Kind != toastError || msg.Text == "" {
		t.Fatalf("directory error must show an error instead of launching the editor: %#v", msg)
	}
	if data, err := os.ReadFile(configHome); err != nil || string(data) != "keep-this-file" {
		t.Fatalf("edit changed the existing file: %q, %v", data, err)
	}
}
