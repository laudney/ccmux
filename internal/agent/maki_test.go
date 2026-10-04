package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestMakiRoots(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "xdg", true: "legacy"}[legacy], func(t *testing.T) {
			home := t.TempDir()
			config, state := t.TempDir(), t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", config)
			t.Setenv("XDG_STATE_HOME", state)
			t.Setenv("APPDATA", config)
			wantConfig := filepath.Join(config, "maki")
			wantState := filepath.Join(state, "maki", "sessions")
			if runtime.GOOS == "windows" {
				wantState = filepath.Join(config, "maki", "sessions")
			}
			if legacy {
				wantConfig = filepath.Join(home, ".maki")
				wantState = filepath.Join(wantConfig, "sessions")
				if err := os.Mkdir(wantConfig, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if got := (Maki{}).ConfigRoot(home); got != wantConfig {
				t.Errorf("ConfigRoot = %q, want %q", got, wantConfig)
			}
			if got := (Maki{}).TranscriptsRoot(home); got != wantState {
				t.Errorf("TranscriptsRoot = %q, want %q", got, wantState)
			}
		})
	}
}

func TestMakiRejectsRelativeRoots(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative")
	t.Setenv("XDG_STATE_HOME", "relative")
	t.Setenv("APPDATA", "relative")
	home := t.TempDir()
	wantConfig := filepath.Join(home, ".config", "maki")
	wantState := filepath.Join(home, ".local", "state", "maki", "sessions")
	if runtime.GOOS == "windows" {
		wantConfig = filepath.Join(home, "AppData", "Roaming", "maki")
		wantState = filepath.Join(wantConfig, "sessions")
	}
	if got := (Maki{}).ConfigRoot(home); got != wantConfig {
		t.Errorf("ConfigRoot = %q, want %q", got, wantConfig)
	}
	if got := (Maki{}).TranscriptsRoot(home); got != wantState {
		t.Errorf("TranscriptsRoot = %q, want %q", got, wantState)
	}
}

func TestMakiDialogsOverrideMovingSpinner(t *testing.T) {
	for _, pane := range []string{
		"╭ Permission Required ───╮\n│ y Allow  n Deny        │\n╰───────────────────────╯\n ⠹ [BUILD] /p",
		"╭ Permission Required ───╮\n│ Enter / y Confirm allow-always (project) │\n╰───────────────────────╯\n ⠹ [BUILD] /p",
		"╭ Question ───╮\n│ Which option? │\n│ Enter submit  Tab next  Esc dismiss │\n╰─────────────╯\n ⠹ [BUILD] /p",
	} {
		if got := (Maki{}).Classify(pane, time.Now(), time.Minute); got != StateNeedsInput {
			t.Errorf("Classify moving dialog = %s, want %s", got, StateNeedsInput)
		}
	}
}

func TestMakiSpinnerMustBeInStatusBar(t *testing.T) {
	pane := "maki> Here is a braille example\n [BUILD] /p ⠹"
	if got := (Maki{}).Classify(pane, time.Now().Add(-time.Hour), time.Second); got != StateNeedsInput {
		t.Errorf("Classify completed output = %s, want %s", got, StateNeedsInput)
	}
}
