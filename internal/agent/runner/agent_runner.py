"""Agent role permissions for the Claude Code harness.

Each role maps to the set of tools the agent is allowed or explicitly
disallowed from using during a run.
"""

_ROLE_PERMISSIONS = {
    "merge_coordinator": {
        "allowed_tools": ["Read", "Edit", "Bash", "Glob", "Grep"],
        # Write (create new files) is disallowed — conflict resolution only
        # edits existing files. Bash is allowed for git operations.
        "disallowed_tools": ["Write"],
    },
}


def get_role_permissions(role):
    """Return the permissions dict for the given role, or None if unknown."""
    return _ROLE_PERMISSIONS.get(role)
