"""Unit tests for the hook logic in agent_runner.py."""

import asyncio
import io
import json
import sys
import unittest
from unittest.mock import patch

# Import helpers directly from agent_runner
from agent_runner import _is_protected, make_audit_hook, make_protected_path_hook


def run(coro):
    """Run a coroutine synchronously."""
    return asyncio.get_event_loop().run_until_complete(coro)


class TestIsProtected(unittest.TestCase):
    """Tests for the _is_protected path-matching helper."""

    def test_exact_match(self):
        result = _is_protected("CLAUDE.md", ["CLAUDE.md", "tests/scenarios/"])
        self.assertEqual(result, "CLAUDE.md")

    def test_directory_prefix_match(self):
        result = _is_protected(
            "tests/scenarios/foo.md", ["CLAUDE.md", "tests/scenarios/"]
        )
        self.assertEqual(result, "tests/scenarios/")

    def test_directory_prefix_match_without_trailing_slash(self):
        # Protected path listed without trailing slash should still match files inside it
        result = _is_protected(
            "tests/scenarios/bar.md", ["tests/scenarios"]
        )
        self.assertEqual(result, "tests/scenarios")

    def test_non_protected_path_returns_none(self):
        result = _is_protected(
            "internal/foo/bar.go", ["CLAUDE.md", "tests/scenarios/"]
        )
        self.assertIsNone(result)

    def test_empty_protected_paths(self):
        result = _is_protected("CLAUDE.md", [])
        self.assertIsNone(result)

    def test_partial_prefix_not_matched(self):
        # "tests/scenarios_extra/file.md" should NOT match "tests/scenarios/"
        result = _is_protected(
            "tests/scenarios_extra/file.md", ["tests/scenarios/"]
        )
        self.assertIsNone(result)

    def test_exact_match_with_trailing_slash_in_protected(self):
        # A protected dir path without a filename should not match just the dir itself
        # as a file path, but exact normalisation: strip trailing slash → "tests/scenarios"
        # so "tests/scenarios" matches "tests/scenarios" (exact)
        result = _is_protected("tests/scenarios", ["tests/scenarios/"])
        self.assertEqual(result, "tests/scenarios/")


class TestProtectedPathHook(unittest.TestCase):
    """Tests for the make_protected_path_hook PreToolUse hook."""

    def _call(self, hook, tool_input: dict) -> dict:
        hook_input = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Write",
            "tool_input": tool_input,
            "tool_use_id": "tu_001",
            "session_id": "sess_001",
            "transcript_path": "/tmp/transcript",
            "cwd": "/workspace",
        }
        return run(hook(hook_input, "Write|Edit", None))

    def test_exact_path_blocked(self):
        hook = make_protected_path_hook(["CLAUDE.md", "tests/scenarios/"])
        result = self._call(hook, {"file_path": "CLAUDE.md"})
        self.assertEqual(result.get("decision"), "block")
        self.assertIn("CLAUDE.md", result.get("systemMessage", ""))

    def test_directory_prefix_blocked(self):
        hook = make_protected_path_hook(["tests/scenarios/"])
        result = self._call(hook, {"file_path": "tests/scenarios/foo.md"})
        self.assertEqual(result.get("decision"), "block")
        self.assertIn("tests/scenarios/foo.md", result.get("systemMessage", ""))

    def test_non_protected_path_allowed(self):
        hook = make_protected_path_hook(["CLAUDE.md", "tests/scenarios/"])
        result = self._call(hook, {"file_path": "internal/foo/bar.go"})
        self.assertNotEqual(result.get("decision"), "block")

    def test_system_message_injected(self):
        hook = make_protected_path_hook(["CLAUDE.md"])
        result = self._call(hook, {"file_path": "CLAUDE.md"})
        self.assertEqual(result.get("decision"), "block")
        self.assertIsNotNone(result.get("systemMessage"))
        self.assertGreater(len(result["systemMessage"]), 0)

    def test_empty_protected_paths_allows_all(self):
        hook = make_protected_path_hook([])
        result = self._call(hook, {"file_path": "CLAUDE.md"})
        self.assertNotEqual(result.get("decision"), "block")

    def test_missing_file_path_allowed(self):
        hook = make_protected_path_hook(["CLAUDE.md"])
        result = self._call(hook, {})  # no file_path key
        self.assertNotEqual(result.get("decision"), "block")


class TestAuditHook(unittest.TestCase):
    """Tests for the make_audit_hook PostToolUse hook."""

    def _call(self, hook, tool_name: str, tool_input: dict) -> tuple[dict, str]:
        """Returns (hook_result, stderr_output)."""
        hook_input = {
            "hook_event_name": "PostToolUse",
            "tool_name": tool_name,
            "tool_input": tool_input,
            "tool_response": "ok",
            "tool_use_id": "tu_002",
            "session_id": "sess_001",
            "transcript_path": "/tmp/transcript",
            "cwd": "/workspace",
        }
        buf = io.StringIO()
        with patch("sys.stderr", buf):
            result = run(hook(hook_input, None, None))
        return result, buf.getvalue()

    def test_writes_json_to_stderr(self):
        hook = make_audit_hook()
        _, stderr = self._call(hook, "Bash", {"command": "go test ./..."})
        self.assertTrue(stderr.strip(), "Expected non-empty stderr output")
        record = json.loads(stderr.strip())
        self.assertIn("tool", record)
        self.assertIn("input_summary", record)
        self.assertIn("timestamp", record)

    def test_tool_name_in_log(self):
        hook = make_audit_hook()
        _, stderr = self._call(hook, "Write", {"file_path": "foo.go", "content": "x"})
        record = json.loads(stderr.strip())
        self.assertEqual(record["tool"], "Write")

    def test_input_summary_in_log(self):
        hook = make_audit_hook()
        _, stderr = self._call(hook, "Read", {"file_path": "bar.go"})
        record = json.loads(stderr.strip())
        self.assertIn("bar.go", record["input_summary"])

    def test_long_input_truncated(self):
        long_content = "x" * 500
        hook = make_audit_hook()
        _, stderr = self._call(hook, "Write", {"file_path": "f.go", "content": long_content})
        record = json.loads(stderr.strip())
        # input_summary should be truncated to at most ~203 chars (200 + "...")
        self.assertLessEqual(len(record["input_summary"]), 203)

    def test_returns_empty_dict(self):
        hook = make_audit_hook()
        result, _ = self._call(hook, "Glob", {"pattern": "*.go"})
        self.assertEqual(result, {})


if __name__ == "__main__":
    unittest.main()
