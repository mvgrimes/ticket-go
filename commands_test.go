package ticket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestTicketWithStatus(t *testing.T, store *Store, id, title string, status Status) {
	t.Helper()
	content := `---
id: ` + id + `
status: ` + string(status) + `
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

func createTestTicketWithPriority(t *testing.T, store *Store, id, title string, priority int) {
	t.Helper()
	content := `---
id: ` + id + `
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: ` + string(rune('0'+priority)) + `
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

func createTestTicketWithAssignee(t *testing.T, store *Store, id, title, assignee string) {
	t.Helper()
	content := `---
id: ` + id + `
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
assignee: ` + assignee + `
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

func createTestTicketWithTags(t *testing.T, store *Store, id, title string, tags []string) {
	t.Helper()
	tagsStr := "[" + strings.Join(tags, ", ") + "]"
	content := `---
id: ` + id + `
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
tags: ` + tagsStr + `
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

func createTestTicketWithDeps(t *testing.T, store *Store, id, title string, deps []string) {
	t.Helper()
	depsStr := "[" + strings.Join(deps, ", ") + "]"
	content := `---
id: ` + id + `
status: open
deps: ` + depsStr + `
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

func createTestTicketWithParent(t *testing.T, store *Store, id, title, parent string) {
	t.Helper()
	content := `---
id: ` + id + `
status: open
deps: []
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
parent: ` + parent + `
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

func createTestTicketWithLinks(t *testing.T, store *Store, id, title string, links []string) {
	t.Helper()
	linksStr := "[" + strings.Join(links, ", ") + "]"
	content := `---
id: ` + id + `
status: open
deps: []
links: ` + linksStr + `
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# ` + title + `
`
	store.EnsureDir()
	path := store.TicketPath(id)
	os.WriteFile(path, []byte(content), 0644)
}

// ============================================================================
// ListTicketsFiltered Tests
// ============================================================================

func TestStore_ListTicketsFiltered_NoFilter(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "filter-001", "First")
	createTestTicket(t, store, "filter-002", "Second")

	tickets, err := store.ListTicketsFiltered(ListOptions{})
	if err != nil {
		t.Fatalf("ListTicketsFiltered error: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("Count = %d, want 2", len(tickets))
	}
}

func TestStore_ListTicketsFiltered_ByStatus(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "filter-001", "Open", StatusOpen)
	createTestTicketWithStatus(t, store, "filter-002", "Closed", StatusClosed)
	createTestTicketWithStatus(t, store, "filter-003", "InProgress", StatusInProgress)

	tickets, err := store.ListTicketsFiltered(ListOptions{Status: "open"})
	if err != nil {
		t.Fatalf("ListTicketsFiltered error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("Count = %d, want 1", len(tickets))
	}
	if tickets[0].ID != "filter-001" {
		t.Errorf("ID = %q, want filter-001", tickets[0].ID)
	}
}

func TestStore_ListTicketsFiltered_ByAssignee(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithAssignee(t, store, "filter-001", "Alice Task", "Alice")
	createTestTicketWithAssignee(t, store, "filter-002", "Bob Task", "Bob")
	createTestTicketWithAssignee(t, store, "filter-003", "Alice Other", "Alice")

	tickets, err := store.ListTicketsFiltered(ListOptions{Assignee: "Alice"})
	if err != nil {
		t.Fatalf("ListTicketsFiltered error: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("Count = %d, want 2", len(tickets))
	}
}

func TestStore_ListTicketsFiltered_ByTag(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithTags(t, store, "filter-001", "UI Task", []string{"ui", "frontend"})
	createTestTicketWithTags(t, store, "filter-002", "Backend Task", []string{"backend", "api"})
	createTestTicketWithTags(t, store, "filter-003", "Full Stack", []string{"ui", "backend"})

	tickets, err := store.ListTicketsFiltered(ListOptions{Tag: "ui"})
	if err != nil {
		t.Fatalf("ListTicketsFiltered error: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("Count = %d, want 2", len(tickets))
	}
}

// ============================================================================
// ReadyTickets Tests
// ============================================================================

func TestStore_ReadyTickets_NoDeps(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "ready-001", "No Deps", StatusOpen)

	tickets, err := store.ReadyTickets(ListOptions{})
	if err != nil {
		t.Fatalf("ReadyTickets error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("Count = %d, want 1", len(tickets))
	}
}

func TestStore_ReadyTickets_ClosedDeps(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "dep-001", "Dependency", StatusClosed)
	createTestTicketWithDeps(t, store, "ready-001", "With Closed Dep", []string{"dep-001"})

	tickets, err := store.ReadyTickets(ListOptions{})
	if err != nil {
		t.Fatalf("ReadyTickets error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("Count = %d, want 1", len(tickets))
	}
	if tickets[0].ID != "ready-001" {
		t.Errorf("ID = %q, want ready-001", tickets[0].ID)
	}
}

func TestStore_ReadyTickets_ExcludesBlocked(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "dep-001", "Open Dependency", StatusOpen)
	createTestTicketWithDeps(t, store, "blocked-001", "Blocked", []string{"dep-001"})

	tickets, err := store.ReadyTickets(ListOptions{})
	if err != nil {
		t.Fatalf("ReadyTickets error: %v", err)
	}

	// Should only include the dependency (it has no deps), not the blocked ticket
	if len(tickets) != 1 {
		t.Errorf("Count = %d, want 1", len(tickets))
	}
	if tickets[0].ID != "dep-001" {
		t.Errorf("ID = %q, want dep-001", tickets[0].ID)
	}
}

func TestStore_ReadyTickets_ExcludesClosed(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "closed-001", "Closed Ticket", StatusClosed)
	createTestTicketWithStatus(t, store, "open-001", "Open Ticket", StatusOpen)

	tickets, err := store.ReadyTickets(ListOptions{})
	if err != nil {
		t.Fatalf("ReadyTickets error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("Count = %d, want 1", len(tickets))
	}
	if tickets[0].ID != "open-001" {
		t.Errorf("ID = %q, want open-001", tickets[0].ID)
	}
}

func TestStore_ReadyTickets_SortedByPriority(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithPriority(t, store, "low-001", "Low Priority", 3)
	createTestTicketWithPriority(t, store, "high-001", "High Priority", 1)
	createTestTicketWithPriority(t, store, "med-001", "Medium Priority", 2)

	tickets, err := store.ReadyTickets(ListOptions{})
	if err != nil {
		t.Fatalf("ReadyTickets error: %v", err)
	}

	if len(tickets) != 3 {
		t.Fatalf("Count = %d, want 3", len(tickets))
	}

	if tickets[0].Priority != 1 {
		t.Errorf("First priority = %d, want 1", tickets[0].Priority)
	}
	if tickets[1].Priority != 2 {
		t.Errorf("Second priority = %d, want 2", tickets[1].Priority)
	}
	if tickets[2].Priority != 3 {
		t.Errorf("Third priority = %d, want 3", tickets[2].Priority)
	}
}

// ============================================================================
// BlockedTickets Tests
// ============================================================================

func TestStore_BlockedTickets(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "blocker-001", "Blocker", StatusOpen)
	createTestTicketWithDeps(t, store, "blocked-001", "Blocked", []string{"blocker-001"})

	tickets, blockers, err := store.BlockedTickets(ListOptions{})
	if err != nil {
		t.Fatalf("BlockedTickets error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("Tickets count = %d, want 1", len(tickets))
	}
	if tickets[0].ID != "blocked-001" {
		t.Errorf("ID = %q, want blocked-001", tickets[0].ID)
	}
	if len(blockers) != 1 || len(blockers[0]) != 1 {
		t.Errorf("Blockers format incorrect")
	}
	if blockers[0][0] != "blocker-001" {
		t.Errorf("Blocker = %q, want blocker-001", blockers[0][0])
	}
}

func TestStore_BlockedTickets_ExcludesUnblocked(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "blocker-001", "Blocker", StatusClosed)
	createTestTicketWithDeps(t, store, "unblocked-001", "Unblocked", []string{"blocker-001"})

	tickets, _, err := store.BlockedTickets(ListOptions{})
	if err != nil {
		t.Fatalf("BlockedTickets error: %v", err)
	}

	if len(tickets) != 0 {
		t.Errorf("Count = %d, want 0 (all deps closed)", len(tickets))
	}
}

func TestStore_BlockedTickets_OnlyUnclosedBlockers(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "open-blocker", "Open Blocker", StatusOpen)
	createTestTicketWithStatus(t, store, "closed-blocker", "Closed Blocker", StatusClosed)

	content := `---
id: blocked-001
status: open
deps: [open-blocker, closed-blocker]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# Blocked
`
	store.EnsureDir()
	os.WriteFile(store.TicketPath("blocked-001"), []byte(content), 0644)

	_, blockers, err := store.BlockedTickets(ListOptions{})
	if err != nil {
		t.Fatalf("BlockedTickets error: %v", err)
	}

	if len(blockers[0]) != 1 {
		t.Errorf("Blockers count = %d, want 1 (only open)", len(blockers[0]))
	}
	if blockers[0][0] != "open-blocker" {
		t.Errorf("Blocker = %q, want open-blocker", blockers[0][0])
	}
}

// ============================================================================
// ClosedTickets Tests
// ============================================================================

func TestStore_ClosedTickets(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "closed-001", "Closed", StatusClosed)
	createTestTicketWithStatus(t, store, "open-001", "Open", StatusOpen)

	tickets, err := store.ClosedTickets(ListOptions{}, 10)
	if err != nil {
		t.Fatalf("ClosedTickets error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("Count = %d, want 1", len(tickets))
	}
	if tickets[0].ID != "closed-001" {
		t.Errorf("ID = %q, want closed-001", tickets[0].ID)
	}
}

func TestStore_ClosedTickets_Limit(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "closed-001", "Closed 1", StatusClosed)
	createTestTicketWithStatus(t, store, "closed-002", "Closed 2", StatusClosed)
	createTestTicketWithStatus(t, store, "closed-003", "Closed 3", StatusClosed)

	tickets, err := store.ClosedTickets(ListOptions{}, 2)
	if err != nil {
		t.Fatalf("ClosedTickets error: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("Count = %d, want 2 (limit)", len(tickets))
	}
}

// ============================================================================
// GetShowInfo Tests
// ============================================================================

func TestStore_GetShowInfo_Basic(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "show-001", "Show Test")

	info, err := store.GetShowInfo("show-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if info.Content == "" {
		t.Error("Content should not be empty")
	}
	if len(info.Blockers) != 0 {
		t.Errorf("Blockers = %d, want 0", len(info.Blockers))
	}
}

func TestStore_GetShowInfo_WithBlockers(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "blocker-001", "Blocker", StatusOpen)
	createTestTicketWithDeps(t, store, "show-001", "Show Test", []string{"blocker-001"})

	info, err := store.GetShowInfo("show-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if len(info.Blockers) != 1 {
		t.Errorf("Blockers = %d, want 1", len(info.Blockers))
	}
	if info.Blockers[0].ID != "blocker-001" {
		t.Errorf("Blocker ID = %q, want blocker-001", info.Blockers[0].ID)
	}
}

