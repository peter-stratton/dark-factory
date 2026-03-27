## Phase 19: Spring Cleaning ✅

**Goal**: The codebase has zero duplicated patterns, all agent output parsing
uses structured formats with unified parsers, magic strings are typed constants,
and shared helpers replace copy-pasted boilerplate — making every file a clean
example of the project's conventions.

**Milestone**: `Phase 19` | **Label**: `phase-19`

### Verdict parsing & prompt consolidation
- Unify prompt verdict format — replace `REVIEW_RESULT=` and
  `QUALITY_RESULT=` with single `AGENT_RESULT=` prefix across `reviewer.txt`
  and `quality_reviewer.txt`
- Unified verdict parser — extract `parseVerdictFromOutput(stdout, keyword)`
  replacing duplicate `ParseReviewResult()` / `ParseQualityResult()`
- Extract shared CRITICAL RULES template variable — `{{.CriticalRulesText}}`
  rendered from single source, replacing duplicated rules across 5 prompt files
- Single-source scenario spec format — deduplicate format definition between
  `spec_generator.txt` and `godark-create-scenarios/SKILL.md`

### Agent loop simplification
- Extract review cycle function — `processReviewCycle()` to flatten quality
  and functional review nesting in `loop.go`
- Extract non-blocking agent result handler — shared
  `handleNonBlockingResult()` for spec-gen, recon, and verify
  error/timeout/hook-write boilerplate
- Extract handoff policy function — `shouldUseHandoff()` and
  `buildRetryContext()` replacing scattered session/handoff conditionals
- Extract drift-check helper — consolidate 4 repeated
  `checkDriftAndClose()` + early-return blocks

### CLI and command helpers
- Extract CLI flag parser — shared `parseCLIFlagsToConfig()` replacing
  duplicate flag blocks in `run.go` and `implement.go`
- Consolidate config resolution — move inline tag/milestone resolution from
  `run.go` into `vet_helpers.go` resolve functions with early returns
- Extract vet data fetcher — shared `fetchVetData(repo, milestone)` replacing
  duplicate GitHub fetch patterns across vet commands
- Consolidate file scaffold functions — shared `scaffoldDocs()` /
  `scaffoldPrompts()` replacing duplicate loops in `init.go` and `new.go`

### Shared utilities
- Extract `writeFileWithDirs()` helper — replace repeated `os.MkdirAll` +
  `os.WriteFile` + error-wrap pattern
- Extract `WalkMarkdownFiles()` helper — replace 3 identical
  `filepath.WalkDir` + `.md` filter patterns
- Extract `extractJSONFromText()` helper — safe JSON extraction replacing
  brittle `strings.Index("[")` / `strings.LastIndex("]")` in punchlist parsing
- Extract checkbox/bullet markdown parser — `extractCheckboxItem()` and
  `extractBulletItem()` replacing duplicated `HasPrefix`/`TrimPrefix` chains

### Type safety and constants
- Define outcome status constants — typed `OutcomeStatus` replacing magic
  strings in `implement.go` switch
- Define merge strategy enum — typed `MergeStrategy` with `Valid()` method
  replacing inline string checks
- Extract `issueDir()` method on `rundata.Writer` — replace 15+
  `fmt.Sprintf("%d", issueNum)` path constructions
- Group truncation limits into config struct — consolidate `maxPRDiffLen`,
  `verifyOutputLimit`, and other scattered magic numbers

### Test and infrastructure cleanup
- Consolidate skills test helpers — shared `readSkill(t, name)` and
  `parseFrontmatter()` replacing 6 duplicate test functions
- Unify `CommandRunner` pattern — shared interface replacing 3 independent
  `var CommandRunner` definitions across packages

**Issues**: #384–#408

**Planning doc**: `docs/planning/phase-19-spring-cleaning.md`

