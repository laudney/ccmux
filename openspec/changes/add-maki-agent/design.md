## Design

### Verified facts (upstream tontinton/maki source + live v0.6.0 sessions)

- Binary/flags: `maki [PROMPT]`; `-c, --continue` (resume latest in
  cwd), `-r, --resume <id>` (alias `-s, --session`), `-p, --print`
  (headless, Claude-Code-compatible wire format).
- Config root: `~/.config/maki` (`init.lua`, `providers.toml`,
  `permissions.toml`, `mcp.toml`). Session data: `~/.local/state/maki/`
  (`sessions/<id>.jsonl`, `projects/<slug>-<hash>/`, `auth/`). Cache:
  `~/.cache/maki`. Strict XDG split, like OpenCode/Kilo.
- Transcript JSONL: header line `{"t":"header","v":2,"id","model",
  "cwd","created_at"}` (epoch seconds), then `msg` (`d.role`,
  `d.content[].text`), `meta` (`title`, `token_usage` — cumulative,
  Anthropic-style field names — `updated_at`, `mode`), `out`, `frame`
  events. No launch-mode marker anywhere (`SessionMeta` has
  build/plan only; print mode hardcodes build).
- Pane signatures: working = braille spinner (U+2800–U+28FF frames,
  80 ms) in the status bar's last line and on running tool headers;
  blocked = the `╭ Permission Required ─╮` overlay (y Allow / n Deny
  rows). Maki never sets the OSC window title itself (that API is
  exposed to users/plugins via `maki.ui.set_window_title`), so state
  rules read the body only.
- `ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` are honored like the
  official SDKs, so the existing config-driven OpenRouter routing
  (`[openrouter] route_agents`) applies without code.

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
   session via the documented tmux recipe; rules carry `require_idle`
   on the blocked rule (a frozen dialog on a dead pane is not a
   request) and none on the spinner (the daemon's signs-of-life check
   covers frozen frames).
5. **Usage: bespoke walker** — `internal/makiusage` (agentusage's
   Summary contract) because the generic walker matches `usage` keys /
   top-level token fields, not maki's cumulative `token_usage` inside
   `meta`. Last-meta-per-file wins; cache tokens ignored (consistent
   with the generic walker); user turns exclude `[Cancelled by user]`
   markers.
6. **Accent: Pink** — the only design-token accent not yet assigned to
   an agent.
