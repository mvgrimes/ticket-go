package ticket

import (
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

// headerBuf is a reusable buffer for reading ticket headers.
// Most frontmatter + title fits in 2KB.
var headerBuf = make([]byte, 2048)

// trie is a generic trie for ASCII string lookup.
type trie[T any] struct {
	children [128]*trie[T]
	value    T
	hasValue bool
}

// buildTrie constructs a trie from key-value pairs.
func buildTrie[T any](entries []struct {
	key   string
	value T
}) *trie[T] {
	root := &trie[T]{}
	for _, e := range entries {
		node := root
		for i := 0; i < len(e.key); i++ {
			c := e.key[i]
			if node.children[c] == nil {
				node.children[c] = &trie[T]{}
			}
			node = node.children[c]
		}
		node.value = e.value
		node.hasValue = true
	}
	return root
}

// fieldSetter assigns a parsed value to a ticket field.
type fieldSetter func(t *Ticket, val []byte)

// keyTrie is the pre-built trie for YAML key lookup.
var keyTrie = buildKeyTrie()

// statusTrie is the pre-built trie for status value lookup.
var statusTrie = buildStatusTrie()

// buildStatusTrie constructs the status lookup trie from ValidStatuses.
func buildStatusTrie() *trie[Status] {
	// Status is type string, so we can build trie directly
	root := &trie[Status]{}
	for _, s := range ValidStatuses {
		node := root
		for i := 0; i < len(s); i++ {
			c := s[i]
			if node.children[c] == nil {
				node.children[c] = &trie[Status]{}
			}
			node = node.children[c]
		}
		node.value = s
		node.hasValue = true
	}
	return root
}

// lookupStatus returns a pre-allocated Status constant if found, or converts val.
func lookupStatus(val []byte) Status {
	node := statusTrie
	for _, c := range val {
		if c >= 128 || node.children[c] == nil {
			return Status(val)
		}
		node = node.children[c]
	}
	if node.hasValue {
		return node.value
	}
	return Status(val)
}

// buildKeyTrie constructs the key lookup trie at init time.
// Leading whitespace loops at root; keys terminate on ':' or whitespace.
func buildKeyTrie() *trie[fieldSetter] {
	root := buildTrie([]struct {
		key   string
		value fieldSetter
	}{
		{"id", func(t *Ticket, val []byte) { t.ID = string(val) }},
		{"status", func(t *Ticket, val []byte) { t.Status = lookupStatus(val) }},
		{"deps", func(t *Ticket, val []byte) { t.Deps = parseArrayBytes(val) }},
		{"links", func(t *Ticket, val []byte) { t.Links = parseArrayBytes(val) }},
		{"created", func(t *Ticket, val []byte) { t.Created = parseTime(string(val)) }},
		{"type", func(t *Ticket, val []byte) { t.Type = string(val) }},
		{"priority", func(t *Ticket, val []byte) { t.Priority, _ = strconv.Atoi(string(val)) }},
		{"assignee", func(t *Ticket, val []byte) { t.Assignee = string(val) }},
		{"external-ref", func(t *Ticket, val []byte) { t.ExternalRef = string(val) }},
		{"parent", func(t *Ticket, val []byte) { t.Parent = string(val) }},
		{"tags", func(t *Ticket, val []byte) { t.Tags = parseArrayBytes(val) }},
	})

	// Leading whitespace stays at root
	root.children[' '] = root
	root.children['\t'] = root

	return root
}

// LoadTicketHeader loads only the frontmatter and title from a ticket file.
// Uses a single shared buffer and minimal allocations.
func LoadTicketHeader(path string) (*Ticket, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	n, err := f.Read(headerBuf)
	if err != nil {
		return nil, err
	}

	return parseHeaderBytes(headerBuf[:n])
}

// parseHeaderBytes parses ticket header from buffer with minimal allocations.
func parseHeaderBytes(data []byte) (*Ticket, error) {
	n := len(data)
	if n < 4 {
		return nil, fmt.Errorf("file too small")
	}

	// Check opening "---\n"
	if data[0] != '-' || data[1] != '-' || data[2] != '-' || data[3] != '\n' {
		return nil, fmt.Errorf("missing frontmatter")
	}

	// Find closing "\n---\n" using simple scan
	fmEnd := -1
	for i := 4; i < n-4; i++ {
		if data[i] == '\n' && data[i+1] == '-' && data[i+2] == '-' && data[i+3] == '-' && data[i+4] == '\n' {
			fmEnd = i
			break
		}
	}
	if fmEnd == -1 {
		return nil, fmt.Errorf("unclosed frontmatter")
	}

	t := &Ticket{
		Status: StatusOpen,
	}

	// Parse frontmatter lines in-place
	lineStart := 4
	for i := 4; i <= fmEnd; i++ {
		if i == fmEnd || data[i] == '\n' {
			if i > lineStart {
				parseHeaderLine(data[lineStart:i], t)
			}
			lineStart = i + 1
		}
	}

	// Find title after frontmatter (starts after "\n---\n")
	bodyStart := fmEnd + 5
	for i := bodyStart; i < n-2; i++ {
		if data[i] == '#' && data[i+1] == ' ' {
			t.Title = extractLine(data, i+2, n)
			break
		}
		if data[i] == '\n' && i+2 < n && data[i+1] == '#' && data[i+2] == ' ' {
			t.Title = extractLine(data, i+3, n)
			break
		}
	}

	return t, nil
}

// extractLine extracts a string from start until newline or end.
func extractLine(data []byte, start, end int) string {
	lineEnd := start
	for lineEnd < end && data[lineEnd] != '\n' {
		lineEnd++
	}
	return string(data[start:lineEnd])
}

// parseHeaderLine parses a single YAML line into ticket fields.
// The trie handles leading whitespace via self-loops at root.
// Key matching terminates when we can't advance and hit ':' or whitespace.
func parseHeaderLine(line []byte, t *Ticket) {
	n := len(line)
	node := keyTrie
	i := 0

	// Walk trie to match key
	for i < n {
		c := line[i]
		if c >= 128 {
			return
		}
		next := node.children[c]
		if next == nil {
			// Can't advance - check if we completed a key
			if node.hasValue && (c == ':' || c == ' ' || c == '\t') {
				break
			}
			return
		}
		node = next
		i++
	}

	if !node.hasValue || i >= n {
		return
	}

	// Skip to colon
	for i < n && line[i] != ':' {
		i++
	}
	if i >= n {
		return
	}
	i++ // skip colon

	// Trim value: skip leading whitespace
	for i < n && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	// Trim trailing whitespace
	end := n
	for end > i && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}

	node.value(t, line[i:end])
}

// trimBytes trims leading and trailing whitespace from a byte slice.
func trimBytes(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}

// parseArrayBytes parses a YAML array like "[a, b, c]" from bytes.
func parseArrayBytes(val []byte) []string {
	if len(val) < 2 || val[0] != '[' || val[len(val)-1] != ']' {
		return []string{}
	}
	inner := val[1 : len(val)-1]
	if len(inner) == 0 {
		return []string{}
	}

	// Count commas to pre-allocate
	count := 1
	for _, b := range inner {
		if b == ',' {
			count++
		}
	}

	result := make([]string, 0, count)
	start := 0
	for i := 0; i <= len(inner); i++ {
		if i == len(inner) || inner[i] == ',' {
			elem := trimBytes(inner[start:i])
			if len(elem) > 0 {
				result = append(result, string(elem))
			}
			start = i + 1
		}
	}
	return result
}

// ParseTicket parses a ticket from markdown content with YAML frontmatter.
func ParseTicket(content string) (*Ticket, error) {
	t := &Ticket{
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

