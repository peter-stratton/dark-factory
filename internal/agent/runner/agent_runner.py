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
  GH_TOKEN               GitHub token forwarded to the agent environment
"""

import asyncio
import json
import os
import sys

import claude_agent_sdk
from claude_agent_sdk import ClaudeAgentOptions, ResultMessage

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


async def main() -> None:
    prompt = os.environ.get("GODARK_PROMPT", "")
    role = os.environ.get("GODARK_ROLE", "")
    session_id = os.environ.get("GODARK_SESSION_ID", "")
    work_dir = os.environ.get("GODARK_WORKDIR", "/workspace")

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

    options = ClaudeAgentOptions(
        permission_mode="bypassPermissions",
        setting_sources=["project"],
        system_prompt={"type": "preset", "preset": "claude_code"},
        cwd=work_dir,
        env=env if env else None,
        allowed_tools=permissions.get("allowed_tools"),
        disallowed_tools=permissions.get("disallowed_tools"),
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
