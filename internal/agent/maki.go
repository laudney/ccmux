package agent

import (
	"path/filepath"
	"time"
)

// Maki is the Maki CLI (tontinton/maki, https://maki.sh) — a Rust
// terminal coding agent. Binary: `maki`. Config: ~/.config/maki
// (init.lua, providers.toml, permissions.toml, mcp.toml); session data
// under ~/.local/state/maki (XDG split). Reads AGENTS.md for project
// context. Honors ANTHROPIC_BASE_URL / OPENAI_BASE_URL like the
// official SDKs, so OpenRouter routing applies.
type Maki struct{}

func (Maki) ID() ID              { return IDMaki }
func (Maki) DisplayName() string { return "Maki" }
func (Maki) Binary() string      { return "maki" }

func (Maki) LaunchCmd(continueFlag bool) string {
	// `maki --continue` resumes the most recent session in this
	// directory (`-c, --continue`); the zsh→bash→sh fallback keeps the
	// pane alive when maki is missing. See launchChain.
	if continueFlag {
		return "maki --continue || maki || zsh || bash || sh"
	}
	return "maki"
}

func (Maki) ConfigRoot(home string) string { return filepath.Join(home, ".config", "maki") }

// TranscriptsRoot is maki's session store: one flat <id>.jsonl per
// session (a header line, then msg/meta/out/frame events). The archive/
// subdirectory holds user-deleted sessions and is not listed.
func (Maki) TranscriptsRoot(home string) string {
	return filepath.Join(home, ".local", "state", "maki", "sessions")
}

func (Maki) InitialPrompt(name, description string) string {
	return agentsMdInitialPrompt(name, description)
}

func (Maki) Classify(pane string, lastChange time.Time, idleThreshold time.Duration) State {
	return engineClassify(IDMaki, pane, "", lastChange, idleThreshold)
}

// ClassifyWithTitle routes through the data-driven engine
// (internal/agentdetect). Maki's rule file matches the "Permission
// Required" dialog and the braille working spinner in the status bar.
// Maki sets no OSC title of its own (the title is user/plugin-set), so
// the title region stays empty. Falls back to the legacy time-based
// heuristic when no rule matches.
func (Maki) ClassifyWithTitle(pane, title string, lastChange time.Time, idleThreshold time.Duration) State {
	return engineClassify(IDMaki, pane, title, lastChange, idleThreshold)
}