func TestStore_GetShowInfo_HidesClosedBlockers(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithStatus(t, store, "blocker-001", "Closed Blocker", StatusClosed)
	createTestTicketWithDeps(t, store, "show-001", "Show Test", []string{"blocker-001"})

	info, err := store.GetShowInfo("show-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if len(info.Blockers) != 0 {
		t.Errorf("Blockers = %d, want 0 (closed)", len(info.Blockers))
	}
}

func TestStore_GetShowInfo_WithBlocking(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "show-001", "Blocker")
	createTestTicketWithDeps(t, store, "blocked-001", "Blocked", []string{"show-001"})

	info, err := store.GetShowInfo("show-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if len(info.Blocking) != 1 {
		t.Errorf("Blocking = %d, want 1", len(info.Blocking))
	}
	if info.Blocking[0].ID != "blocked-001" {
		t.Errorf("Blocking ID = %q, want blocked-001", info.Blocking[0].ID)
	}
}

func TestStore_GetShowInfo_WithChildren(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "parent-001", "Parent")
	createTestTicketWithParent(t, store, "child-001", "Child", "parent-001")

	info, err := store.GetShowInfo("parent-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if len(info.Children) != 1 {
		t.Errorf("Children = %d, want 1", len(info.Children))
	}
	if info.Children[0].ID != "child-001" {
		t.Errorf("Child ID = %q, want child-001", info.Children[0].ID)
	}
}

