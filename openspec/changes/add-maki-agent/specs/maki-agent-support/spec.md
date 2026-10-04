## ADDED Requirements

### Requirement: Maki Agent Registration

ccmux SHALL register the Maki CLI (tontinton/maki) as a supported
interactive agent with canonical ID `maki`, display name `Maki`, and
default binary `maki`.

#### Scenario: Maki is parsed from user input

- **WHEN** ccmux parses the agent ID `maki` (any case, surrounding
  whitespace trimmed)
- **THEN** parsing succeeds with the Maki agent ID

#### Scenario: Supported agents are enumerated

- **WHEN** ccmux enumerates supported agents
- **THEN** Maki appears after the existing agents, at the end of the
  canonical order
- **AND** Claude remains the default agent

### Requirement: Maki Launch Commands

ccmux SHALL launch Maki with the command dialect documented for
interactive Maki sessions.

#### Scenario: New Maki project starts an interactive session

- **GIVEN** a project is configured with agent `maki`
- **WHEN** ccmux starts a new project session
- **THEN** the tmux pane command starts `maki`

#### Scenario: Existing Maki project resumes the latest session

- **GIVEN** a project is configured with agent `maki`
- **WHEN** ccmux starts the session with continue
- **THEN** the pane command runs `maki --continue`, falling back to a
  fresh `maki` and then an interactive shell when maki is missing or
  has no prior session

#### Scenario: A specific conversation resumes by ID

- **GIVEN** a maki conversation with on-disk ID `<id>`
- **WHEN** ccmux resumes it
- **THEN** the launched argv is `maki --resume <id>`

### Requirement: Maki State Detection

ccmux SHALL classify maki panes through the data-driven rule engine
with a bundled `maki` rule file, falling back to the legacy quiet-pane
heuristic when no rule matches.

#### Scenario: A running turn is working

- **WHEN** the pane's status bar shows the braille spinner
- **THEN** the session classifies as working

#### Scenario: A permission dialog blocks on the user

- **GIVEN** the pane shows the " Permission Required " overlay with
  Allow/Deny options
- **WHEN** the pane has been quiet for the idle threshold
- **THEN** the session classifies as needing input
- **AND** the classification is idle-gated, so a dialog frozen on a
  dead pane does not ring

#### Scenario: Maki sets no OSC title

- **WHEN** the daemon classifies a maki pane
- **THEN** no bundled rule requires a title match (the window title is
  user/plugin territory)

### Requirement: Maki Conversation Listing

ccmux SHALL enumerate past maki sessions from
`~/.local/state/maki/sessions` for the Conversations screen and the
`ccmux list-conversations` CLI.

#### Scenario: Sessions are listed with previews

- **GIVEN** maki session JSONL files exist under
  `~/.local/state/maki/sessions`
- **WHEN** conversations are enumerated
- **THEN** each session appears with its ID, project label derived
  from the header cwd, last-activity time from the final `meta`
  event, and a preview from the first real user prompt

#### Scenario: Archived sessions are not listed

- **GIVEN** a session file exists under the `archive/` subdirectory
- **WHEN** conversations are enumerated
- **THEN** it does not appear (the user deleted it)

#### Scenario: Maki conversations resume by ID

- **WHEN** the user resumes a maki conversation row
- **THEN** the agent is launched with `maki --resume <id>`

### Requirement: Maki Headless Handling

Because maki transcripts carry no launch-mode marker (verified in
upstream source), ccmux SHALL treat maki conversations as never
headless.

#### Scenario: Print-mode runs are not filtered

- **WHEN** conversations are enumerated with the exclude-headless
  default
- **THEN** maki rows are never filtered (the Antigravity precedent)

### Requirement: Maki Usage Aggregation

ccmux SHALL report maki token usage on the dashboard by walking
maki session transcripts for their cumulative `token_usage` totals.

#### Scenario: Usage appears for agents with data

- **GIVEN** maki sessions with non-zero `token_usage` in the window
- **WHEN** the usage panel refreshes
- **THEN** a maki row shows prompt count and input+output tokens

#### Scenario: No recognizable usage yields no row

- **WHEN** no maki transcript in the window carries usable usage
- **THEN** no maki usage row is rendered

### Requirement: Maki User-Facing Surfaces

ccmux SHALL surface maki everywhere the supported-agent set is shown.

#### Scenario: Install guidance

- **WHEN** `ccmux doctor` runs or the setup wizard lists agents and
  maki is not installed
- **THEN** the hint names the official install script
  (`https://maki.sh/install.sh`)

#### Scenario: Color coding

- **WHEN** a dashboard row, conversations label, or agents sub-tab
  names the maki agent
- **THEN** it wears a stable accent color distinct from the other
  agents'
