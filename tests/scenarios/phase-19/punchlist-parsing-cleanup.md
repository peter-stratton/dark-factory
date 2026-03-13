# Scenario: Clean up punchlist parsing

Relates to: Issue #398

## Setup
- `internal/agent/punchlist.go` contains `extractJSONArray()`
- `internal/punchlist/punchlist.go` contains `extractPrefixedItem()`

## Cases

### Plain JSON array extracted
Call `extractJSONArray("[\"a\",\"b\"]")`.
- Returns `("[\"a\",\"b\"]", true)`

### JSON in code fence extracted
Call `extractJSONArray("```json\n[\"a\"]\n```")`.
- Returns `("[\"a\"]", true)`

### JSON with surrounding text extracted
Call `extractJSONArray("Here are tests: [\"a\", \"b\"] done")`.
- Returns `("[\"a\", \"b\"]", true)`

### Nested brackets handled correctly
Call `extractJSONArray("text [with [nested] brackets]")`.
- Extracts `"[with [nested] brackets]"` using bracket-depth matching
- Does not incorrectly match first `[` with last `]` across unrelated content

### No JSON returns false
Call `extractJSONArray("no brackets here")`.
- Returns `("", false)`

### Checkbox with dash prefix
Call `extractPrefixedItem("- [ ] foo", "- [ ] ", "* [ ] ")`.
- Returns `("foo", true)`

### Checkbox with star prefix
Call `extractPrefixedItem("* [ ] bar", "- [ ] ", "* [ ] ")`.
- Returns `("bar", true)`

### Bullet with dash prefix
Call `extractPrefixedItem("- item", "- ", "* ")`.
- Returns `("item", true)`

### No matching prefix
Call `extractPrefixedItem("plain text", "- [ ] ", "* [ ] ")`.
- Returns `("", false)`
