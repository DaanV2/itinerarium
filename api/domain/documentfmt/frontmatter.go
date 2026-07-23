// Package documentfmt parses the Obsidian-compatible markdown frontmatter used
// by documents and journals, so vaults import as-is (see "Document Format" in
// docs/architecture.md). It is pure parsing: callers classify a parse failure
// (e.g. as an invalid-document error) in their own layer.
package documentfmt

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Frontmatter is the YAML metadata block a markdown document may start with.
type Frontmatter struct {
	Title   string   `yaml:"title"`
	Tags    []string `yaml:"tags"`
	GameDay *int     `yaml:"game_day"`
	// Repository targets a repository by name on vault import (M6); document
	// creation inside a known repository ignores it.
	Repository string `yaml:"repository"`
}

// Parse splits markdown into its leading YAML frontmatter and the body.
// Markdown without a frontmatter block returns a zero Frontmatter and the input
// unchanged. A malformed block returns a wrapped parse error.
func Parse(markdown string) (Frontmatter, string, error) {
	var meta Frontmatter

	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return meta, strings.TrimSpace(markdown), nil
	}

	rest := normalized[len("---\n"):]
	block, body, found := strings.Cut(rest, "\n---")
	if !found {
		return meta, strings.TrimSpace(markdown), nil
	}

	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		return meta, "", fmt.Errorf("bad frontmatter: %w", err)
	}

	return meta, strings.TrimSpace(strings.TrimPrefix(body, "\n")), nil
}
