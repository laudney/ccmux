## Why

ccmux supervises a long roster of terminal coding agents, but not Maki —
the Rust terminal coding agent at tontinton/maki (https://maki.sh). Maki
is in daily use in this environment (installed at `~/.cargo/bin/maki`,
state under `~/.local/state/maki`), so users who want it in the same
supervision workflow — dashboard states, bells, conversations, usage —
have no path today.

## What Changes

- Add the Maki CLI as a supported interactive agent with canonical ID
  `maki`, display name `Maki`, default binary `maki`.
- Register Maki in `agent.All()` after Gemini (canonical order append;
  Claude stays the default).
- Launch new Maki sessions with `maki`; resume the latest session for an
  existing project with `maki --continue` (falling back to `maki`, then
  an interactive shell). Explicit conversation resume is
  `maki --resume <id>` (`-r, --resume`, alias `-s, --session`).
- Detect Maki pane state with a bundled rule file
  (`internal/agentdetect/rules/maki.toml`) pinned to real captured pane
  fixtures: the braille status-bar spinner (working) and the
  " Permission Required " dialog (blocked, idle-gated). Maki sets no OSC
  title of its own, so no title rules exist.
- Parse Maki transcripts for the Conversations screen
  (`~/.local/state/maki/sessions/<id>.jsonl`, flat JSONL; the `archive/`
  subdirectory of user-deleted sessions is skipped). Maki transcripts
  carry no launch-mode marker (verified in upstream source: `maki -p`
  writes the same shape as interactive), so `IsHeadless()` is always
  false for maki — the Antigravity precedent.
- Aggregate Maki token usage with a small bespoke walker
  (`internal/makiusage`): the generic JSONL walker does not recognize
  maki's `token_usage` block inside `meta` events.
- Give Maki an accent color (Pink) for the dashboard/conversations
  color coding, an install hint (`curl -fsSL https://maki.sh/install.sh
  | sh`) in the doctor and setup wizard, and a placeholder Agents
  sub-tab (config is `~/.config/maki/init.lua` + `providers.toml`,
  managed by the maki CLI).
- OpenRouter routing needs no code: it is config-driven
  (`[openrouter] route_agents` resolves through `agent.ParseID`), and
  maki honors `OPENAI_BASE_URL` like the official SDKs (verified in
  upstream docs).

## Capabilities

### New Capabilities

- `maki-agent-support`: ccmux can supervise the Maki CLI as a
  first-class interactive agent — registration, launch/resume dialects,
  pane-state detection, conversation listing/resume, usage aggregation,
  and the user-facing surfaces (picker, dashboard badge, agents tab,
  doctor, wizard).
