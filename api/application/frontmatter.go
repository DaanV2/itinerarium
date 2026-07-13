package application

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// frontmatter is the YAML metadata block a markdown document may start with
// (see "Document Format" in docs/architecture.md). The format is
// Obsidian-compatible so vaults can be imported as-is.
type frontmatter struct {
	Title   string   `yaml:"title"`
	Tags    []string `yaml:"tags"`
	GameDay *int     `yaml:"game_day"`
	// Repository targets a repository by name on vault import (M6); document
	// creation inside a known repository ignores it.
	Repository string `yaml:"repository"`
}

// parseFrontmatter splits markdown into its leading YAML frontmatter and the
// body. Markdown without a frontmatter block returns a zero frontmatter and
// the input unchanged.
func parseFrontmatter(markdown string) (frontmatter, string, error) {
	var meta frontmatter

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
		return meta, "", fmt.Errorf("%w: bad frontmatter: %w", ErrInvalidDocument, err)
	}

	return meta, strings.TrimSpace(strings.TrimPrefix(body, "\n")), nil
}
