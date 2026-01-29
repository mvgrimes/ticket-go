package ticket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTicket(t *testing.T) {
	content := `---
id: test-001
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
assignee: John Doe
---
# Test Ticket Title

This is the description.
`

	ticket, err := ParseTicket(content)
	if err != nil {
		t.Fatalf("ParseTicket error: %v", err)
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"ID", ticket.ID, "test-001"},
		{"Status", ticket.Status, StatusOpen},
		{"Type", ticket.Type, "task"},
		{"Priority", ticket.Priority, 2},
		{"Assignee", ticket.Assignee, "John Doe"},
		{"Title", ticket.Title, "Test Ticket Title"},
		{"Deps length", len(ticket.Deps), 0},
		{"Links length", len(ticket.Links), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	// Check created time
	expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !ticket.Created.Equal(expectedTime) {
		t.Errorf("Created = %v, want %v", ticket.Created, expectedTime)
	}
}

func TestParseTicket_WithDeps(t *testing.T) {
	content := `---
id: test-001
status: open
deps: [dep-001, dep-002]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# Test
`

	ticket, err := ParseTicket(content)
	if err != nil {
		t.Fatalf("ParseTicket error: %v", err)
	}

	if len(ticket.Deps) != 2 {
		t.Errorf("Deps length = %d, want 2", len(ticket.Deps))
	}

	if ticket.Deps[0] != "dep-001" || ticket.Deps[1] != "dep-002" {
		t.Errorf("Deps = %v, want [dep-001, dep-002]", ticket.Deps)
	}
}

func TestParseTicket_WithLinks(t *testing.T) {
	content := `---
id: test-001
status: open
deps: []
links: [link-001, link-002, link-003]
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# Test
`

	ticket, err := ParseTicket(content)
	if err != nil {
		t.Fatalf("ParseTicket error: %v", err)
	}

	if len(ticket.Links) != 3 {
		t.Errorf("Links length = %d, want 3", len(ticket.Links))
	}
}

func TestParseTicket_WithTags(t *testing.T) {
	content := `---
id: test-001
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
tags: [ui, backend, urgent]
---
# Test
`

	ticket, err := ParseTicket(content)
	if err != nil {
		t.Fatalf("ParseTicket error: %v", err)
	}

	if len(ticket.Tags) != 3 {
		t.Errorf("Tags length = %d, want 3", len(ticket.Tags))
	}

	expected := []string{"ui", "backend", "urgent"}
	for i, tag := range expected {
		if ticket.Tags[i] != tag {
			t.Errorf("Tags[%d] = %q, want %q", i, ticket.Tags[i], tag)
		}
	}
}

func TestParseTicket_WithParent(t *testing.T) {
	content := `---
id: child-001
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
parent: parent-001
---
# Child Ticket
`

	ticket, err := ParseTicket(content)
	if err != nil {
		t.Fatalf("ParseTicket error: %v", err)
	}

	if ticket.Parent != "parent-001" {
		t.Errorf("Parent = %q, want %q", ticket.Parent, "parent-001")
	}
}

func TestParseTicket_WithExternalRef(t *testing.T) {
	content := `---
id: test-001
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
external-ref: JIRA-123
---
# Test
`

	ticket, err := ParseTicket(content)
	if err != nil {
		t.Fatalf("ParseTicket error: %v", err)
	}

	if ticket.ExternalRef != "JIRA-123" {
		t.Errorf("ExternalRef = %q, want %q", ticket.ExternalRef, "JIRA-123")
	}
}

func TestParseTicket_MissingFrontmatter(t *testing.T) {
	content := `# No Frontmatter
Just content.
`

	_, err := ParseTicket(content)
	if err == nil {
		t.Error("Expected error for missing frontmatter")
	}
}

func TestParseTicket_UnclosedFrontmatter(t *testing.T) {
	content := `---
id: test-001
status: open
# Missing closing ---
`

	_, err := ParseTicket(content)
	if err == nil {
		t.Error("Expected error for unclosed frontmatter")
	}
}

func TestParseTicket_AllStatuses(t *testing.T) {
	statuses := []Status{StatusOpen, StatusInProgress, StatusClosed}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			content := `---
id: test-001
status: ` + string(status) + `
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# Test
`
			ticket, err := ParseTicket(content)
			if err != nil {
				t.Fatalf("ParseTicket error: %v", err)
			}

			if ticket.Status != status {
				t.Errorf("Status = %v, want %v", ticket.Status, status)
			}
		})
	}
}