func TestStore_GetShowInfo_WithLinked(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithLinks(t, store, "show-001", "First", []string{"linked-001"})
	createTestTicket(t, store, "linked-001", "Linked")

	info, err := store.GetShowInfo("show-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if len(info.Linked) != 1 {
		t.Errorf("Linked = %d, want 1", len(info.Linked))
	}
	if info.Linked[0].ID != "linked-001" {
		t.Errorf("Linked ID = %q, want linked-001", info.Linked[0].ID)
	}
}

func TestStore_GetShowInfo_WithParent(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "parent-001", "Parent Ticket")
	createTestTicketWithParent(t, store, "child-001", "Child", "parent-001")

	info, err := store.GetShowInfo("child-001")
	if err != nil {
		t.Fatalf("GetShowInfo error: %v", err)
	}

	if info.ParentInfo == nil {
		t.Error("ParentInfo should not be nil")
	} else if info.ParentInfo.Title != "Parent Ticket" {
		t.Errorf("Parent title = %q, want 'Parent Ticket'", info.ParentInfo.Title)
	}
}

// ============================================================================
// GetDepTree Tests
// ============================================================================

func TestStore_GetDepTree_Simple(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithDeps(t, store, "root", "Root", []string{"child"})
	createTestTicket(t, store, "child", "Child")

	tree, err := store.GetDepTree("root", false)
	if err != nil {
		t.Fatalf("GetDepTree error: %v", err)
	}

	if tree.Ticket.ID != "root" {
		t.Errorf("Root ID = %q, want 'root'", tree.Ticket.ID)
	}
	if len(tree.Children) != 1 {
		t.Errorf("Children count = %d, want 1", len(tree.Children))
	}
	if tree.Children[0].Ticket.ID != "child" {
		t.Errorf("Child ID = %q, want 'child'", tree.Children[0].Ticket.ID)
	}
}

