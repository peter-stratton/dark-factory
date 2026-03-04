package prompts

import "embed"

// FS embeds the prompt template files so the binary is self-contained.
// Projects can override individual prompts via the config file.
//
//go:embed *.txt
var FS embed.FS