func TestSaveTicket(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-001.md")

	ticket := &Ticket{
		ID:          "test-001",
		Status:      StatusOpen,
		Deps:        []string{"dep-001"},
		Links:       []string{"link-001"},
		Created:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Type:        "bug",
		Priority:    1,
		Assignee:    "Jane Doe",
		ExternalRef: "GH-456",
		Parent:      "parent-001",
		Tags:        []string{"critical", "frontend"},
		Title:       "Test Bug",
		Description: "Bug description here.",
		Design:      "Design notes.",
		Acceptance:  "Should work correctly.",
	}

	err := SaveTicket(ticket, path)
	if err != nil {
		t.Fatalf("SaveTicket error: %v", err)
	}

	// Read back and parse
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	contentStr := string(content)

	// Check all fields are present
	checks := []string{
		"id: test-001",
		"status: open",
		"deps: [dep-001]",
		"links: [link-001]",
		"type: bug",
		"priority: 1",
		"assignee: Jane Doe",
		"external-ref: GH-456",
		"parent: parent-001",
		"tags: [critical, frontend]",
		"# Test Bug",
		"Bug description here.",
		"## Design",
		"Design notes.",
		"## Acceptance Criteria",
		"Should work correctly.",
	}

	for _, check := range checks {
		if !strings.Contains(contentStr, check) {
			t.Errorf("SaveTicket output missing %q", check)
		}
	}
}

func TestSaveAndLoadTicket_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "roundtrip.md")

	original := &Ticket{
		ID:       "rt-001",
		Status:   StatusInProgress,
		Deps:     []string{"dep-a", "dep-b"},
		Links:    []string{"link-x"},
		Created:  time.Date(2024, 6, 20, 14, 0, 0, 0, time.UTC),
		Type:     "feature",
		Priority: 0,
		Assignee: "Developer",
		Title:    "Round Trip Test",
	}

	err := SaveTicket(original, path)
	if err != nil {
		t.Fatalf("SaveTicket error: %v", err)
	}

	loaded, err := LoadTicket(path)
	if err != nil {
		t.Fatalf("LoadTicket error: %v", err)
	}

	// Compare fields
	if loaded.ID != original.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, original.ID)
	}
	if loaded.Status != original.Status {
		t.Errorf("Status = %v, want %v", loaded.Status, original.Status)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type = %q, want %q", loaded.Type, original.Type)
	}
	if loaded.Priority != original.Priority {
		t.Errorf("Priority = %d, want %d", loaded.Priority, original.Priority)
	}
	if loaded.Assignee != original.Assignee {
		t.Errorf("Assignee = %q, want %q", loaded.Assignee, original.Assignee)
	}
	if loaded.Title != original.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, original.Title)
	}
	if len(loaded.Deps) != len(original.Deps) {
		t.Errorf("Deps length = %d, want %d", len(loaded.Deps), len(original.Deps))
	}
	if len(loaded.Links) != len(original.Links) {
		t.Errorf("Links length = %d, want %d", len(loaded.Links), len(original.Links))
	}
}

func TestUpdateYAMLField(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "update.md")

	content := `---
id: test-001
status: open
deps: []
priority: 2
---
# Test
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Update status
	err = UpdateYAMLField(path, "status", "closed")
	if err != nil {
		t.Fatalf("UpdateYAMLField error: %v", err)
	}

	// Verify
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !strings.Contains(string(updated), "status: closed") {
		t.Error("Status not updated to closed")
	}

	// Update priority
	err = UpdateYAMLField(path, "priority", "0")
	if err != nil {
		t.Fatalf("UpdateYAMLField error: %v", err)
	}

	updated, _ = os.ReadFile(path)
	if !strings.Contains(string(updated), "priority: 0") {
		t.Error("Priority not updated to 0")
	}
}

func TestUpdateYAMLField_AddNew(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "addfield.md")

	content := `---
id: test-001
status: open
---
# Test
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Add new field
	err = UpdateYAMLField(path, "assignee", "New Person")
	if err != nil {
		t.Fatalf("UpdateYAMLField error: %v", err)
	}

	updated, _ := os.ReadFile(path)
	if !strings.Contains(string(updated), "assignee: New Person") {
		t.Error("New field not added")
	}
}

func TestGetYAMLField(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "getfield.md")

	content := `---
id: test-001
status: in_progress
priority: 3
assignee: Test User
---
# Test
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	tests := []struct {
		field string
		want  string
	}{
		{"id", "test-001"},
		{"status", "in_progress"},
		{"priority", "3"},
		{"assignee", "Test User"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, err := GetYAMLField(path, tt.field)
			if err != nil {
				t.Fatalf("GetYAMLField error: %v", err)
			}
			if got != tt.want {
				t.Errorf("GetYAMLField(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestParseArray(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"[]", []string{}},
		{"[a]", []string{"a"}},
		{"[a, b]", []string{"a", "b"}},
		{"[a, b, c]", []string{"a", "b", "c"}},
		{"[  a  ,  b  ]", []string{"a", "b"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseArray(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseArray(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseArray(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
