#!/usr/bin/env python3
"""Agent runner for dark-factory.

Reads configuration from environment variables and invokes the Claude Agent SDK.
Streams all messages to stdout as newline-delimited JSON and prints a final
structured result line.

Environment variables:
  GODARK_PROMPT          The prompt text to send to the agent
  GODARK_ROLE            Agent role: implementer, implementer_retry, reviewer, or spec_generator
  GODARK_SESSION_ID      Session ID for resuming a previous session
  GODARK_WORKDIR         Working directory (default: /workspace)
  GODARK_PROTECTED_PATHS Comma-separated list of protected paths (exact or dir prefix)
  GH_TOKEN               GitHub token forwarded to the agent environment
"""

import asyncio
import datetime
import json
import os
import sys

import claude_agent_sdk
from claude_agent_sdk import ClaudeAgentOptions, ResultMessage
from claude_agent_sdk.types import (
    HookMatcher,
    PostToolUseHookInput,
    PreToolUseHookInput,
)

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
    "spec_generator": {
        "allowed_tools": ["Read", "Write", "Glob", "Grep"],
        "disallowed_tools": ["Bash"],
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
    """Return an async PreToolUse hook that blocks writes to protected paths."""

    async def hook(hook_input: PreToolUseHookInput, matcher: str | None, ctx) -> dict:
        file_path = hook_input.get("tool_input", {}).get("file_path", "")
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

    if role not in _ROLE_PERMISSIONS:
        valid = ", ".join(sorted(_ROLE_PERMISSIONS.keys()))
        print(
            f"error: unknown GODARK_ROLE {role!r}; valid roles are: {valid}",
            file=sys.stderr,
        )
        sys.exit(1)

    permissions = _ROLE_PERMISSIONS[role]

    env: dict = {}
    if os.environ.get("ANTHROPIC_API_KEY"):
        env["ANTHROPIC_API_KEY"] = os.environ["ANTHROPIC_API_KEY"]
    if os.environ.get("GH_TOKEN"):
        env["GH_TOKEN"] = os.environ["GH_TOKEN"]

    # Build hooks: PreToolUse guard for protected paths + PostToolUse audit log.
    protected_paths = [p.strip() for p in protected_paths_raw.split(",") if p.strip()]

    hooks: dict = {}
    if protected_paths:
        hooks["PreToolUse"] = [
            HookMatcher(
                matcher="Write|Edit",
                hooks=[make_protected_path_hook(protected_paths)],
            )
        ]
    hooks["PostToolUse"] = [
        HookMatcher(
            matcher=None,
            hooks=[make_audit_hook()],
        )
    ]

    options = ClaudeAgentOptions(
        permission_mode="bypassPermissions",
        setting_sources=["project"],
        system_prompt={"type": "preset", "preset": "claude_code"},
        cwd=work_dir,
        env=env if env else None,
        allowed_tools=permissions.get("allowed_tools"),
        disallowed_tools=permissions.get("disallowed_tools"),
        hooks=hooks,
    )

    if session_id:
        options.resume = session_id

    result_session_id = ""
    result_text = ""
    cost_usd = 0.0
    is_error = False
    tool_trace: list[str] = []

    async def _collect_messages(opts):
        """Stream messages from the SDK, printing each one. Returns (session_id, result, cost, is_error)."""
        nonlocal result_session_id, result_text, cost_usd, is_error, tool_trace
        result_session_id = ""
        result_text = ""
        cost_usd = 0.0
        is_error = False
        tool_trace = []
        async for message in claude_agent_sdk.query(prompt=prompt, options=opts):
            try:
                msg_dict = message.model_dump(mode="json") if hasattr(message, "model_dump") else vars(message)
            except Exception:
                msg_dict = {"type": type(message).__name__, "raw": str(message)}
            print(json.dumps(msg_dict, default=str), flush=True)

            # Collect tool-use summaries from AssistantMessage content blocks.
            msg_type = type(message).__name__
            if msg_type == "AssistantMessage":
                for block in getattr(message, "content", []):
                    block_type = type(block).__name__
                    if block_type == "ToolUseBlock":
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

    try:
        await _collect_messages(options)
    except Exception as exc:
        if session_id:
            # Resume failed — fall back to a fresh session with the retry prompt.
            warning = {
                "warning": f"session resume failed, starting fresh session: {exc!r}",
            }
            print(json.dumps(warning), file=sys.stderr, flush=True)
            options.resume = None
            await _collect_messages(options)
        else:
            raise

    # For the reviewer role, extract the verdict from result_text.
    verdict = ""
    if role == "reviewer" and result_text:
        upper = result_text.upper()
        for line in upper.splitlines():
            stripped = line.strip()
            if "REVIEW" in stripped and "RESULT" in stripped:
                if "CHANGES" in stripped:
                    verdict = "CHANGES_REQUESTED"
                    break
                elif "APPROVED" in stripped:
                    verdict = "APPROVED"
                    break

    final: dict = {
        "session_id": result_session_id,
        "result": result_text,
        "cost_usd": cost_usd,
        "is_error": is_error,
    }
    if verdict:
        final["verdict"] = verdict
    if tool_trace:
        final["tool_trace"] = tool_trace
    print(json.dumps(final), flush=True)


if __name__ == "__main__":
    asyncio.run(main())
