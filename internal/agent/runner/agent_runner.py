#!/usr/bin/env python3
"""Agent runner for dark-factory.

Reads configuration from environment variables and invokes the Claude Agent SDK.
Streams all messages to stdout as newline-delimited JSON and prints a final
structured result line.

Environment variables:
  GODARK_PROMPT            The prompt text to send to the agent
  GODARK_ROLE              Agent role: implementer, implementer_retry, reviewer, quality_reviewer, or spec_generator
  GODARK_SESSION_ID        Session ID for resuming a previous session
  GODARK_WORKDIR           Working directory (default: /workspace)
  GODARK_PROTECTED_PATHS   Comma-separated list of protected paths (exact or dir prefix)
  GODARK_GENERATED_PATHS   Comma-separated list of generated paths (dir prefix or glob pattern)
  GODARK_DENIED_COMMANDS   Comma-separated list of denied Bash command patterns (substring match)
  GH_TOKEN                 GitHub token forwarded to the agent environment
"""

import asyncio
import datetime
import fnmatch
import json
import os
import random
import sys
import time
import traceback

import claude_agent_sdk
from claude_agent_sdk import ClaudeAgentOptions, ResultMessage
from claude_agent_sdk.types import (
    HookMatcher,
    PostToolUseHookInput,
    PreToolUseHookInput,
)


def _log_diagnostic(event: str, **fields) -> None:
    """Write a structured diagnostic line to stderr."""
    entry = {"diagnostic": event, "timestamp": datetime.datetime.utcnow().isoformat() + "Z", **fields}
    print(json.dumps(entry, default=str), file=sys.stderr, flush=True)

# Role-scoped tool permissions. Each role maps to allowed_tools and optionally
# disallowed_tools. disallowed_tools are hard-denied even if allowed_tools
# would otherwise permit them.
_ROLE_PERMISSIONS: dict[str, dict] = {
    "implementer": {
        "allowed_tools": ["Read", "Write", "Edit", "Bash", "Glob", "Grep"],
    },
    "implementer_retry": {
        "allowed_tools": ["Read", "Write", "Edit", "Bash", "Glob", "Grep"],
    },
    "reviewer": {
        "allowed_tools": ["Read", "Glob", "Grep", "Bash"],
        "disallowed_tools": ["Write", "Edit"],
    },
    "quality_reviewer": {
        "allowed_tools": ["Read", "Glob", "Grep", "Bash"],
        "disallowed_tools": ["Write", "Edit"],
    },
    "spec_generator": {
        "allowed_tools": ["Read", "Write", "Glob", "Grep"],
        "disallowed_tools": ["Bash"],
    },
    "punchlist": {
        "allowed_tools": ["Read", "Glob", "Grep"],
        "disallowed_tools": ["Write", "Edit", "Bash"],
    },
    "recon": {
        "allowed_tools": ["Read", "Glob", "Grep"],
        "disallowed_tools": ["Write", "Edit", "Bash"],
    },
}


def _is_protected(file_path: str, protected_paths: list[str]) -> str | None:
    """Return the first matching protected path if file_path is protected, else None.

    Matching rules:
    - Exact match: file_path == protected_path (ignoring trailing slash on protected_path)
    - Directory prefix: file_path starts with protected_path (with trailing slash normalised)
    """
    for p in protected_paths:
        # Normalise: strip trailing slash for prefix comparison
        prefix = p.rstrip("/")
        if file_path == prefix:
            return p
        if file_path.startswith(prefix + "/"):
            return p
    return None


def make_protected_path_hook(protected_paths: list[str]):
    """Return an async PreToolUse hook that blocks writes to protected paths.

    For Write/Edit tools: checks tool_input.file_path against protected paths.
    For Bash tool: heuristically checks if the command string references any
    protected path (catches direct references like ``echo > CLAUDE.md``).
    """

    async def hook(hook_input: PreToolUseHookInput, matcher: str | None, ctx) -> dict:
        tool_name = hook_input.get("tool_name", "")
        tool_input = hook_input.get("tool_input", {})

        if tool_name in ("Write", "Edit"):
            file_path = tool_input.get("file_path", "")
            if not file_path:
                return {}
            matched = _is_protected(file_path, protected_paths)
            if matched is None:
                return {}
            return {
                "decision": "block",
                "systemMessage": (
                    f"Cannot modify protected path: {file_path}. "
                    f"This path is listed in GODARK_PROTECTED_PATHS ({matched!r}) "
                    "and must not be modified by implementing agents. "
                    "Please adjust your approach to avoid writing to this path."
                ),
            }

        if tool_name == "Bash":
            command = tool_input.get("command", "")
            if not command:
                return {}
            for p in protected_paths:
                prefix = p.rstrip("/")
                if prefix in command:
                    return {
                        "decision": "block",
                        "systemMessage": (
                            f"Cannot run Bash command referencing protected path: {p!r}. "
                            f"The command contains a reference to a protected path "
                            "and must not be used to modify protected files. "
                            "Please adjust your approach."
                        ),
                    }
            return {}

        return {}

    return hook


