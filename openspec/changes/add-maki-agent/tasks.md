## 1. OpenSpec

- [x] 1.1 Proposal, design, capability spec, and this checklist for
  Maki agent support.

## 2. Agent Model

- [x] 2.1 Add `internal/agent/maki.go` implementing the `Agent`
  interface: ID `maki`, display name `Maki`, binary `maki`,
  `LaunchCmd` (`maki` / `maki --continue || maki || zsh || bash ||
  sh`), `ConfigRoot` = `~/.config/maki`, `TranscriptsRoot` =
  `~/.local/state/maki/sessions`, AGENTS.md-centered `InitialPrompt`,
  engine-backed `Classify`/`ClassifyWithTitle`.
- [x] 2.2 Register Maki in `agent.go`: `IDMaki` const, `Maki{}` in
  `All()` (after Gemini), cases in `ByID`/`ParseID`, and the
  `maki --resume <id>` branch in `resumeArgv`.
- [x] 2.3 Focused unit tests: canonical-order, every-agent-complete,
  launch/continue table, picker cycle (`nextAgent`) extended for maki.

## 3. State Detection

- [x] 3.1 Capture real pane fixtures via the documented tmux recipe
  (`maki_idle`, `maki_working`, `maki_permission` + `.title` sidecars).
- [x] 3.2 Add `internal/agentdetect/rules/maki.toml` (status-bar
  braille spinner → working; explicit permission and question dialogs
  → blocked without the idle gate) and second-wave detection test rows.

## 4. Conversations & Usage

- [x] 4.1 `internal/conversations/maki.go`: `ListMaki` (flat
  `sessions/*.jsonl` and legacy `*.json`, skip `archive/`), `readMakiMessages`,
  `countMakiMessages`, resume dispatch, project-filter roots, package
  doc updates; `IsHeadless()` false (no marker — Antigravity
  precedent). Fixture-driven tests.
- [x] 4.2 `internal/makiusage` walker (deltas in cumulative `token_usage` between
  `meta` snapshots; `usage_by_model` ignored for cost) wired into
  `internal/usage`'s per-agent dispatch. Fixture-driven tests.

## 5. Product Surfaces

- [x] 5.1 Install hints: `cmd/ccmux/cmd` `agentInstallHint` and
  `internal/setupwizard` `installHintFor` (`curl -fsSL
  https://maki.sh/install.sh | sh`). Doctor/wizard iterate `All()`.
- [x] 5.2 TUI: Agents sub-tab with native config previews and editor,
  Conversations section
  nav + roots legend, Pink accent in `styles.AgentAccent`.
- [x] 5.3 Default-agent selectors in setup and Settings use the registry;
  MCP launch descriptions include all registered agent IDs.
- [x] 5.4 Generated daemon service PATH includes detected agent directories,
  including Maki installations in `~/.cargo/bin`.

## 6. Docs

- [x] 6.1 README (agent list, badge tags, cycle wording),
  `docs/01_Specs/02_Multi_Agent.md`,
  `docs/02_Architecture/04_Agents.md`.
- [ ] 6.2 Website docs/MDX where the supported-agent set is
  user-visible. DEFERRED: separate `ccmux-website` repo (same reasoning
  as the Grok change — the site's agent lists are already stale and
  need their own backfill pass with sign-off).

## 7. Verification

- [x] 7.1 `go test ./...`, `gofmt -l`, `go vet ./...`, `make lint`,
  `make build`; cross-compile `GOOS=linux` / `GOOS=windows`.
- [x] 7.2 Verified against the real maki install: `ConfigRoot` /
  `TranscriptsRoot` paths, `--continue` / `--resume <id>` flags from
  `maki --help`, pane fixtures from a live v0.6.0 session; live
  `ccmux doctor` shows `✓ Maki (maki)` and live
  `ccmux list-conversations` lists real maki sessions; e2e suite
  (`make test-e2e`) green.

## Review corrections

- [x] Honor legacy `~/.maki`, XDG overrides and Windows AppData paths.
- [x] Read legacy JSON sessions without counting JSON/JSONL duplicates.
- [x] Honor hidden/replaced display text and host message kinds.
- [x] Prevent full historical totals from entering a resumed usage window.
- [x] Test the real pane captures with both fresh and quiet timestamps.
- [x] Test CLI listing, exact resume argv/cwd and deletion in isolated tmux.
