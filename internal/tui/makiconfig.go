package tui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/skzv/ccmux/internal/agent"
	"github.com/skzv/ccmux/internal/tui/styles"
)

// makiConfigModel previews native config files and opens the selected
// file in the user's editor. Maki remains responsible for their format.
type makiConfigModel struct {
	st      styles.Styles
	browser agentBrowser
	root    string
	editor  string
	err     string
}

func newMakiConfig(st styles.Styles) makiConfigModel {
	m := makiConfigModel{st: st, browser: newAgentBrowser(st), editor: pickEditor()}
	m.reload()
	return m
}

func (m *makiConfigModel) reload() {
	home, err := os.UserHomeDir()
	if err != nil {
		m.root, m.err = "", err.Error()
		m.browser.SetSections("Maki configuration", nil)
		return
	}
	m.root, m.err = (agent.Maki{}).ConfigRoot(home), ""
	section := agentBrowserSection{Title: "Files", Color: m.st.P.Pink}
	for _, name := range []string{"init.lua", "providers.toml", "permissions.toml", "mcp.toml"} {
		data, err := os.ReadFile(filepath.Join(m.root, name))
		preview := string(data)
		if os.IsNotExist(err) {
			preview = tr("(not configured)")
		} else if err != nil {
			preview = err.Error()
		}
		section.Items = append(section.Items, agentBrowserItem{Label: name, Preview: preview})
	}
	m.browser.SetSections("Maki configuration", []agentBrowserSection{section})
}

func (m makiConfigModel) Update(msg tea.Msg) (makiConfigModel, tea.Cmd) {
	if b, cmd, handled := m.browser.Update(msg); handled {
		m.browser = b
		return m, cmd
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "e" {
		item, ok := m.browser.SelectedItem()
		if !ok || m.root == "" {
			return m, nil
		}
		path := filepath.Join(m.root, item.Label)
		return m, func() tea.Msg {
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return toastMsg{Text: err.Error(), Kind: toastError, Until: time.Now().Add(5 * time.Second)}
			}
			return openEditorMsg{Editor: m.editor, Path: path, Source: "agents"}
		}
	}
	return m, nil
}

func (m makiConfigModel) viewBodyHeader(width int) string {
	lines := []string{m.st.Subtitle.Render(tr("Maki configuration"))}
	if !isNarrow(width) {
		lines = append(lines, m.st.Muted.Render(summarizePath(m.root)))
	}
	lines = append(lines, m.st.Muted.Width(width).Render(tr("Select a file and press e to edit. Manage authentication with maki auth login.")))
	if m.err != "" {
		lines = append(lines, m.st.StatusError.Width(width).Render(m.err))
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func (m makiConfigModel) ViewBody(width, height int) string {
	header := m.viewBodyHeader(width)
	return lipgloss.JoinVertical(lipgloss.Left, header, m.browser.ViewFit(width, height-lipgloss.Height(header)))
}

func (m *makiConfigModel) setSize(width, height int) {
	m.browser.SetSize(width, max(8, height-lipgloss.Height(m.viewBodyHeader(width))))
}
