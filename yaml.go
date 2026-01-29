package ticket

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LoadTicket loads a ticket from a markdown file with YAML frontmatter.
func LoadTicket(path string) (*Ticket, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseTicket(string(content))
}

// ParseTicket parses a ticket from markdown content with YAML frontmatter.
func ParseTicket(content string) (*Ticket, error) {
	t := &Ticket{
		Deps:   []string{},
		Links:  []string{},
		Tags:   []string{},
		Status: StatusOpen,
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("missing frontmatter")
	}

	// Find end of frontmatter
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, fmt.Errorf("unclosed frontmatter")
	}

	// Parse frontmatter
	for i := 1; i < endIdx; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "id":
			t.ID = value
		case "status":
			t.Status = Status(value)
		case "deps":
			t.Deps = parseArray(value)
		case "links":
			t.Links = parseArray(value)
		case "created":
			t.Created = parseTime(value)
		case "type":
			t.Type = value
		case "priority":
			if p, err := strconv.Atoi(value); err == nil {
				t.Priority = p
			}
		case "assignee":
			t.Assignee = value
		case "external-ref":
			t.ExternalRef = value
		case "parent":
			t.Parent = value
		case "tags":
			t.Tags = parseArray(value)
		}
	}

	// Parse body
	body := strings.Join(lines[endIdx+1:], "\n")
	t.Body = body

	// Extract title from first # heading
	for _, line := range lines[endIdx+1:] {
		if strings.HasPrefix(line, "# ") {
			t.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	return t, nil
}

func parseArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "[]" || s == "" {
		return []string{}
	}

	// Remove brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	// Split by comma
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05Z", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SaveTicket saves a ticket to a markdown file with YAML frontmatter.
func SaveTicket(t *Ticket, path string) error {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("status: %s\n", t.Status))
	sb.WriteString(fmt.Sprintf("deps: %s\n", formatArray(t.Deps)))
	sb.WriteString(fmt.Sprintf("links: %s\n", formatArray(t.Links)))
	sb.WriteString(fmt.Sprintf("created: %s\n", t.Created.Format("2006-01-02T15:04:05Z")))
	sb.WriteString(fmt.Sprintf("type: %s\n", t.Type))
	sb.WriteString(fmt.Sprintf("priority: %d\n", t.Priority))

	if t.Assignee != "" {
		sb.WriteString(fmt.Sprintf("assignee: %s\n", t.Assignee))
	}
	if t.ExternalRef != "" {
		sb.WriteString(fmt.Sprintf("external-ref: %s\n", t.ExternalRef))
	}
	if t.Parent != "" {
		sb.WriteString(fmt.Sprintf("parent: %s\n", t.Parent))
	}
	if len(t.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(t.Tags, ", ")))
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("# %s\n\n", t.Title))

	if t.Description != "" {
		sb.WriteString(t.Description)
		sb.WriteString("\n\n")
	}

	if t.Design != "" {
		sb.WriteString("## Design\n\n")
		sb.WriteString(t.Design)
		sb.WriteString("\n\n")
	}

	if t.Acceptance != "" {
		sb.WriteString("## Acceptance Criteria\n\n")
		sb.WriteString(t.Acceptance)
		sb.WriteString("\n\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// UpdateYAMLField updates a single field in the YAML frontmatter.
func UpdateYAMLField(path, field, value string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("missing frontmatter")
	}

	// Find end of frontmatter and the field line
	endIdx := -1
	fieldIdx := -1
	fieldPattern := regexp.MustCompile(fmt.Sprintf(`^%s:`, regexp.QuoteMeta(field)))

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
		if fieldPattern.MatchString(lines[i]) {
			fieldIdx = i
		}
	}

	if endIdx == -1 {
		return fmt.Errorf("unclosed frontmatter")
	}

	if fieldIdx != -1 {
		// Update existing field
		lines[fieldIdx] = fmt.Sprintf("%s: %s", field, value)
	} else {
		// Insert new field after opening ---
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[0])
		newLines = append(newLines, fmt.Sprintf("%s: %s", field, value))
		newLines = append(newLines, lines[1:]...)
		lines = newLines
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// GetYAMLField extracts a single field value from YAML frontmatter.
func GetYAMLField(path, field string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFront := false
	fieldPrefix := field + ":"

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "---" {
			if inFront {
				break // End of frontmatter
			}
			inFront = true
			continue
		}

		if inFront && strings.HasPrefix(line, fieldPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, fieldPrefix)), nil
		}
	}

	return "", scanner.Err()
}
