## Design

### Verified facts (upstream tontinton/maki source + live v0.6.0 sessions)

- Binary/flags: `maki [PROMPT]`; `-c, --continue` (resume latest in
  cwd), `-r, --resume <id>` (alias `-s, --session`), `-p, --print`
  (headless, Claude-Code-compatible wire format).
- Config root: `~/.config/maki` (`init.lua`, `providers.toml`,
  `permissions.toml`, `mcp.toml`). Session data: `~/.local/state/maki/`
  (`sessions/<id>.jsonl`, `projects/<slug>-<hash>/`, `auth/`). Cache:
  `~/.cache/maki`. XDG overrides are honored on Unix. If `~/.maki` exists,
  upstream and the fork both use it for config and state. Windows
  uses AppData/Roaming. Legacy session `<id>.json` files remain readable.
- Transcript JSONL: header line `{"t":"header","v":2,"id","model",
  "cwd","created_at"}` (epoch seconds), then `msg` (`d.role`,
  `d.content[].text`), `meta` (`title`, `token_usage` — cumulative,
  Anthropic-style field names — `updated_at`, `mode`), `out`, `frame`
  events. No launch-mode marker anywhere (`SessionMeta` has
  build/plan only; print mode hardcodes build).
- Pane signatures: working = braille spinner (U+2800–U+28FF frames,
  80 ms) in the status bar's last line and on running tool headers;
  blocked = the `╭ Permission Required ─╮` overlay and the question
  form. The spinner keeps moving while these forms await input. Maki
  never sets the OSC window title itself (that API is
  exposed to users/plugins via `maki.ui.set_window_title`), so state
  rules read the body only.
- `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` are honored like the
  official SDKs. The generic OpenRouter wrapper only redirects Maki
  when its selected provider uses the OpenAI platform API. It does not
  select a model or redirect an Anthropic or Coding Plan session; use
  Maki's native OpenRouter provider when that is the intended provider.

### Decisions

1. **Second-wave registration shape** — `Maki{}` mirrors `Kilo{}`: no
   per-agent `[agents.*] command` override (that surface stayed
   first-wave), `agentsMdInitialPrompt` bootstrap (maki reads
   AGENTS.md), engine-backed `ClassifyWithTitle`.
2. **Resume beyond the second wave** — `resumeArgv` gets a `maki` case
   (`maki --resume <id>`), so Conversations rows resume directly; most
   second-wave agents return nil there. Maki is a supervised daily
   driver in this environment, so the full flow is wired.
3. **Headless: none** — no marker in the transcript, so
   `IsHeadless()` is always false (Antigravity precedent). If upstream
   later adds a marker, it is a two-line change (parse + case).
4. **Detection rules from live fixtures** — `testdata/panes/
   maki_{idle,working,permission}.txt` captured from a real v0.6.0
   session via the documented tmux recipe. Tests run each captured
   pane through the public classifier with fresh and quiet timestamps.
   Explicit dialogs bypass the idle gate because their spinner still
   animates; the daemon's signs-of-life check covers a dead process.
5. **Usage: bespoke walker** — `internal/makiusage` (agentusage's
   Summary contract) because the generic walker matches `usage` keys /
   top-level token fields, not maki's cumulative `token_usage` inside
   `meta`. Only snapshot deltas in the window contribute tokens. For an
   old compacted session without a baseline, the first surviving snapshot
   seeds later deltas; its older totals cannot be assigned to the window.
   Cache tokens are excluded, consistent with the generic walker.
   Prompts honor `display_text` and exclude synthetic messages, host
   observations, context updates and slash-command-only turns.
6. **Accent: Pink** — the only design-token accent not yet assigned to
   an agent.
7. **Native config editor** — reuse the Agents file browser and shared
   editor process. Show Maki's four native config files without parsing or
   rewriting Lua/TOML. Viewing the pane does not create files; editing a
   missing file creates its directory before the editor runs. Re-read the
   preview after the editor returns and when entering the Maki sub-tab.
8. **Registry-based selection and service discovery** — default selectors
   and MCP launch descriptions enumerate `agent.All()`. Generated service
   PATH retains directories with detected agent binaries in caller PATH
   order, after configured command directories and before system defaults.

### Compatibility evidence

The review compared the storage, provider-message and CLI contracts in
`tontinton/maki` upstream `e6fc72a4` with local fork `1120a60c`. The
public integration uses the common `maki --continue` / `--resume`
commands and native formats; it requires no fork plugins. Fork-only
named runtime profiles are outside this integration. Maki does not
record the launch mode, so print sessions cannot be filtered reliably.
