package ticket

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheLoadTicketHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	// Create a test ticket file
	content := `---
id: test-123
status: open
deps: []
links: []
created: 2024-01-01T00:00:00Z
type: task
priority: 2
---
# Test Ticket

This is the body.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache()

	// First load - should read from disk
	ticket1, err := cache.LoadTicketHeader(path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket1.ID != "test-123" {
		t.Errorf("ID = %q, want %q", ticket1.ID, "test-123")
	}

	// Second load - should use cache
	ticket2, err := cache.LoadTicketHeader(path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket1 != ticket2 {
		t.Error("Expected same pointer from cache")
	}
}

func TestCacheLoadTicketUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := `---
id: test-456
status: in_progress
deps: []
links: []
created: 2024-01-01T00:00:00Z
type: task
priority: 2
---
# Test Ticket

Body content here.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache()

	// Load header first
	header, err := cache.LoadTicketHeader(path)
	if err != nil {
		t.Fatal(err)
	}
	if header.Body != "" {
		t.Error("Header-only load should not have body")
	}

	// Load full ticket - should read from disk since we only had header
	full, err := cache.LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if full.Body == "" {
		t.Error("Full load should have body")
	}

	// Load full again - should use cache
	full2, err := cache.LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if full != full2 {
		t.Error("Expected same pointer from cache")
	}
}

func TestCacheUpdateYAMLField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := `---
id: test-789
status: open
deps: []
links: []
created: 2024-01-01T00:00:00Z
type: task
priority: 2
---
# Test Ticket
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache()

	// Load into cache
	ticket, err := cache.LoadTicketHeader(path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Status != StatusOpen {
		t.Errorf("Status = %q, want %q", ticket.Status, StatusOpen)
	}

	// Update status
	if err := cache.UpdateYAMLField(path, "status", "closed"); err != nil {
		t.Fatal(err)
	}

	// Cache should be updated in-place
	if ticket.Status != StatusClosed {
		t.Errorf("Status = %q, want %q", ticket.Status, StatusClosed)
	}

	// Verify disk was updated
	diskTicket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if diskTicket.Status != StatusClosed {
		t.Errorf("Disk status = %q, want %q", diskTicket.Status, StatusClosed)
	}
}

func TestCacheGetYAMLField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := `---
id: cache-get-test
status: open
deps: []
links: []
created: 2024-01-01T00:00:00Z
type: bug
priority: 1
---
# Test
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache()

	// Get without cache - reads from disk
	val, err := cache.GetYAMLField(path, "type")
	if err != nil {
		t.Fatal(err)
	}
	if val != "bug" {
		t.Errorf("type = %q, want %q", val, "bug")
	}

	// Load into cache
	_, err = cache.LoadTicketHeader(path)
	if err != nil {
		t.Fatal(err)
	}

	// Get with cache - should use cached value
	val, err = cache.GetYAMLField(path, "priority")
	if err != nil {
		t.Fatal(err)
	}
	if val != "1" {
		t.Errorf("priority = %q, want %q", val, "1")
	}
}
