# Implement GitHub Issue (Parallel-Safe)

This skill implements a GitHub issue in an isolated git worktree so multiple agents can work in parallel without interfering with each other.

## ⚠️ CRITICAL: Worktree Is MANDATORY

**You MUST create a worktree BEFORE writing any code.** Never work directly on `main`. Multiple agents run in parallel — working on `main` causes competing changes to overwrite each other. If worktree creation fails, STOP and report the error. Do not fall back to working on `main`.

## Steps

1. Fetch the specified GitHub issue: `gh issue view <issue-number>`
2. Create a branch from origin/main named `<issue-number>-<short-slug>` (e.g., `42-add-photo-upload`)
   ```bash
   git fetch origin main
   git branch <branch-name> origin/main
   ```
3. **[CRITICAL]** Create a worktree for the branch — do NOT skip this step:
   ```bash
   git worktree add .worktrees/issue-<issue-number> <branch-name>
   ```
4. cd into the worktree and run `flutter pub get`
5. Plan implementation across affected layers following existing code patterns
6. Implement changes
9. Commit with conventional commit message referencing the issue (e.g., `feat: add issue parser (#42)`)
10. Rebase onto latest main before pushing:
    ```bash
    git fetch origin main
    git rebase origin/main
    ```
11. If rebase has conflicts, attempt to resolve them. Re-run tests after rebase. If conflicts cannot be resolved, stop and notify the user.
12. Push the branch:
    ```bash
    git push -u origin <branch-name>
    ```
13. Create PR with `Closes #<issue-number>` in the body:
    ```bash
    gh pr create --title "<title>" --body "Closes #<issue-number>\n\n..."
    ```