func TestStore_GetDepTree_Deep(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithDeps(t, store, "root", "Root", []string{"level1"})
	createTestTicketWithDeps(t, store, "level1", "Level 1", []string{"level2"})
	createTestTicketWithDeps(t, store, "level2", "Level 2", []string{"level3"})
	createTestTicket(t, store, "level3", "Level 3")

	tree, err := store.GetDepTree("root", false)
	if err != nil {
		t.Fatalf("GetDepTree error: %v", err)
	}

	// Navigate to level 3
	current := tree
	for i := 0; i < 3; i++ {
		if len(current.Children) != 1 {
			t.Fatalf("Level %d should have 1 child", i)
		}
		current = current.Children[0]
	}
	if current.Ticket.ID != "level3" {
		t.Errorf("Deepest node = %q, want 'level3'", current.Ticket.ID)
	}
}

func TestStore_GetDepTree_HandlesCycle(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create cycle: a -> b -> a
	content1 := `---
id: cycle-a
status: open
deps: [cycle-b]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# Cycle A
`
	content2 := `---
id: cycle-b
status: open
deps: [cycle-a]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# Cycle B
`
	store.EnsureDir()
	os.WriteFile(store.TicketPath("cycle-a"), []byte(content1), 0644)
	os.WriteFile(store.TicketPath("cycle-b"), []byte(content2), 0644)

	// Should not hang or crash
	tree, err := store.GetDepTree("cycle-a", false)
	if err != nil {
		t.Fatalf("GetDepTree error: %v", err)
	}

	if tree == nil {
		t.Error("Tree should not be nil")
	}
}

