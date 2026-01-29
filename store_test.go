package ticket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	tmpDir := t.TempDir()
	ticketsDir := filepath.Join(tmpDir, ".tickets")
	store := NewStore(ticketsDir)
	return store, tmpDir
}

func createTestTicket(t *testing.T, store *Store, id, title string) {
	t.Helper()
	content := `---
id: ` + id + `
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# ` + title + `

Description
`
	err := store.EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}

	path := store.TicketPath(id)
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func TestNewStore(t *testing.T) {
	store := NewStore("/test/path/.tickets")
	if store.Dir != "/test/path/.tickets" {
		t.Errorf("Store.Dir = %q, want %q", store.Dir, "/test/path/.tickets")
	}
}

func TestStore_EnsureDir(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	// Directory doesn't exist yet
	ticketsDir := filepath.Join(tmpDir, ".tickets")
	if _, err := os.Stat(ticketsDir); !os.IsNotExist(err) {
		t.Fatal("Tickets dir should not exist yet")
	}

	err := store.EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}

	// Now it should exist
	if _, err := os.Stat(ticketsDir); os.IsNotExist(err) {
		t.Error("EnsureDir did not create directory")
	}

	// Calling again should not error
	err = store.EnsureDir()
	if err != nil {
		t.Errorf("EnsureDir on existing dir error: %v", err)
	}
}

func TestStore_TicketPath(t *testing.T) {
	store := NewStore("/test/.tickets")

	path := store.TicketPath("abc-1234")
	want := "/test/.tickets/abc-1234.md"
	if path != want {
		t.Errorf("TicketPath = %q, want %q", path, want)
	}
}

func TestStore_ResolveID_ExactMatch(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "test-0001", "Test Ticket")

	id, err := store.ResolveID("test-0001")
	if err != nil {
		t.Fatalf("ResolveID error: %v", err)
	}
	if id != "test-0001" {
		t.Errorf("ResolveID = %q, want %q", id, "test-0001")
	}
}

func TestStore_ResolveID_PartialSuffix(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "test-0001", "Test Ticket")

	id, err := store.ResolveID("0001")
	if err != nil {
		t.Fatalf("ResolveID error: %v", err)
	}
	if id != "test-0001" {
		t.Errorf("ResolveID = %q, want %q", id, "test-0001")
	}
}

func TestStore_ResolveID_PartialPrefix(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "abc-1234", "Test Ticket")

	id, err := store.ResolveID("abc")
	if err != nil {
		t.Fatalf("ResolveID error: %v", err)
	}
	if id != "abc-1234" {
		t.Errorf("ResolveID = %q, want %q", id, "abc-1234")
	}
}

func TestStore_ResolveID_PartialSubstring(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "abc-1234", "Test Ticket")

	id, err := store.ResolveID("c-12")
	if err != nil {
		t.Fatalf("ResolveID error: %v", err)
	}
	if id != "abc-1234" {
		t.Errorf("ResolveID = %q, want %q", id, "abc-1234")
	}
}

func TestStore_ResolveID_Ambiguous(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "abc-1234", "First")
	createTestTicket(t, store, "abc-5678", "Second")

	_, err := store.ResolveID("abc")
	if err == nil {
		t.Error("Expected error for ambiguous ID")
	}
}

func TestStore_ResolveID_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)
	store.EnsureDir()

	_, err := store.ResolveID("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent ID")
	}
}

func TestStore_ResolveID_ExactMatchPrecedence(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "abc", "Short ID")
	createTestTicket(t, store, "abc-1234", "Long ID")

	id, err := store.ResolveID("abc")
	if err != nil {
		t.Fatalf("ResolveID error: %v", err)
	}
	if id != "abc" {
		t.Errorf("ResolveID = %q, want %q (exact match)", id, "abc")
	}
}

func TestStore_Load(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "load-001", "Load Test")

	ticket, err := store.Load("load-001")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if ticket.ID != "load-001" {
		t.Errorf("ID = %q, want %q", ticket.ID, "load-001")
	}
	if ticket.Title != "Load Test" {
		t.Errorf("Title = %q, want %q", ticket.Title, "Load Test")
	}
}

func TestStore_LoadByPartialID(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "partial-001", "Partial Test")

	ticket, err := store.LoadByPartialID("001")
	if err != nil {
		t.Fatalf("LoadByPartialID error: %v", err)
	}

	if ticket.ID != "partial-001" {
		t.Errorf("ID = %q, want %q", ticket.ID, "partial-001")
	}
}

func TestStore_ListTickets(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "list-001", "First")
	createTestTicket(t, store, "list-002", "Second")
	createTestTicket(t, store, "list-003", "Third")

	tickets, err := store.ListTickets()
	if err != nil {
		t.Fatalf("ListTickets error: %v", err)
	}

	if len(tickets) != 3 {
		t.Errorf("ListTickets count = %d, want 3", len(tickets))
	}
}

func TestStore_ListTickets_Empty(t *testing.T) {
	store, _ := setupTestStore(t)
	store.EnsureDir()

	tickets, err := store.ListTickets()
	if err != nil {
		t.Fatalf("ListTickets error: %v", err)
	}

	if len(tickets) != 0 {
		t.Errorf("ListTickets count = %d, want 0", len(tickets))
	}
}

func TestStore_ListTickets_NoDir(t *testing.T) {
	store := NewStore("/nonexistent/.tickets")

	tickets, err := store.ListTickets()
	if err != nil {
		t.Fatalf("ListTickets error: %v", err)
	}

	if len(tickets) != 0 {
		t.Errorf("ListTickets should return nil or empty for non-existent dir")
	}
}

