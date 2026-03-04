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
    # Prefer OAuth token over API key so the SDK uses OAuth when available.
    if os.environ.get("CLAUDE_CODE_OAUTH_TOKEN"):
        env["CLAUDE_CODE_OAUTH_TOKEN"] = os.environ["CLAUDE_CODE_OAUTH_TOKEN"]
    elif os.environ.get("ANTHROPIC_API_KEY"):
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

    async for message in claude_agent_sdk.query(prompt=prompt, options=options):
        try:
            msg_dict = message.model_dump(mode="json") if hasattr(message, "model_dump") else vars(message)
        except Exception:
            msg_dict = {"type": type(message).__name__, "raw": str(message)}
        print(json.dumps(msg_dict, default=str), flush=True)

        if isinstance(message, ResultMessage):
            result_session_id = getattr(message, "session_id", "") or ""
            result_text = getattr(message, "result", "") or ""
            cost_usd = float(getattr(message, "total_cost_usd", 0.0) or 0.0)
            is_error = bool(getattr(message, "is_error", False))

    final = {
        "session_id": result_session_id,
        "result": result_text,
        "cost_usd": cost_usd,
        "is_error": is_error,
    }
    print(json.dumps(final), flush=True)


if __name__ == "__main__":
    asyncio.run(main())