// ============================================================================
// PrintDepTree Tests
// ============================================================================

func TestPrintDepTree(t *testing.T) {
	tree := &DepTreeNode{
		Ticket: &Ticket{ID: "root", Status: StatusOpen, Title: "Root"},
		Children: []*DepTreeNode{
			{Ticket: &Ticket{ID: "child", Status: StatusOpen, Title: "Child"}},
		},
	}

	output := PrintDepTree(tree, "", true, true)

	if !strings.Contains(output, "root [open] Root") {
		t.Error("Output should contain root line")
	}
	if !strings.Contains(output, "child [open] Child") {
		t.Error("Output should contain child line")
	}
	if !strings.Contains(output, "└──") {
		t.Error("Output should contain box-drawing character")
	}
}

// ============================================================================
// FindCycles Tests
// ============================================================================

func TestStore_FindCycles_NoCycle(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicketWithDeps(t, store, "a", "A", []string{"b"})
	createTestTicket(t, store, "b", "B")

	cycles, err := store.FindCycles()
	if err != nil {
		t.Fatalf("FindCycles error: %v", err)
	}

	if len(cycles) != 0 {
		t.Errorf("Cycles = %d, want 0", len(cycles))
	}
}

func TestStore_FindCycles_SimpleCycle(t *testing.T) {
	store, _ := setupTestStore(t)

	content1 := `---
id: cycle-a
status: open
deps: [cycle-b]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# A
`
	content2 := `---
id: cycle-b
status: open
deps: [cycle-a]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# B
`
	store.EnsureDir()
	os.WriteFile(store.TicketPath("cycle-a"), []byte(content1), 0644)
	os.WriteFile(store.TicketPath("cycle-b"), []byte(content2), 0644)

	cycles, err := store.FindCycles()
	if err != nil {
		t.Fatalf("FindCycles error: %v", err)
	}

	if len(cycles) != 1 {
		t.Errorf("Cycles = %d, want 1", len(cycles))
	}
}

func TestStore_FindCycles_IgnoresClosed(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create cycle but with one closed ticket
	content1 := `---
id: cycle-a
status: open
deps: [cycle-b]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# A
`
	content2 := `---
id: cycle-b
status: closed
deps: [cycle-a]
links: []
created: 2024-01-15T10:30:00Z
type: task
priority: 2
---
# B
`
	store.EnsureDir()
	os.WriteFile(store.TicketPath("cycle-a"), []byte(content1), 0644)
	os.WriteFile(store.TicketPath("cycle-b"), []byte(content2), 0644)

	cycles, err := store.FindCycles()
	if err != nil {
		t.Fatalf("FindCycles error: %v", err)
	}

	if len(cycles) != 0 {
		t.Errorf("Cycles = %d, want 0 (closed ticket breaks cycle)", len(cycles))
	}
}

// ============================================================================
// QueryTicketsFiltered Tests
// ============================================================================

func TestStore_QueryTicketsFiltered(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "query-001", "Query Test")
	createTestTicket(t, store, "query-002", "Another")

	tickets, err := store.QueryTicketsFiltered("")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered error: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("Tickets = %d, want 2", len(tickets))
	}

	// Check required fields
	for _, ticket := range tickets {
		if ticket.ID == "" {
			t.Error("Ticket missing 'id' field")
		}
		if ticket.Status == "" {
			t.Error("Ticket missing 'status' field")
		}
	}
}

