package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Maki is the terminal coding agent at https://github.com/tontinton/maki.
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

func (Maki) ConfigRoot(home string) string {
	return makiRoot(home, "XDG_CONFIG_HOME", ".config")
}

// TranscriptsRoot is Maki's flat JSONL/legacy JSON session store.
// archive/ contains old snapshots and is not listed.
func (Maki) TranscriptsRoot(home string) string {
	return filepath.Join(makiRoot(home, "XDG_STATE_HOME", ".local/state"), "sessions")
}

// Both upstream Maki and the fork use ~/.maki when it exists. Otherwise
// etcetera selects XDG directories on Unix and AppData/Roaming on Windows.
func makiRoot(home, key, fallback string) string {
	legacy := filepath.Join(home, ".maki")
	if info, err := os.Stat(legacy); err == nil && info.IsDir() {
		return legacy
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if !filepath.IsAbs(base) {
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "maki")
	}
	base := os.Getenv(key)
	if !filepath.IsAbs(base) {
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, "maki")
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
// Maki sets no OSC title of its own (the title is user/plugin-set).
// Explicit input dialogs override its still-moving status spinner.
// Falls back to the legacy time-based heuristic when no rule matches.
func (Maki) ClassifyWithTitle(pane, title string, lastChange time.Time, idleThreshold time.Duration) State {
	return engineClassify(IDMaki, pane, title, lastChange, idleThreshold)
}
