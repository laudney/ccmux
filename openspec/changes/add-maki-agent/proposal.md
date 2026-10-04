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
  " Permission Required " dialog and question forms (blocked, even
  while the status spinner moves). Maki sets no OSC
  title of its own, so no title rules exist.
- Parse Maki transcripts for the Conversations screen
  (`~/.local/state/maki/sessions/<id>.jsonl`, flat JSONL; the `archive/`
  subdirectory of old snapshots is skipped; legacy JSON, the
  `~/.maki` layout and XDG overrides are supported). Maki transcripts
  carry no launch-mode marker (verified in upstream source: `maki -p`
  writes the same shape as interactive), so `IsHeadless()` is always
  false for maki — the Antigravity precedent.
- Aggregate Maki token usage with a small bespoke walker
  (`internal/makiusage`): the generic JSONL walker does not recognize
  maki's `token_usage` block inside `meta` events.
- Give Maki an accent color (Pink) for the dashboard/conversations
  color coding, an install hint (`curl -fsSL https://maki.sh/install.sh
  | sh`) in the doctor and setup wizard, and an Agents sub-tab that
  previews `init.lua`, `providers.toml`, `permissions.toml`, and
  `mcp.toml` in the native config directory and opens the selected
  file in the user's editor. Maki owns the file formats and authentication.
- Use the shared agent registry for default-agent selectors and MCP
  launch descriptions. Include detected agent directories in generated
  daemon service PATH so PATH-only installations remain launchable.
- Provider and model selection remain in Maki. The generic OpenRouter
  wrapper only redirects the OpenAI platform provider; it does not
  change Maki's selected provider. Maki also has a native OpenRouter
  provider.

## Capabilities

### New Capabilities

- `maki-agent-support`: ccmux can supervise the Maki CLI as a
  first-class interactive agent — registration, launch/resume dialects,
  pane-state detection, conversation listing/resume, usage aggregation,
  and the user-facing surfaces (picker, dashboard badge, agents tab,
  doctor, wizard).
