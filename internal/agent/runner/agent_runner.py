#!/usr/bin/env python3
"""Agent runner for dark-factory.

Reads configuration from environment variables and invokes the Claude Agent SDK.
Streams all messages to stdout as newline-delimited JSON and prints a final
structured result line.

Environment variables:
  GODARK_PROMPT          The prompt text to send to the agent
  GODARK_ROLE            Agent role: implementer, reviewer, or spec_generator
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


async def main() -> None:
    prompt = os.environ.get("GODARK_PROMPT", "")
    session_id = os.environ.get("GODARK_SESSION_ID", "")
    work_dir = os.environ.get("GODARK_WORKDIR", "/workspace")

    env: dict = {}
    for key in ("GH_TOKEN", "ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"):
        if key in os.environ:
            env[key] = os.environ[key]

    options = ClaudeAgentOptions(
        permission_mode="bypassPermissions",
        setting_sources=["project"],
        system_prompt={"type": "preset", "preset": "claude_code"},
        cwd=work_dir,
        env=env if env else None,
    )

    if session_id:
        options.resume = session_id

    result_session_id = ""
    result_text = ""
    cost_usd = 0.0
    is_error = False

    async for message in claude_agent_sdk.query(prompt=prompt, options=options):
        try:
            msg_dict = message.model_dump() if hasattr(message, "model_dump") else vars(message)
        except Exception:
            msg_dict = {"type": type(message).__name__, "raw": str(message)}
        print(json.dumps(msg_dict), flush=True)

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