def _is_generated(file_path: str, generated_paths: list[str]) -> str | None:
    """Return the first matching generated path if file_path is generated, else None.

    Matching rules:
    - Glob pattern (entry contains '*'): fnmatch match against file_path
    - Directory prefix (no '*'): file_path starts with entry (with trailing slash normalised)
    """
    for p in generated_paths:
        if "*" in p:
            if fnmatch.fnmatch(file_path, p):
                return p
        else:
            prefix = p.rstrip("/")
            if file_path == prefix or file_path.startswith(prefix + "/"):
                return p
    return None


def make_generated_path_hook(generated_paths: list[str]):
    """Return an async PreToolUse hook that blocks writes to generated paths.

    For Write/Edit tools: checks tool_input.file_path against generated paths.
    Directory prefixes (no '*') use startswith matching; glob patterns (contain
    '*') use fnmatch matching.
    """

    async def hook(hook_input: PreToolUseHookInput, matcher: str | None, ctx) -> dict:
        tool_name = hook_input.get("tool_name", "")
        if tool_name not in ("Write", "Edit"):
            return {}

        tool_input = hook_input.get("tool_input", {})
        file_path = tool_input.get("file_path", "")
        if not file_path:
            return {}

        matched = _is_generated(file_path, generated_paths)
        if matched is None:
            return {}

        return {
            "decision": "block",
            "systemMessage": (
                "this file is generated — edit the source file or re-run code generation instead"
            ),
        }

    return hook


def make_denied_commands_hook(denied_patterns: list[str]):
    """Return an async PreToolUse hook that blocks Bash commands matching denied patterns.

    Checks the Bash tool's ``command`` input against each denied pattern using
    substring match. On match, the hook blocks the command and returns a system
    message explaining which pattern matched and why the command is blocked.
    """

    async def hook(hook_input: PreToolUseHookInput, matcher: str | None, ctx) -> dict:
        tool_name = hook_input.get("tool_name", "")
        if tool_name != "Bash":
            return {}

        tool_input = hook_input.get("tool_input", {})
        command = tool_input.get("command", "")
        if not command:
            return {}

        for pattern in denied_patterns:
            if pattern in command:
                return {
                    "decision": "block",
                    "systemMessage": (
                        f"Cannot run Bash command matching denied pattern: {pattern!r}. "
                        f"The command {command!r} contains a pattern that is blocked by "
                        "GODARK_DENIED_COMMANDS to prevent destructive operations. "
                        "Please adjust your approach to avoid this command."
                    ),
                }
        return {}

    return hook


def make_audit_hook():
    """Return an async PostToolUse hook that logs tool calls to stderr as JSON."""

    async def hook(hook_input: PostToolUseHookInput, matcher: str | None, ctx) -> dict:
        tool_name = hook_input.get("tool_name", "")
        tool_input = hook_input.get("tool_input", {})
        # Build a brief input summary (first 200 chars of JSON representation)
        try:
            input_repr = json.dumps(tool_input)
        except Exception:
            input_repr = str(tool_input)
        input_summary = input_repr[:200] + ("..." if len(input_repr) > 200 else "")

        record = {
            "tool": tool_name,
            "input_summary": input_summary,
            "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
        }
        print(json.dumps(record), file=sys.stderr, flush=True)
        return {}

    return hook