func TestStore_Create(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	// Change to temp dir for ID generation
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	opts := CreateOptions{
		Title:       "New Ticket",
		Description: "Description here",
		Type:        "bug",
		Priority:    1,
		PrioritySet: true,
		Assignee:    "Tester",
	}

	ticket, err := store.Create(opts)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if ticket.Title != "New Ticket" {
		t.Errorf("Title = %q, want %q", ticket.Title, "New Ticket")
	}
	if ticket.Type != "bug" {
		t.Errorf("Type = %q, want %q", ticket.Type, "bug")
	}
	if ticket.Priority != 1 {
		t.Errorf("Priority = %d, want 1", ticket.Priority)
	}
	if ticket.Assignee != "Tester" {
		t.Errorf("Assignee = %q, want %q", ticket.Assignee, "Tester")
	}
	if ticket.Status != StatusOpen {
		t.Errorf("Status = %v, want %v", ticket.Status, StatusOpen)
	}

	// Verify file exists
	path := store.TicketPath(ticket.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Ticket file not created")
	}
}

func TestStore_Create_Defaults(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	opts := CreateOptions{}

	ticket, err := store.Create(opts)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if ticket.Title != "Untitled" {
		t.Errorf("Default title = %q, want %q", ticket.Title, "Untitled")
	}
	if ticket.Type != "task" {
		t.Errorf("Default type = %q, want %q", ticket.Type, "task")
	}
	if ticket.Priority != 2 {
		t.Errorf("Default priority = %d, want 2", ticket.Priority)
	}
}

func TestStore_Create_PriorityZero(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	opts := CreateOptions{
		Title:       "High Priority",
		Priority:    0,
		PrioritySet: true,
	}

	ticket, err := store.Create(opts)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if ticket.Priority != 0 {
		t.Errorf("Priority = %d, want 0", ticket.Priority)
	}
}

func TestStore_UpdateField(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "update-001", "Update Test")

	err := store.UpdateField("update-001", "status", "closed")
	if err != nil {
		t.Fatalf("UpdateField error: %v", err)
	}

	ticket, _ := store.Load("update-001")
	if ticket.Status != StatusClosed {
		t.Errorf("Status = %v, want %v", ticket.Status, StatusClosed)
	}
}

func TestStore_AddDep(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "main-001", "Main")
	createTestTicket(t, store, "dep-001", "Dependency")

	err := store.AddDep("main-001", "dep-001")
	if err != nil {
		t.Fatalf("AddDep error: %v", err)
	}

	ticket, _ := store.Load("main-001")
	if !ticket.HasDep("dep-001") {
		t.Error("Dependency not added")
	}
}

func TestStore_AddDep_Idempotent(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "main-001", "Main")
	createTestTicket(t, store, "dep-001", "Dependency")

	store.AddDep("main-001", "dep-001")
	store.AddDep("main-001", "dep-001") // Add again

	ticket, _ := store.Load("main-001")
	count := 0
	for _, d := range ticket.Deps {
		if d == "dep-001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Dep appears %d times, should be 1", count)
	}
}

func TestStore_RemoveDep(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "main-001", "Main")
	createTestTicket(t, store, "dep-001", "Dependency")

	store.AddDep("main-001", "dep-001")

	err := store.RemoveDep("main-001", "dep-001")
	if err != nil {
		t.Fatalf("RemoveDep error: %v", err)
	}

	ticket, _ := store.Load("main-001")
	if ticket.HasDep("dep-001") {
		t.Error("Dependency not removed")
	}
}

func TestStore_RemoveDep_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "main-001", "Main")

	err := store.RemoveDep("main-001", "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent dependency")
	}
}

func TestStore_AddLink(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "link-001", "First")
	createTestTicket(t, store, "link-002", "Second")

	err := store.AddLink("link-001", "link-002")
	if err != nil {
		t.Fatalf("AddLink error: %v", err)
	}

	ticket, _ := store.Load("link-001")
	if !ticket.HasLink("link-002") {
		t.Error("Link not added")
	}
}

func TestStore_RemoveLink(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "link-001", "First")
	createTestTicket(t, store, "link-002", "Second")

	store.AddLink("link-001", "link-002")

	err := store.RemoveLink("link-001", "link-002")
	if err != nil {
		t.Fatalf("RemoveLink error: %v", err)
	}

	ticket, _ := store.Load("link-001")
	if ticket.HasLink("link-002") {
		t.Error("Link not removed")
	}
}

func TestStore_AddNote(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "note-001", "Note Test")

	err := store.AddNote("note-001", "This is a note")
	if err != nil {
		t.Fatalf("AddNote error: %v", err)
	}

	content, _ := store.GetFileContent("note-001")
	if !strings.Contains(content, "## Notes") {
		t.Error("Notes section not added")
	}
	if !strings.Contains(content, "This is a note") {
		t.Error("Note content not added")
	}
}

func TestStore_AddNote_WithTimestamp(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "note-001", "Note Test")

	store.AddNote("note-001", "Timestamped note")

	content, _ := store.GetFileContent("note-001")
	// Should contain a bold timestamp like **2024-01-15T10:30:00Z**
	if !strings.Contains(content, "**20") {
		t.Error("Timestamp not added to note")
	}
}

func TestStore_FileHasSection(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "section-001", "Section Test")

	store.AddNote("section-001", "A note")

	has, err := store.FileHasSection("section-001", "## Notes")
	if err != nil {
		t.Fatalf("FileHasSection error: %v", err)
	}
	if !has {
		t.Error("Should have Notes section")
	}

	has, _ = store.FileHasSection("section-001", "## Nonexistent")
	if has {
		t.Error("Should not have Nonexistent section")
	}
}
