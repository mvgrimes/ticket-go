package ticket

import (
	"os"
	"strings"
	"testing"
)

func TestConvertToJQExpr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Simple field=value syntax
		{"status=open", ".[] | select(.status == \"open\")"},
		{"priority<2", ".[] | select(.priority < 2)"},
		{"priority<=2", ".[] | select(.priority <= 2)"},
		{"priority>1", ".[] | select(.priority > 1)"},
		{"priority>=1", ".[] | select(.priority >= 1)"},
		{"status!=closed", ".[] | select(.status != \"closed\")"},

		// jq expressions starting with "."
		{".status == \"open\"", ".[] | select(.status == \"open\")"},
		{".priority < 2", ".[] | select(.priority < 2)"},
		{".status", ".[] | .status"},

		// Already wrapped in select
		{"select(.status == \"open\")", ".[] | select(.status == \"open\")"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ConvertToJQExpr(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertToJQExpr(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestQueryTicketsFiltered(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Create test tickets
	t1, err := store.Create(CreateOptions{Title: "Open task", Type: "task"})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	t2, err := store.Create(CreateOptions{Title: "Closed task", Type: "task"})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}
	store.UpdateField(t2.ID, "status", "closed")

	t3, err := store.Create(CreateOptions{Title: "High priority", Type: "bug", Priority: 0, PrioritySet: true})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Test: no filter returns all
	result, err := store.QueryTicketsFiltered("")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered('') failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 tickets, got %d", len(result))
	}

	// Test: filter by status
	result, err = store.QueryTicketsFiltered("status=open")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered('status=open') failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 open tickets, got %d", len(result))
	}

	// Test: filter by priority
	result, err = store.QueryTicketsFiltered("priority<1")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered('priority<1') failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 ticket with priority<1, got %d", len(result))
	}
	if len(result) > 0 && result[0].ID != t3.ID {
		t.Errorf("expected ticket %s, got %s", t3.ID, result[0].ID)
	}

	// Test: filter by type
	result, err = store.QueryTicketsFiltered("type=bug")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered('type=bug') failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 bug ticket, got %d", len(result))
	}

	// Test: jq expression
	result, err = store.QueryTicketsFiltered(".status == \"closed\"")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered('.status == \"closed\"') failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 closed ticket, got %d", len(result))
	}
	if len(result) > 0 && result[0].ID != t2.ID {
		t.Errorf("expected ticket %s, got %s", t2.ID, result[0].ID)
	}

	_ = t1 // silence unused variable warning
}

func TestQueryTicketsFilteredEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Ensure directory exists
	os.MkdirAll(dir, 0755)

	result, err := store.QueryTicketsFiltered("")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered('') on empty store failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(result))
	}
}

func TestQueryTicketsFilteredInvalid(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Create a ticket so directory exists
	_, err := store.Create(CreateOptions{Title: "Test"})
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Invalid jq expression
	_, err = store.QueryTicketsFiltered("invalid[[[")
	if err == nil {
		t.Error("expected error for invalid jq expression")
	}
}

func TestTicketJSONConversion(t *testing.T) {
	tj := TicketJSON{
		ID:       "test-001",
		Status:   "open",
		Deps:     []string{"dep-1", "dep-2"},
		Links:    []string{},
		Created:  "2024-01-01T00:00:00Z",
		Type:     "task",
		Priority: 2,
		Assignee: "alice",
		Tags:     []string{"urgent", "backend"},
	}

	// Convert to map and back
	m := ticketJSONToMap(tj)

	// Check fields
	if m["id"] != "test-001" {
		t.Errorf("expected id=test-001, got %v", m["id"])
	}
	if m["status"] != "open" {
		t.Errorf("expected status=open, got %v", m["status"])
	}
	if m["priority"] != 2 {
		t.Errorf("expected priority=2, got %v", m["priority"])
	}

	// Convert back
	result, err := mapToTicketJSON(m)
	if err != nil {
		t.Fatalf("mapToTicketJSON failed: %v", err)
	}

	if result.ID != tj.ID {
		t.Errorf("ID mismatch: %s != %s", result.ID, tj.ID)
	}
	if result.Status != tj.Status {
		t.Errorf("Status mismatch: %s != %s", result.Status, tj.Status)
	}
	if result.Priority != tj.Priority {
		t.Errorf("Priority mismatch: %d != %d", result.Priority, tj.Priority)
	}
	if len(result.Deps) != len(tj.Deps) {
		t.Errorf("Deps length mismatch: %d != %d", len(result.Deps), len(tj.Deps))
	}
}

func TestFormatShowInfo(t *testing.T) {
	// Create test ticket content
	dir := t.TempDir()
	store := NewStore(dir)

	parent, _ := store.Create(CreateOptions{Title: "Parent ticket"})
	child, _ := store.Create(CreateOptions{Title: "Child ticket", Parent: parent.ID})

	// Get show info for parent
	info, err := store.GetShowInfo(parent.ID)
	if err != nil {
		t.Fatalf("GetShowInfo failed: %v", err)
	}

	output := FormatShowInfo(info)

	// Should contain the title
	if !strings.Contains(output, "Parent ticket") {
		t.Error("output should contain 'Parent ticket'")
	}

	// Should have children section
	if !strings.Contains(output, "## Children") {
		t.Error("output should contain '## Children' section")
	}
	if !strings.Contains(output, child.ID) {
		t.Errorf("output should contain child ID %s", child.ID)
	}

	_ = dir // silence unused
}