func TestStore_QueryTicketsFiltered_Empty(t *testing.T) {
	store, _ := setupTestStore(t)
	store.EnsureDir()

	tickets, err := store.QueryTicketsFiltered("")
	if err != nil {
		t.Fatalf("QueryTicketsFiltered error: %v", err)
	}

	if len(tickets) != 0 {
		t.Errorf("Tickets = %d, want 0", len(tickets))
	}
}

// ============================================================================
// LinkTickets Tests
// ============================================================================

func TestStore_LinkTickets(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "link-001", "First")
	createTestTicket(t, store, "link-002", "Second")

	count, err := store.LinkTickets([]string{"link-001", "link-002"})
	if err != nil {
		t.Fatalf("LinkTickets error: %v", err)
	}

	if count != 2 {
		t.Errorf("Count = %d, want 2 (bidirectional)", count)
	}

	// Verify both have links
	t1, _ := store.Load("link-001")
	t2, _ := store.Load("link-002")

	if !t1.HasLink("link-002") {
		t.Error("link-001 should link to link-002")
	}
	if !t2.HasLink("link-001") {
		t.Error("link-002 should link to link-001")
	}
}

func TestStore_LinkTickets_Multiple(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "link-001", "First")
	createTestTicket(t, store, "link-002", "Second")
	createTestTicket(t, store, "link-003", "Third")

	count, err := store.LinkTickets([]string{"link-001", "link-002", "link-003"})
	if err != nil {
		t.Fatalf("LinkTickets error: %v", err)
	}

	// 3 tickets = 6 links (each links to 2 others)
	if count != 6 {
		t.Errorf("Count = %d, want 6", count)
	}
}

func TestStore_LinkTickets_Idempotent(t *testing.T) {
	store, _ := setupTestStore(t)
	createTestTicket(t, store, "link-001", "First")
	createTestTicket(t, store, "link-002", "Second")

	store.LinkTickets([]string{"link-001", "link-002"})
	count, err := store.LinkTickets([]string{"link-001", "link-002"})
	if err != nil {
		t.Fatalf("LinkTickets error: %v", err)
	}

	if count != 0 {
		t.Errorf("Count = %d, want 0 (already linked)", count)
	}
}

// ============================================================================
// Ticket.ToJSON Tests
// ============================================================================

func TestTicket_ToJSON(t *testing.T) {
	ticket := &Ticket{
		ID:       "json-001",
		Status:   StatusOpen,
		Deps:     []string{"dep-001"},
		Links:    []string{"link-001"},
		Type:     "bug",
		Priority: 1,
		Assignee: "Tester",
		Tags:     []string{"urgent"},
	}

	j := ticket.ToJSON()

	if j.ID != "json-001" {
		t.Errorf("ID = %q, want 'json-001'", j.ID)
	}
	if j.Status != "open" {
		t.Errorf("Status = %q, want 'open'", j.Status)
	}
	if len(j.Deps) != 1 || j.Deps[0] != "dep-001" {
		t.Errorf("Deps = %v, want [dep-001]", j.Deps)
	}
	if j.Priority != 1 {
		t.Errorf("Priority = %d, want 1", j.Priority)
	}
}

// ============================================================================
// FindTicketsDir Tests
// ============================================================================

func TestFindTicketsDir_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	ticketsDir := filepath.Join(tmpDir, "custom-tickets")
	os.MkdirAll(ticketsDir, 0755)

	os.Setenv("TICKETS_DIR", ticketsDir)
	defer os.Unsetenv("TICKETS_DIR")

	dir, err := FindTicketsDir()
	if err != nil {
		t.Fatalf("FindTicketsDir error: %v", err)
	}

	if dir != ticketsDir {
		t.Errorf("Dir = %q, want %q", dir, ticketsDir)
	}
}
