## Phase 39: Meta-Agent Human Mode

**Goal**: A `godark meta propose` command that reads trace data and emits cited
prompt-edit proposals for humans to review and apply manually. No side effects
beyond a proposals directory. Automates the "find the gap" half of the
prompt-tuning workflow described in `docs/philosophy/engineering-roles.md`;
humans still own the "adjust and commit" half.

**Milestone**: `Phase 39: Meta-Agent Human Mode` | **Label**: `phase-39`

**Depends on**: Phase 37 (Benchmarking Framework)

- Corpus query helpers — aggregation functions over `stats.db` grouped by `harness_hash`, `prompt_hash`, `trace_id`, and outcome status
- Transcript reader — decompress and iterate `*-transcript.jsonl.gz` artifacts from run directories
- `godark meta` command skeleton — parent command, shared flags, `propose` subcommand stub
- Proposal JSON schema — structured format (file path, anchor text, replacement, rationale, cited `trace_id`s and `prompt_hash`es)
- Meta-agent prompt template — system prompt that forces citations and rejects hand-wavy proposals
- `meta propose` implementation — invoke Claude Code with read-only corpus tools, parse output, write to `~/.godark/proposals/<timestamp>/`
- Citation validator — verify every cited `trace_id` and `prompt_hash` resolves against `stats.db`; reject proposals with dead citations
- Markdown report generator — human-readable report from the proposal JSON with links into `godark trace`
- First real proposal run — evaluate output quality against existing godark traces
