## Phase 34: Frontend-Aware Verification

**Goal**: When an agent implementation touches UI code, godark automatically detects the framework, generates interactive integration tests, captures screenshots, and uses vision to verify the result - so frontend work gets the same autonomous feedback loop that backend work already has.

**Milestone**: `Phase 34: Frontend-Aware Verification` | **Label**: `phase-34`

- UI framework detection — scan changed files for framework imports (Flutter, SwiftUI, UIKit, Jetpack Compose, Android XML layouts, React, Vue, Svelte) to determine if visual verification should activate
- Platform adapter — select the correct test framework (integration_test, XCUITest, Espresso, Playwright, etc.) and execution target (sandbox container vs host service) based on detected framework
- Host service routing for iOS — route iOS simulator execution to macOS host via host services when sandbox cannot run Xcode/simulators
- Integration test generation — agent generates interactive screenshot sequences: launch app, simulate taps/input, capture state, compare before/after
- Vision-based screenshot review — pipeline step that feeds captured screenshots to a vision model to verify layout correctness, element presence, and state transitions after interactions
- Conditional pipeline stage — visual verification only activates when UI framework imports are detected in changed files; backend-only work is unaffected