async def main() -> None:
    prompt = os.environ.get("GODARK_PROMPT", "")
    role = os.environ.get("GODARK_ROLE", "")
    session_id = os.environ.get("GODARK_SESSION_ID", "")
    work_dir = os.environ.get("GODARK_WORKDIR", "/workspace")
    protected_paths_raw = os.environ.get("GODARK_PROTECTED_PATHS", "")
    generated_paths_raw = os.environ.get("GODARK_GENERATED_PATHS", "")
    denied_commands_raw = os.environ.get("GODARK_DENIED_COMMANDS", "")

    if role not in _ROLE_PERMISSIONS:
        valid = ", ".join(sorted(_ROLE_PERMISSIONS.keys()))
        print(
            f"error: unknown GODARK_ROLE {role!r}; valid roles are: {valid}",
            file=sys.stderr,
        )
        sys.exit(1)

    permissions = _ROLE_PERMISSIONS[role]

    # Startup diagnostics — versions and environment.
    _log_diagnostic(
        "startup",
        role=role,
        sdk_version=getattr(claude_agent_sdk, "__version__", "unknown"),
        session_resume=bool(session_id),
        prompt_bytes=len(prompt),
    )

    env: dict = {}
    if os.environ.get("CLAUDE_CODE_OAUTH_TOKEN"):
        env["CLAUDE_CODE_OAUTH_TOKEN"] = os.environ["CLAUDE_CODE_OAUTH_TOKEN"]
    elif os.environ.get("ANTHROPIC_API_KEY"):
        env["ANTHROPIC_API_KEY"] = os.environ["ANTHROPIC_API_KEY"]
    if os.environ.get("GH_TOKEN"):
        env["GH_TOKEN"] = os.environ["GH_TOKEN"]

    # Build hooks: PreToolUse guard for protected paths + generated paths + denied commands + PostToolUse audit log.
    protected_paths = [p.strip() for p in protected_paths_raw.split(",") if p.strip()]
    generated_paths = [p.strip() for p in generated_paths_raw.split(",") if p.strip()]
    denied_commands = [c.strip() for c in denied_commands_raw.split(",") if c.strip()]

    pre_tool_use_hooks: list = []
    if protected_paths:
        pre_tool_use_hooks.append(
            HookMatcher(
                matcher="Write|Edit|Bash",
                hooks=[make_protected_path_hook(protected_paths)],
            )
        )
    if generated_paths:
        pre_tool_use_hooks.append(
            HookMatcher(
                matcher="Write|Edit",
                hooks=[make_generated_path_hook(generated_paths)],
            )
        )
    if denied_commands:
        pre_tool_use_hooks.append(
            HookMatcher(
                matcher="Bash",
                hooks=[make_denied_commands_hook(denied_commands)],
            )
        )

    hooks: dict = {}
    if pre_tool_use_hooks:
        hooks["PreToolUse"] = pre_tool_use_hooks
    hooks["PostToolUse"] = [
        HookMatcher(
            matcher=None,
            hooks=[make_audit_hook()],
        )
    ]

    options = ClaudeAgentOptions(
        permission_mode="bypassPermissions",
        setting_sources=["project"],
        plugins=[],
        system_prompt={"type": "preset", "preset": "claude_code"},
        cwd=work_dir,
        env=env if env else None,
        allowed_tools=permissions.get("allowed_tools") or None,
        disallowed_tools=permissions.get("disallowed_tools") or None,
        hooks=hooks,
    )

    if session_id:
        options.resume = session_id

    result_session_id = ""
    result_text = ""
    cost_usd = 0.0
    is_error = False
    tool_trace: list[str] = []
    assistant_texts: list[str] = []

    async def _collect_messages(opts):
        """Stream messages from the SDK, printing each one. Returns (session_id, result, cost, is_error)."""
        nonlocal result_session_id, result_text, cost_usd, is_error, tool_trace, assistant_texts
        result_session_id = ""
        result_text = ""
        cost_usd = 0.0
        is_error = False
        tool_trace = []
        assistant_texts = []
        async for message in claude_agent_sdk.query(prompt=prompt, options=opts):
            try:
                msg_dict = message.model_dump(mode="json") if hasattr(message, "model_dump") else vars(message)
            except Exception:
                msg_dict = {"type": type(message).__name__, "raw": str(message)}
            print(json.dumps(msg_dict, default=str), flush=True)

            # Collect text and tool-use summaries from AssistantMessage content blocks.
            msg_type = type(message).__name__
            if msg_type == "AssistantMessage":
                for block in getattr(message, "content", []):
                    block_type = type(block).__name__
                    if block_type == "TextBlock":
                        text = getattr(block, "text", "")
                        if text:
                            assistant_texts.append(text)
                    elif block_type == "ToolUseBlock":
                        tool_name = getattr(block, "name", "")
                        tool_input = getattr(block, "input", {}) or {}
                        if tool_name in ("Edit", "Write", "Read"):
                            fp = tool_input.get("file_path", "")
                            tool_trace.append(f"{tool_name} {fp}" if fp else tool_name)
                        elif tool_name == "Bash":
                            cmd = (tool_input.get("command", "") or "")[:80]
                            tool_trace.append(f"Bash: {cmd}")
                        elif tool_name:
                            tool_trace.append(tool_name)

            if isinstance(message, ResultMessage):
                result_session_id = getattr(message, "session_id", "") or ""
                result_text = getattr(message, "result", "") or ""
                cost_usd = float(getattr(message, "total_cost_usd", 0.0) or 0.0)
                is_error = bool(getattr(message, "is_error", False))

    max_retries = 3
    base_delay = 2.0  # seconds
    last_exc: Exception | None = None

    for attempt in range(max_retries + 1):
        query_start = time.monotonic()
        try:
            await _collect_messages(options)
            elapsed = time.monotonic() - query_start
            _log_diagnostic("query_success", attempt=attempt + 1, elapsed_seconds=round(elapsed, 2), tool_calls=len(tool_trace))
            break  # success
        except Exception as exc:
            elapsed = time.monotonic() - query_start
            last_exc = exc

            # Build rich error diagnostic with exception chain.
            exc_info: dict = {
                "attempt": attempt + 1,
                "elapsed_seconds": round(elapsed, 2),
                "tool_calls_before_failure": len(tool_trace),
                "exception_type": type(exc).__qualname__,
                "exception": repr(exc),
            }
            # Capture nested exceptions from ExceptionGroups.
            if hasattr(exc, "exceptions"):
                exc_info["nested_exceptions"] = [
                    {"type": type(e).__qualname__, "message": str(e)}
                    for e in exc.exceptions  # type: ignore[attr-defined]
                ]
            # Capture full traceback for first failure only (avoid log spam).
            if attempt == 0:
                exc_info["traceback"] = traceback.format_exception(exc)
            _log_diagnostic("query_failure", **exc_info)

            # Session resume failure — fall back to fresh session, then retry.
            if session_id and options.resume:
                _log_diagnostic("session_resume_fallback", reason=repr(exc))
                options.resume = None
                continue

            if attempt >= max_retries:
                raise

            # Exponential backoff with jitter: base * 2^attempt + random(0, base)
            delay = base_delay * (2 ** attempt) + random.uniform(0, base_delay)
            _log_diagnostic("retry_backoff", attempt=attempt + 1, delay_seconds=round(delay, 1))
            await asyncio.sleep(delay)

    # Use the full accumulated assistant text as the result so the dashboard
    # shows the complete agent output, not just the final sentinel line.
    full_output = "\n\n".join(assistant_texts) if assistant_texts else result_text

    # For reviewer/quality_reviewer roles, extract the verdict.
    # Check full_output (all assistant text) since the sentinel may not be
    # in the final ResultMessage alone.
    verdict = ""
    verdict_source = full_output or result_text
    if role == "reviewer" and verdict_source:
        upper = verdict_source.upper()
        for line in upper.splitlines():
            stripped = line.strip()
            if "REVIEW" in stripped and "RESULT" in stripped:
                if "CHANGES" in stripped:
                    verdict = "CHANGES_REQUESTED"
                    break
                elif "APPROVED" in stripped:
                    verdict = "APPROVED"
                    break

    if role == "quality_reviewer" and verdict_source:
        upper = verdict_source.upper()
        for line in upper.splitlines():
            stripped = line.strip()
            if "QUALITY" in stripped and "RESULT" in stripped:
                if "CHANGES" in stripped:
                    verdict = "CHANGES_REQUESTED"
                    break
                elif "APPROVED" in stripped:
                    verdict = "APPROVED"
                    break

    final: dict = {
        "session_id": result_session_id,
        "result": full_output,
        "cost_usd": cost_usd,
        "is_error": is_error,
    }
    if verdict:
        final["verdict"] = verdict
    if tool_trace:
        final["tool_trace"] = tool_trace
    print(json.dumps(final), flush=True)

    # Force-exit after printing the result. The SDK's close() can hang
    # indefinitely when the CLI subprocess blocks on shutdown (see
    # https://github.com/anthropics/claude-agent-sdk-python/issues/728).
    # The result is already on stdout for the Go side to capture.
    _log_diagnostic("force_exit", reason="result printed, bypassing SDK cleanup")
    os._exit(0)


if __name__ == "__main__":
    asyncio.run(main())
