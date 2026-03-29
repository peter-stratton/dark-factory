"""Tests for agent role permissions."""

import unittest

from agent_runner import _ROLE_PERMISSIONS, get_role_permissions


class TestMergeCoordinatorRole(unittest.TestCase):
    def test_role_exists(self):
        self.assertIn("merge_coordinator", _ROLE_PERMISSIONS)

    def test_allowed_tools(self):
        perms = _ROLE_PERMISSIONS["merge_coordinator"]
        self.assertEqual(
            perms["allowed_tools"],
            ["Read", "Edit", "Bash", "Glob", "Grep"],
        )

    def test_disallowed_tools(self):
        perms = _ROLE_PERMISSIONS["merge_coordinator"]
        self.assertEqual(perms["disallowed_tools"], ["Write"])

    def test_get_role_permissions_returns_dict(self):
        perms = get_role_permissions("merge_coordinator")
        self.assertIsNotNone(perms)
        self.assertIn("allowed_tools", perms)

    def test_get_role_permissions_unknown_role(self):
        self.assertIsNone(get_role_permissions("nonexistent"))


if __name__ == "__main__":
    unittest.main()
