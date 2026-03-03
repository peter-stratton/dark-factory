package patterns

import "regexp"

// IssueRef matches "#N" references (e.g., #1, #42).
var IssueRef = regexp.MustCompile(`#(\d+)`)

// Phase matches "Phase N" at the start of a milestone title (case-insensitive).
var Phase = regexp.MustCompile(`(?i)^phase\s+(\d+)`)
