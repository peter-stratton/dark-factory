package templates

import "embed"

// FS holds all embedded harness document templates.
// Callers may read files directly from FS when they need content that
// WriteIfNotExists would skip (e.g. godark new writing CLAUDE.md into
// projects where the file already exists from godark init).
//
//go:embed architecture.md architecture.json conventions.md roadmap.md claude.md godark.md gitignore
var FS embed.FS
