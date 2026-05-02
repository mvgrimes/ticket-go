package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// testEnv holds the test environment state
type testEnv struct {
	t          *testing.T
	dir        string // temp directory
	ticketsDir string // .tickets directory path
	origDir    string // original working directory
	stdoutBuf  *bytes.Buffer
	stderrBuf  *bytes.Buffer
	err        error
	lastID     string // last created ticket ID
}

// newTestEnv creates a new test environment
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()

	// Save original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Change to test directory
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to test directory: %v", err)
	}

	e := &testEnv{
		t:          t,
		dir:        dir,
		ticketsDir: filepath.Join(dir, ".tickets"),
		origDir:    origDir,
		stdoutBuf:  &bytes.Buffer{},
		stderrBuf:  &bytes.Buffer{},
	}

	// Set up cleanup to restore directory
	t.Cleanup(func() {
		os.Chdir(origDir)
		// Restore global writers
		stdout = os.Stdout
		stderr = os.Stderr
	})

	return e
}

// run executes a tk command using the internal run function
func (e *testEnv) run(args ...string) *testEnv {
	e.t.Helper()

	// Reset buffers
	e.stdoutBuf.Reset()
	e.stderrBuf.Reset()

	// Set global writers to our buffers
	stdout = e.stdoutBuf
	stderr = e.stderrBuf

	// Ensure we're in the right directory
	os.Chdir(e.dir)

	// Run the command
	e.err = run(args)

	// Track created ticket ID
	if len(args) > 0 && args[0] == "create" && e.err == nil {
		e.lastID = strings.TrimSpace(e.stdoutBuf.String())
	}

	return e
}

// runInSubdir runs command from a subdirectory
func (e *testEnv) runInSubdir(subdir string, args ...string) *testEnv {
	e.t.Helper()
	workDir := filepath.Join(e.dir, subdir)
	os.MkdirAll(workDir, 0755)

	// Change to subdirectory
	if err := os.Chdir(workDir); err != nil {
		e.t.Fatalf("failed to change to subdirectory: %v", err)
	}

	return e.run(args...)
}

// runWithEnv executes with custom environment variables
func (e *testEnv) runWithEnv(env map[string]string, args ...string) *testEnv {
	e.t.Helper()

	// Set environment variables
	for k, v := range env {
		old := os.Getenv(k)
		os.Setenv(k, v)
		defer os.Setenv(k, old)
	}

	return e.run(args...)
}

// initTicketsDir creates the .tickets directory
func (e *testEnv) initTicketsDir() *testEnv {
	e.t.Helper()
	os.MkdirAll(e.ticketsDir, 0755)
	return e
}

// createTicket creates a ticket file directly
func (e *testEnv) createTicket(id, title string) *testEnv {
	return e.createTicketFull(id, title, "open", 2, "", nil)
}

// createTicketFull creates a ticket with all options
func (e *testEnv) createTicketFull(id, title, status string, priority int, parent string, deps []string) *testEnv {
	e.t.Helper()
	os.MkdirAll(e.ticketsDir, 0755)

	depsStr := "[]"
	if len(deps) > 0 {
		depsStr = "[" + strings.Join(deps, ", ") + "]"
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmt.Fprintf(&buf, "id: %s\n", id)
	fmt.Fprintf(&buf, "status: %s\n", status)
	fmt.Fprintf(&buf, "deps: %s\n", depsStr)
	buf.WriteString("links: []\n")
	buf.WriteString("created: 2024-01-01T00:00:00Z\n")
	buf.WriteString("type: task\n")
	fmt.Fprintf(&buf, "priority: %d\n", priority)
	if parent != "" {
		fmt.Fprintf(&buf, "parent: %s\n", parent)
	}
	buf.WriteString("---\n")
	fmt.Fprintf(&buf, "# %s\n\n", title)
	buf.WriteString("Description\n")

	path := filepath.Join(e.ticketsDir, id+".md")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		e.t.Fatalf("failed to create ticket: %v", err)
	}
	return e
}

// setStatus updates a ticket's status
func (e *testEnv) setStatus(id, status string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket: %v", err)
	}
	re := regexp.MustCompile(`(?m)^status: \w+`)
	newContent := re.ReplaceAllString(string(content), "status: "+status)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		e.t.Fatalf("failed to write ticket: %v", err)
	}
	return e
}

// addDep adds a dependency to a ticket
func (e *testEnv) addDep(id, depID string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket: %v", err)
	}
	re := regexp.MustCompile(`(?m)^deps: \[(.*?)\]`)
	matches := re.FindStringSubmatch(string(content))
	var newDeps string
	if len(matches) > 1 && matches[1] != "" {
		newDeps = matches[1] + ", " + depID
	} else {
		newDeps = depID
	}
	newContent := re.ReplaceAllString(string(content), "deps: ["+newDeps+"]")
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		e.t.Fatalf("failed to write ticket: %v", err)
	}
	return e
}

// addLink adds a link to a ticket (one direction only)
func (e *testEnv) addLink(id, linkID string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket: %v", err)
	}
	re := regexp.MustCompile(`(?m)^links: \[(.*?)\]`)
	matches := re.FindStringSubmatch(string(content))
	var newLinks string
	if len(matches) > 1 && matches[1] != "" {
		newLinks = matches[1] + ", " + linkID
	} else {
		newLinks = linkID
	}
	newContent := re.ReplaceAllString(string(content), "links: ["+newLinks+"]")
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		e.t.Fatalf("failed to write ticket: %v", err)
	}
	return e
}

// addBidirectionalLink adds links in both directions
func (e *testEnv) addBidirectionalLink(id1, id2 string) *testEnv {
	return e.addLink(id1, id2).addLink(id2, id1)
}

// Output helpers
func (e *testEnv) stdout() string {
	return strings.TrimSpace(e.stdoutBuf.String())
}

func (e *testEnv) stderr() string {
	return strings.TrimSpace(e.stderrBuf.String())
}

func (e *testEnv) output() string {
	out := e.stdout() + e.stderr()
	// Include error message if present (run() returns error, not printed to stderr)
	if e.err != nil {
		out += e.err.Error()
	}
	return out
}

// Assertions

func (e *testEnv) assertSuccess() *testEnv {
	e.t.Helper()
	if e.err != nil {
		e.t.Errorf("expected success, got error: %v\nstdout: %s\nstderr: %s", e.err, e.stdout(), e.stderr())
	}
	return e
}

func (e *testEnv) assertFail() *testEnv {
	e.t.Helper()
	if e.err == nil {
		e.t.Errorf("expected failure, got success\nstdout: %s", e.stdout())
	}
	return e
}

func (e *testEnv) assertOutput(expected string) *testEnv {
	e.t.Helper()
	if e.stdout() != expected {
		e.t.Errorf("expected output %q, got %q", expected, e.stdout())
	}
	return e
}

func (e *testEnv) assertOutputContains(text string) *testEnv {
	e.t.Helper()
	combined := e.output()
	if !strings.Contains(combined, text) {
		e.t.Errorf("expected output to contain %q\nstdout: %s\nstderr: %s", text, e.stdout(), e.stderr())
	}
	return e
}

func (e *testEnv) assertOutputNotContains(text string) *testEnv {
	e.t.Helper()
	combined := e.output()
	if strings.Contains(combined, text) {
		e.t.Errorf("expected output to NOT contain %q\nstdout: %s\nstderr: %s", text, e.stdout(), e.stderr())
	}
	return e
}

func (e *testEnv) assertOutputEmpty() *testEnv {
	e.t.Helper()
	if e.stdout() != "" {
		e.t.Errorf("expected empty output, got %q", e.stdout())
	}
	return e
}

func (e *testEnv) assertOutputMatchesPattern(pattern string) *testEnv {
	e.t.Helper()
	re := regexp.MustCompile(pattern)
	if !re.MatchString(e.stdout()) {
		e.t.Errorf("output %q does not match pattern %q", e.stdout(), pattern)
	}
	return e
}

func (e *testEnv) assertOutputMatchesIDPattern() *testEnv {
	e.t.Helper()
	pattern := `^[a-z0-9]+-[a-z0-9]{4}$`
	return e.assertOutputMatchesPattern(pattern)
}

func (e *testEnv) assertTicketHasField(id, field, value string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket %s: %v", id, err)
	}
	pattern := `(?m)^` + regexp.QuoteMeta(field) + `:\s*(.+)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) < 2 {
		e.t.Errorf("field %q not found in ticket %s", field, id)
		return e
	}
	actual := strings.TrimSpace(matches[1])
	if actual != value {
		e.t.Errorf("ticket %s field %q = %q, want %q", id, field, actual, value)
	}
	return e
}

func (e *testEnv) assertTicketContains(id, text string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket %s: %v", id, err)
	}
	if !strings.Contains(string(content), text) {
		e.t.Errorf("ticket %s does not contain %q", id, text)
	}
	return e
}

func (e *testEnv) assertTicketHasDep(id, depID string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket %s: %v", id, err)
	}
	re := regexp.MustCompile(`(?m)^deps:\s*\[([^\]]*)\]`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) < 2 || !strings.Contains(matches[1], depID) {
		e.t.Errorf("ticket %s does not have dep %s", id, depID)
	}
	return e
}

func (e *testEnv) assertTicketNotHasDep(id, depID string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket %s: %v", id, err)
	}
	re := regexp.MustCompile(`(?m)^deps:\s*\[([^\]]*)\]`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) >= 2 && strings.Contains(matches[1], depID) {
		e.t.Errorf("ticket %s should not have dep %s", id, depID)
	}
	return e
}

func (e *testEnv) assertTicketHasLink(id, linkID string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket %s: %v", id, err)
	}
	re := regexp.MustCompile(`(?m)^links:\s*\[([^\]]*)\]`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) < 2 || !strings.Contains(matches[1], linkID) {
		e.t.Errorf("ticket %s does not have link %s", id, linkID)
	}
	return e
}

func (e *testEnv) assertTicketNotHasLink(id, linkID string) *testEnv {
	e.t.Helper()
	path := filepath.Join(e.ticketsDir, id+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket %s: %v", id, err)
	}
	re := regexp.MustCompile(`(?m)^links:\s*\[([^\]]*)\]`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) >= 2 && strings.Contains(matches[1], linkID) {
		e.t.Errorf("ticket %s should not have link %s", id, linkID)
	}
	return e
}

func (e *testEnv) assertCreatedTicketHasField(field, value string) *testEnv {
	e.t.Helper()
	if e.lastID == "" {
		e.t.Fatal("no ticket was created")
	}
	return e.assertTicketHasField(e.lastID, field, value)
}

func (e *testEnv) assertCreatedTicketContains(text string) *testEnv {
	e.t.Helper()
	if e.lastID == "" {
		e.t.Fatal("no ticket was created")
	}
	return e.assertTicketContains(e.lastID, text)
}

func (e *testEnv) assertCreatedTicketHasTitle(title string) *testEnv {
	e.t.Helper()
	return e.assertCreatedTicketContains("# " + title)
}

func (e *testEnv) assertCreatedTicketHasTimestamp() *testEnv {
	e.t.Helper()
	if e.lastID == "" {
		e.t.Fatal("no ticket was created")
	}
	path := filepath.Join(e.ticketsDir, e.lastID+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("failed to read ticket: %v", err)
	}
	pattern := `created:\s*\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`
	re := regexp.MustCompile(pattern)
	if !re.MatchString(string(content)) {
		e.t.Error("ticket does not have valid created timestamp")
	}
	return e
}

func (e *testEnv) assertTicketsDirExists() *testEnv {
	e.t.Helper()
	if _, err := os.Stat(e.ticketsDir); os.IsNotExist(err) {
		e.t.Error(".tickets directory does not exist")
	}
	return e
}

func (e *testEnv) assertOutputLineCount(count int) *testEnv {
	e.t.Helper()
	lines := strings.Split(e.stdout(), "\n")
	var nonEmpty int
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	if nonEmpty != count {
		e.t.Errorf("expected %d lines, got %d\noutput: %s", count, nonEmpty, e.stdout())
	}
	return e
}

func (e *testEnv) assertOutputLineContains(lineNum int, text string) *testEnv {
	e.t.Helper()
	lines := strings.Split(e.stdout(), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		e.t.Errorf("line %d out of range (have %d lines)", lineNum, len(lines))
		return e
	}
	if !strings.Contains(lines[lineNum-1], text) {
		e.t.Errorf("line %d does not contain %q\nline: %s", lineNum, text, lines[lineNum-1])
	}
	return e
}

func (e *testEnv) assertValidJSONL() *testEnv {
	e.t.Helper()
	lines := strings.Split(e.stdout(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			e.t.Errorf("invalid JSONL line: %s\nerror: %v", line, err)
		}
	}
	return e
}

func (e *testEnv) assertJSONLHasField(field string) *testEnv {
	e.t.Helper()
	lines := strings.Split(e.stdout(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if _, ok := data[field]; !ok {
			e.t.Errorf("JSONL missing field %q", field)
		}
		return e
	}
	e.t.Error("no JSONL output")
	return e
}

func (e *testEnv) assertTreeFormat() *testEnv {
	e.t.Helper()
	hasTreeChars := strings.ContainsAny(e.stdout(), "├└│─")
	if !hasTreeChars {
		e.t.Errorf("output does not contain box-drawing characters:\n%s", e.stdout())
	}
	return e
}

func (e *testEnv) assertOrderInOutput(first, second string) *testEnv {
	e.t.Helper()
	out := e.stdout()
	firstIdx := strings.Index(out, first)
	secondIdx := strings.Index(out, second)
	if firstIdx == -1 {
		e.t.Errorf("%q not found in output", first)
		return e
	}
	if secondIdx == -1 {
		e.t.Errorf("%q not found in output", second)
		return e
	}
	if firstIdx >= secondIdx {
		e.t.Errorf("expected %q before %q in output:\n%s", first, second, out)
	}
	return e
}

// ============================================================================
// Ticket Creation Tests
// ============================================================================

func TestCreate(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantTitle    string
		wantField    string
		wantValue    string
		wantContains string
	}{
		{
			name:      "basic ticket with title",
			args:      []string{"create", "My first ticket"},
			wantTitle: "My first ticket",
		},
		{
			name:      "ticket with default title",
			args:      []string{"create"},
			wantTitle: "Untitled",
		},
		{
			name:         "ticket with description",
			args:         []string{"create", "Test ticket", "-d", "This is the description"},
			wantContains: "This is the description",
		},
		{
			name:      "ticket with type",
			args:      []string{"create", "Bug ticket", "-t", "bug"},
			wantField: "type",
			wantValue: "bug",
		},
		{
			name:      "ticket with priority",
			args:      []string{"create", "High priority", "-p", "0"},
			wantField: "priority",
			wantValue: "0",
		},
		{
			name:      "ticket with assignee",
			args:      []string{"create", "Assigned ticket", "-a", "John Doe"},
			wantField: "assignee",
			wantValue: "John Doe",
		},
		{
			name:      "ticket with external reference",
			args:      []string{"create", "External ticket", "--external-ref", "JIRA-123"},
			wantField: "external-ref",
			wantValue: "JIRA-123",
		},
		{
			name:         "ticket with design notes",
			args:         []string{"create", "Design ticket", "--design", "Use microservices"},
			wantContains: "## Design",
		},
		{
			name:         "ticket with acceptance criteria",
			args:         []string{"create", "Story ticket", "--acceptance", "Should pass all tests"},
			wantContains: "## Acceptance Criteria",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			e.initTicketsDir()
			e.run(tt.args...).assertSuccess().assertOutputMatchesIDPattern()

			if tt.wantTitle != "" {
				e.assertCreatedTicketHasTitle(tt.wantTitle)
			}
			if tt.wantField != "" {
				e.assertCreatedTicketHasField(tt.wantField, tt.wantValue)
			}
			if tt.wantContains != "" {
				e.assertCreatedTicketContains(tt.wantContains)
			}
		})
	}
}

func TestCreateDefaults(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
	}{
		{"default status open", "status", "open"},
		{"default priority 2", "priority", "2"},
		{"default type task", "type", "task"},
		{"default empty deps", "deps", "[]"},
		{"default empty links", "links", "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			e.initTicketsDir()
			e.run("create", "Test").assertSuccess()
			e.assertCreatedTicketHasField(tt.field, tt.value)
		})
	}
}

func TestCreateWithParent(t *testing.T) {
	e := newTestEnv(t)
	e.createTicket("parent-001", "Parent ticket")
	e.run("create", "Child ticket", "--parent", "parent-001").assertSuccess()
	e.assertCreatedTicketHasField("parent", "parent-001")
}

func TestCreateTimestamp(t *testing.T) {
	e := newTestEnv(t)
	e.initTicketsDir()
	e.run("create", "Timestamped").assertSuccess()
	e.assertCreatedTicketHasTimestamp()
}

func TestCreateInitializesDirectory(t *testing.T) {
	e := newTestEnv(t)
	// Don't initialize - let create do it
	e.run("create", "First ticket").assertSuccess()
	e.assertTicketsDirExists()
}

// ============================================================================
// Status Tests
// ============================================================================

func TestStatus(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus string
		command       string
		args          []string
		wantStatus    string
		wantOutput    string
	}{
		{
			name:          "status to in_progress",
			initialStatus: "open",
			command:       "status",
			args:          []string{"test-0001", "in_progress"},
			wantStatus:    "in_progress",
			wantOutput:    "Updated test-0001 -> in_progress",
		},
		{
			name:          "status to closed",
			initialStatus: "open",
			command:       "status",
			args:          []string{"test-0001", "closed"},
			wantStatus:    "closed",
			wantOutput:    "Updated test-0001 -> closed",
		},
		{
			name:          "status to open",
			initialStatus: "closed",
			command:       "status",
			args:          []string{"test-0001", "open"},
			wantStatus:    "open",
			wantOutput:    "Updated test-0001 -> open",
		},
		{
			name:          "start command",
			initialStatus: "open",
			command:       "start",
			args:          []string{"test-0001"},
			wantStatus:    "in_progress",
			wantOutput:    "Updated test-0001 -> in_progress",
		},
		{
			name:          "close command",
			initialStatus: "open",
			command:       "close",
			args:          []string{"test-0001"},
			wantStatus:    "closed",
			wantOutput:    "Updated test-0001 -> closed",
		},
		{
			name:          "reopen command",
			initialStatus: "closed",
			command:       "reopen",
			args:          []string{"test-0001"},
			wantStatus:    "open",
			wantOutput:    "Updated test-0001 -> open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			e.createTicket("test-0001", "Test ticket")
			if tt.initialStatus != "open" {
				e.setStatus("test-0001", tt.initialStatus)
			}

			args := append([]string{tt.command}, tt.args...)
			e.run(args...).assertSuccess().assertOutput(tt.wantOutput)
			e.assertTicketHasField("test-0001", "status", tt.wantStatus)
		})
	}
}

func TestStatusInvalid(t *testing.T) {
	e := newTestEnv(t)
	e.createTicket("test-0001", "Test ticket")
	e.run("status", "test-0001", "invalid").assertFail()
	e.assertOutputContains("Error: invalid status 'invalid'")
	e.assertOutputContains("open in_progress closed")
}

func TestStatusNonExistent(t *testing.T) {
	e := newTestEnv(t)
	e.initTicketsDir()
	e.run("status", "nonexistent", "open").assertFail()
	e.assertOutputContains("Error: ticket 'nonexistent' not found")
}

func TestStatusPartialID(t *testing.T) {
	e := newTestEnv(t)
	e.createTicket("test-0001", "Test ticket")
	e.run("status", "0001", "in_progress").assertSuccess()
	e.assertTicketHasField("test-0001", "status", "in_progress")
}

// ============================================================================
// Dependencies Tests
// ============================================================================

func TestDependencies(t *testing.T) {
	t.Run("add dependency", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.run("dep", "task-0001", "task-0002").assertSuccess()
		e.assertOutput("Added dependency: task-0001 -> task-0002")
		e.assertTicketHasDep("task-0001", "task-0002")
	})

	t.Run("add dependency idempotent", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.addDep("task-0001", "task-0002")
		e.run("dep", "task-0001", "task-0002").assertSuccess()
		e.assertOutput("Dependency already exists")
	})

	t.Run("remove dependency", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.addDep("task-0001", "task-0002")
		e.run("undep", "task-0001", "task-0002").assertSuccess()
		e.assertOutput("Removed dependency: task-0001 -/-> task-0002")
		e.assertTicketNotHasDep("task-0001", "task-0002")
	})

	t.Run("remove non-existent dependency", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.run("undep", "task-0001", "task-0002").assertFail()
		e.assertOutput("Dependency not found")
	})

	t.Run("add dependency non-existent ticket", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.run("dep", "task-0001", "nonexistent").assertFail()
		e.assertOutputContains("Error: ticket 'nonexistent' not found")
	})

	t.Run("add dependency from non-existent ticket", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.run("dep", "nonexistent", "task-0001").assertFail()
		e.assertOutputContains("Error: ticket 'nonexistent' not found")
	})
}

func TestDepTree(t *testing.T) {
	t.Run("basic tree", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.createTicket("task-0003", "Another task")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0002", "task-0003")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOutputContains("task-0001")
		e.assertOutputContains("task-0002")
		e.assertOutputContains("task-0003")
	})

	t.Run("tree shows status and title", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.addDep("task-0001", "task-0002")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOutputContains("[open]")
		e.assertOutputContains("Main task")
		e.assertOutputContains("Dependency task")
	})

	t.Run("tree uses box-drawing characters", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.addDep("task-0001", "task-0002")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertTreeFormat()
	})

	t.Run("tree handles cycles", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency task")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0002", "task-0001")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOutputContains("task-0001")
		e.assertOutputContains("task-0002")
	})

	t.Run("tree sorted by depth then ID", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Root")
		e.createTicket("task-0002", "Child B shallow")
		e.createTicket("task-0003", "Child A shallow")
		e.createTicket("task-0004", "Child C deep")
		e.createTicket("task-0005", "Grandchild")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0001", "task-0003")
		e.addDep("task-0001", "task-0004")
		e.addDep("task-0004", "task-0005")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOrderInOutput("task-0002", "task-0003")
		e.assertOrderInOutput("task-0003", "task-0004")
	})

	t.Run("tree with multiple children", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dep A")
		e.createTicket("task-0003", "Dep B")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0001", "task-0003")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOutputContains("task-0002")
		e.assertOutputContains("task-0003")
	})

	t.Run("tree full flag shows all occurrences", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Root")
		e.createTicket("task-0002", "Child A")
		e.createTicket("task-0003", "Child B")
		e.createTicket("task-0004", "Shared grandchild")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0001", "task-0003")
		e.addDep("task-0002", "task-0004")
		e.addDep("task-0003", "task-0004")
		e.run("dep", "tree", "--full", "task-0001").assertSuccess()
		e.assertOutputContains("task-0004")
	})

	t.Run("tree sorted by ID when same depth", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Root")
		e.createTicket("task-0005", "Child E")
		e.createTicket("task-0002", "Child B")
		e.createTicket("task-0004", "Child D")
		e.createTicket("task-0003", "Child C")
		e.addDep("task-0001", "task-0005")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0001", "task-0004")
		e.addDep("task-0001", "task-0003")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOrderInOutput("task-0002", "task-0003")
		e.assertOrderInOutput("task-0003", "task-0004")
		e.assertOrderInOutput("task-0004", "task-0005")
	})

	t.Run("tree complex multi-level sorting", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Root")
		e.createTicket("task-0010", "Shallow C")
		e.createTicket("task-0005", "Shallow A")
		e.createTicket("task-0008", "Shallow B")
		e.createTicket("task-0020", "Deep B")
		e.createTicket("task-0015", "Deep A")
		e.createTicket("task-0025", "Deepest")
		e.addDep("task-0001", "task-0010")
		e.addDep("task-0001", "task-0005")
		e.addDep("task-0001", "task-0008")
		e.addDep("task-0001", "task-0020")
		e.addDep("task-0001", "task-0015")
		e.addDep("task-0020", "task-0025")
		e.addDep("task-0015", "task-0025")
		e.run("dep", "tree", "task-0001").assertSuccess()
		e.assertOrderInOutput("task-0005", "task-0008")
		e.assertOrderInOutput("task-0008", "task-0010")
		e.assertOrderInOutput("task-0010", "task-0015")
		e.assertOrderInOutput("task-0015", "task-0020")
	})
}

// ============================================================================
// Links Tests
// ============================================================================

func TestLinks(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testEnv)
		args       []string
		wantFail   bool
		wantOutput string
		check      func(*testEnv)
	}{
		{
			name: "link two tickets",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
				e.createTicket("link-0002", "Second ticket")
			},
			args:       []string{"link", "link-0001", "link-0002"},
			wantOutput: "Added 2 link(s) between 2 tickets",
			check: func(e *testEnv) {
				e.assertTicketHasLink("link-0001", "link-0002")
				e.assertTicketHasLink("link-0002", "link-0001")
			},
		},
		{
			name: "link three tickets",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
				e.createTicket("link-0002", "Second ticket")
				e.createTicket("link-0003", "Third ticket")
			},
			args:       []string{"link", "link-0001", "link-0002", "link-0003"},
			wantOutput: "Added 6 link(s) between 3 tickets",
			check: func(e *testEnv) {
				e.assertTicketHasLink("link-0001", "link-0002")
				e.assertTicketHasLink("link-0001", "link-0003")
				e.assertTicketHasLink("link-0002", "link-0001")
			},
		},
		{
			name: "link idempotent",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
				e.createTicket("link-0002", "Second ticket")
				e.addBidirectionalLink("link-0001", "link-0002")
			},
			args:       []string{"link", "link-0001", "link-0002"},
			wantOutput: "All links already exist",
		},
		{
			name: "unlink two tickets",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
				e.createTicket("link-0002", "Second ticket")
				e.addBidirectionalLink("link-0001", "link-0002")
			},
			args:       []string{"unlink", "link-0001", "link-0002"},
			wantOutput: "Removed link: link-0001 <-> link-0002",
			check: func(e *testEnv) {
				e.assertTicketNotHasLink("link-0001", "link-0002")
				e.assertTicketNotHasLink("link-0002", "link-0001")
			},
		},
		{
			name: "unlink non-existent",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
				e.createTicket("link-0002", "Second ticket")
			},
			args:       []string{"unlink", "link-0001", "link-0002"},
			wantFail:   true,
			wantOutput: "Link not found",
		},
		{
			name: "link with non-existent ticket",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
			},
			args:       []string{"link", "link-0001", "nonexistent"},
			wantFail:   true,
			wantOutput: "Error: ticket 'nonexistent' not found",
		},
		{
			name: "partial linking adds only new links",
			setup: func(e *testEnv) {
				e.createTicket("link-0001", "First ticket")
				e.createTicket("link-0002", "Second ticket")
				e.createTicket("link-0003", "Third ticket")
				e.addBidirectionalLink("link-0001", "link-0002")
			},
			args:       []string{"link", "link-0001", "link-0002", "link-0003"},
			wantOutput: "Added 4 link(s) between 3 tickets",
			check: func(e *testEnv) {
				e.assertTicketHasLink("link-0001", "link-0003")
				e.assertTicketHasLink("link-0003", "link-0001")
				e.assertTicketHasLink("link-0002", "link-0003")
				e.assertTicketHasLink("link-0003", "link-0002")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			e.initTicketsDir()
			if tt.setup != nil {
				tt.setup(e)
			}

			e.run(tt.args...)

			if tt.wantFail {
				e.assertFail()
			} else {
				e.assertSuccess()
			}

			if tt.wantOutput != "" {
				e.assertOutputContains(tt.wantOutput)
			}

			if tt.check != nil {
				tt.check(e)
			}
		})
	}
}

// ============================================================================
// Listing Tests
// ============================================================================

func TestList(t *testing.T) {
	t.Run("list all tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("list-0001", "First ticket")
		e.createTicket("list-0002", "Second ticket")
		e.run("ls").assertSuccess()
		e.assertOutputContains("list-0001")
		e.assertOutputContains("list-0002")
	})

	t.Run("list alias works", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("list-0001", "First ticket")
		e.run("list").assertSuccess()
		e.assertOutputContains("list-0001")
	})

	t.Run("list format", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("list-0001", "My ticket")
		e.run("ls").assertSuccess()
		e.assertOutputMatchesPattern(`list-0001\s+\[open\]\s+-\s+My ticket`)
	})

	t.Run("list with status filter", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("list-0001", "Open ticket")
		e.createTicket("list-0002", "Closed ticket")
		e.setStatus("list-0002", "closed")
		e.run("ls", "--status=open").assertSuccess()
		e.assertOutputContains("list-0001")
		e.assertOutputNotContains("list-0002")
	})

	t.Run("list with status filter space-separated", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("list-0001", "Open ticket")
		e.createTicket("list-0002", "Closed ticket")
		e.setStatus("list-0002", "closed")
		e.run("ls", "--status", "open").assertSuccess()
		e.assertOutputContains("list-0001")
		e.assertOutputNotContains("list-0002")
	})

	t.Run("list shows dependencies", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("list-0001", "Main ticket")
		e.createTicket("list-0002", "Dep ticket")
		e.addDep("list-0001", "list-0002")
		e.run("ls").assertSuccess()
		e.assertOutputContains("<- [list-0002]")
	})

	t.Run("list with no tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.initTicketsDir()
		e.run("ls").assertOutputEmpty()
	})
}

func TestReady(t *testing.T) {
	t.Run("shows tickets with no deps", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("ready-001", "Ready ticket")
		e.run("ready").assertSuccess()
		e.assertOutputContains("ready-001")
	})

	t.Run("shows tickets with closed deps", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("ready-001", "Unblocked ticket")
		e.createTicket("ready-002", "Dependency")
		e.addDep("ready-001", "ready-002")
		e.setStatus("ready-002", "closed")
		e.run("ready").assertSuccess()
		e.assertOutputContains("ready-001")
	})

	t.Run("excludes tickets with unclosed deps", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("ready-001", "Blocked ticket")
		e.createTicket("ready-002", "Open dependency")
		e.addDep("ready-001", "ready-002")
		e.run("ready").assertSuccess()
		e.assertOutputNotContains("ready-001")
		e.assertOutputContains("ready-002")
	})

	t.Run("excludes closed tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("ready-001", "Closed ticket")
		e.setStatus("ready-001", "closed")
		e.run("ready").assertSuccess()
		e.assertOutputNotContains("ready-001")
	})

	t.Run("shows priority in output", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("ready-001", "Priority ticket")
		e.run("ready").assertSuccess()
		e.assertOutputMatchesPattern(`ready-001\s+\[P2\]\[open\]\s+-\s+Priority ticket`)
	})

	t.Run("sorts by priority then ID", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicketFull("ready-003", "Low priority", "open", 3, "", nil)
		e.createTicketFull("ready-001", "High priority", "open", 1, "", nil)
		e.createTicketFull("ready-002", "Also high priority", "open", 1, "", nil)
		e.run("ready").assertSuccess()
		e.assertOutputLineContains(1, "ready-001")
		e.assertOutputLineContains(2, "ready-002")
		e.assertOutputLineContains(3, "ready-003")
	})
}

func TestBlocked(t *testing.T) {
	t.Run("shows tickets with unclosed deps", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("block-001", "Blocked ticket")
		e.createTicket("block-002", "Blocker ticket")
		e.addDep("block-001", "block-002")
		e.run("blocked").assertSuccess()
		e.assertOutputContains("block-001")
		e.assertOutputContains("<- [block-002]")
	})

	t.Run("excludes tickets with all deps closed", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("block-001", "Unblocked ticket")
		e.createTicket("block-002", "Closed blocker")
		e.addDep("block-001", "block-002")
		e.setStatus("block-002", "closed")
		e.run("blocked").assertSuccess()
		e.assertOutputNotContains("block-001")
	})

	t.Run("excludes closed tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("block-001", "Closed blocked")
		e.createTicket("block-002", "Blocker")
		e.addDep("block-001", "block-002")
		e.setStatus("block-001", "closed")
		e.run("blocked").assertSuccess()
		e.assertOutputNotContains("block-001")
	})

	t.Run("shows only unclosed blockers", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("block-001", "Blocked ticket")
		e.createTicket("block-002", "Open blocker")
		e.createTicket("block-003", "Closed blocker")
		e.addDep("block-001", "block-002")
		e.addDep("block-001", "block-003")
		e.setStatus("block-003", "closed")
		e.run("blocked").assertSuccess()
		e.assertOutputContains("<- [block-002]")
		e.assertOutputNotContains("block-003")
	})
}

func TestClosed(t *testing.T) {
	t.Run("shows recently closed tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("done-0001", "Done ticket")
		e.setStatus("done-0001", "closed")
		e.run("closed").assertSuccess()
		e.assertOutputContains("done-0001")
		e.assertOutputContains("[closed]")
		e.assertOutputContains("Done ticket")
	})

	t.Run("respects limit", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("done-0001", "First done")
		e.createTicket("done-0002", "Second done")
		e.setStatus("done-0001", "closed")
		e.setStatus("done-0002", "closed")
		e.run("closed", "--limit=1").assertSuccess()
		e.assertOutputLineCount(1)
	})

	t.Run("excludes open tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("done-0001", "Open ticket")
		e.run("closed").assertSuccess()
		e.assertOutputNotContains("done-0001")
	})
}

// ============================================================================
// Show Tests
// ============================================================================

func TestShow(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(*testEnv)
		args            []string
		wantFail        bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "displays ticket content",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Test ticket")
			},
			args:         []string{"show", "show-001"},
			wantContains: []string{"id: show-001", "# Test ticket"},
		},
		{
			name: "displays all frontmatter fields",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Full ticket")
			},
			args:         []string{"show", "show-001"},
			wantContains: []string{"status: open", "deps: []", "links: []", "type: task", "priority: 2"},
		},
		{
			name: "displays blockers section",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Blocked ticket")
				e.createTicket("show-002", "Blocker ticket")
				e.addDep("show-001", "show-002")
			},
			args:         []string{"show", "show-001"},
			wantContains: []string{"## Blockers", "show-002 [open] Blocker ticket"},
		},
		{
			name: "hides blockers when all deps closed",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Unblocked ticket")
				e.createTicket("show-002", "Closed blocker")
				e.addDep("show-001", "show-002")
				e.setStatus("show-002", "closed")
			},
			args:            []string{"show", "show-001"},
			wantNotContains: []string{"## Blockers"},
		},
		{
			name: "displays blocking section",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Blocker")
				e.createTicket("show-002", "Blocked")
				e.addDep("show-002", "show-001")
			},
			args:         []string{"show", "show-001"},
			wantContains: []string{"## Blocking", "show-002 [open] Blocked"},
		},
		{
			name: "displays children section",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Parent")
				e.createTicketFull("show-002", "Child", "open", 2, "show-001", nil)
			},
			args:         []string{"show", "show-001"},
			wantContains: []string{"## Children", "show-002 [open] Child"},
		},
		{
			name: "displays linked section",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "First")
				e.createTicket("show-002", "Second")
				e.addBidirectionalLink("show-001", "show-002")
			},
			args:         []string{"show", "show-001"},
			wantContains: []string{"## Linked", "show-002 [open] Second"},
		},
		{
			name: "enhances parent field with title",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Parent ticket")
				e.createTicketFull("show-002", "Child ticket", "open", 2, "show-001", nil)
			},
			args:         []string{"show", "show-002"},
			wantContains: []string{"parent: show-001", "# Parent ticket"},
		},
		{
			name:         "non-existent ticket",
			setup:        func(e *testEnv) { e.initTicketsDir() },
			args:         []string{"show", "nonexistent"},
			wantFail:     true,
			wantContains: []string{"Error: ticket 'nonexistent' not found"},
		},
		{
			name: "partial ID",
			setup: func(e *testEnv) {
				e.createTicket("show-001", "Test ticket")
			},
			args:         []string{"show", "001"},
			wantContains: []string{"id: show-001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			if tt.setup != nil {
				tt.setup(e)
			}

			e.run(tt.args...)

			if tt.wantFail {
				e.assertFail()
			} else {
				e.assertSuccess()
			}

			for _, text := range tt.wantContains {
				e.assertOutputContains(text)
			}
			for _, text := range tt.wantNotContains {
				e.assertOutputNotContains(text)
			}
		})
	}
}

// ============================================================================
// Notes Tests
// ============================================================================

func TestNotes(t *testing.T) {
	t.Run("add a note", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("note-0001", "Test ticket")
		e.run("add-note", "note-0001", "This is my note").assertSuccess()
		e.assertOutput("Note added to note-0001")
		e.assertTicketContains("note-0001", "## Notes")
		e.assertTicketContains("note-0001", "This is my note")
	})

	t.Run("note has timestamp", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("note-0001", "Test ticket")
		e.run("add-note", "note-0001", "Timestamped note").assertSuccess()
		// Check for timestamp pattern in ticket
		path := filepath.Join(e.ticketsDir, "note-0001.md")
		content, _ := os.ReadFile(path)
		pattern := `\*\*\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\*\*`
		if !regexp.MustCompile(pattern).MatchString(string(content)) {
			t.Error("note does not have timestamp")
		}
	})

	t.Run("add multiple notes", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("note-0001", "Test ticket")
		e.run("add-note", "note-0001", "First note")
		e.run("add-note", "note-0001", "Second note")
		e.assertTicketContains("note-0001", "First note")
		e.assertTicketContains("note-0001", "Second note")
	})

	t.Run("add note to non-existent ticket", func(t *testing.T) {
		e := newTestEnv(t)
		e.initTicketsDir()
		e.run("add-note", "nonexistent", "My note").assertFail()
		e.assertOutputContains("Error: ticket 'nonexistent' not found")
	})

	t.Run("add note with partial ID", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("note-0001", "Test ticket")
		e.run("add-note", "0001", "Partial ID note").assertSuccess()
		e.assertOutput("Note added to note-0001")
	})

	t.Run("add note to ticket with existing notes section", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("note-0001", "Test ticket")
		e.run("add-note", "note-0001", "First note").assertSuccess()
		e.run("add-note", "note-0001", "Additional note").assertSuccess()
		e.assertTicketContains("note-0001", "First note")
		e.assertTicketContains("note-0001", "Additional note")
	})

	t.Run("add note with empty string adds timestamp-only note", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("note-0001", "Test ticket")
		e.run("add-note", "note-0001", "").assertSuccess()
		e.assertOutput("Note added to note-0001")
		e.assertTicketContains("note-0001", "## Notes")
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestQuery(t *testing.T) {
	t.Run("query all tickets as JSONL", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("query-001", "First ticket")
		e.createTicket("query-002", "Second ticket")
		e.run("query").assertSuccess()
		e.assertValidJSONL()
		e.assertOutputContains("query-001")
		e.assertOutputContains("query-002")
	})

	t.Run("query with filter", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("query-001", "Open ticket")
		e.createTicket("query-002", "Closed ticket")
		e.setStatus("query-002", "closed")
		e.run("query", `.status == "open"`).assertSuccess()
		e.assertOutputContains("query-001")
		e.assertOutputNotContains("query-002")
	})

	t.Run("query includes all fields", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("query-001", "Full ticket")
		e.run("query").assertSuccess()
		e.assertJSONLHasField("id")
		e.assertJSONLHasField("status")
		e.assertJSONLHasField("deps")
		e.assertJSONLHasField("links")
		e.assertJSONLHasField("type")
		e.assertJSONLHasField("priority")
	})

	t.Run("query with no tickets", func(t *testing.T) {
		e := newTestEnv(t)
		e.initTicketsDir()
		e.run("query").assertOutputEmpty()
	})

	t.Run("query deps is JSON array", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("query-001", "Deps ticket")
		e.createTicket("query-002", "Dependency")
		e.addDep("query-001", "query-002")
		e.run("query").assertSuccess()
		// Verify deps is an array by parsing JSONL
		lines := strings.Split(e.stdout(), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				continue
			}
			if deps, ok := data["deps"]; ok {
				if _, isArray := deps.([]any); !isArray {
					t.Errorf("deps is not an array: %T", deps)
				}
			}
		}
	})
}

// ============================================================================
// ID Resolution Tests
// ============================================================================

func TestIDResolution(t *testing.T) {
	tests := []struct {
		name         string
		ticketID     string
		title        string
		partialID    string
		wantContains string
		wantFail     bool
	}{
		{
			name:         "exact match",
			ticketID:     "abc-1234",
			title:        "Test ticket",
			partialID:    "abc-1234",
			wantContains: "id: abc-1234",
		},
		{
			name:         "suffix match",
			ticketID:     "abc-1234",
			title:        "Test ticket",
			partialID:    "1234",
			wantContains: "id: abc-1234",
		},
		{
			name:         "prefix match",
			ticketID:     "abc-1234",
			title:        "Test ticket",
			partialID:    "abc",
			wantContains: "id: abc-1234",
		},
		{
			name:         "substring match",
			ticketID:     "abc-1234",
			title:        "Test ticket",
			partialID:    "c-12",
			wantContains: "id: abc-1234",
		},
		{
			name:         "non-existent",
			ticketID:     "abc-1234",
			title:        "Test ticket",
			partialID:    "nonexistent",
			wantFail:     true,
			wantContains: "Error: ticket 'nonexistent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv(t)
			e.createTicket(tt.ticketID, tt.title)
			e.run("show", tt.partialID)

			if tt.wantFail {
				e.assertFail()
			} else {
				e.assertSuccess()
			}
			e.assertOutputContains(tt.wantContains)
		})
	}
}

func TestIDResolutionAmbiguous(t *testing.T) {
	e := newTestEnv(t)
	e.createTicket("abc-1234", "First ticket")
	e.createTicket("abc-5678", "Second ticket")
	e.run("show", "abc").assertFail()
	e.assertOutputContains("Error: ambiguous ID 'abc' matches multiple tickets")
}

func TestIDResolutionExactPrecedence(t *testing.T) {
	e := newTestEnv(t)
	e.createTicket("abc", "Short ID ticket")
	e.createTicket("abc-1234", "Long ID ticket")
	e.run("show", "abc").assertSuccess()
	e.assertOutputContains("id: abc")
	e.assertOutputContains("Short ID ticket")
}

func TestIDResolutionWithCommands(t *testing.T) {
	t.Run("partial ID works with status command", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("test-9999", "Test ticket")
		e.run("status", "9999", "in_progress").assertSuccess()
		e.assertTicketHasField("test-9999", "status", "in_progress")
	})

	t.Run("partial ID works with dep command", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("dep-aaaa", "Main")
		e.createTicket("dep-bbbb", "Dependency")
		e.run("dep", "aaaa", "bbbb").assertSuccess()
		e.assertTicketHasDep("dep-aaaa", "dep-bbbb")
	})

	t.Run("partial ID works with link command", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("link-cccc", "First")
		e.createTicket("link-dddd", "Second")
		e.run("link", "cccc", "dddd").assertSuccess()
		e.assertTicketHasLink("link-cccc", "link-dddd")
	})
}

// ============================================================================
// Directory Tests
// ============================================================================

func TestDirectory(t *testing.T) {
	t.Run("find tickets in current directory", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("test-0001", "Test ticket")
		e.run("ls").assertSuccess()
		e.assertOutputContains("test-0001")
	})

	t.Run("find tickets in parent directory", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("test-0001", "Test ticket")
		e.runInSubdir("src/components", "ls").assertSuccess()
		e.assertOutputContains("test-0001")
	})

	t.Run("find tickets in grandparent directory", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("test-0001", "Test ticket")
		e.runInSubdir("src/components/ui", "ls").assertSuccess()
		e.assertOutputContains("test-0001")
	})

	t.Run("error when no tickets directory", func(t *testing.T) {
		e := newTestEnv(t)
		// Don't create tickets dir
		e.run("ls").assertFail()
		e.assertOutputContains("no .tickets directory found")
	})

	t.Run("TICKETS_DIR env var takes priority", func(t *testing.T) {
		e := newTestEnv(t)
		// Create ticket in main .tickets
		e.createTicket("parent-0001", "Parent ticket")

		// Create separate tickets dir
		otherDir := filepath.Join(e.dir, "other-tickets")
		os.MkdirAll(otherDir, 0755)
		otherContent := `---
id: other-0001
status: open
deps: []
links: []
created: 2024-01-01T00:00:00Z
type: task
priority: 2
---
# Other ticket

Description
`
		os.WriteFile(filepath.Join(otherDir, "other-0001.md"), []byte(otherContent), 0644)

		// Run with TICKETS_DIR set
		e.runWithEnv(map[string]string{"TICKETS_DIR": otherDir}, "ls")
		e.assertSuccess()
		e.assertOutputContains("other-0001")
		e.assertOutputNotContains("parent-0001")
	})

	t.Run("help works without tickets directory", func(t *testing.T) {
		e := newTestEnv(t)
		// Don't create tickets dir
		e.run("help").assertSuccess()
		e.assertOutputContains("minimal ticket system")
	})

	t.Run("create initializes in current directory when no parent has tickets", func(t *testing.T) {
		e := newTestEnv(t)
		// Don't create tickets dir, run from subdir
		subdir := filepath.Join(e.dir, "new-project")
		os.MkdirAll(subdir, 0755)

		// Reset buffers and set writers
		e.stdoutBuf.Reset()
		e.stderrBuf.Reset()
		stdout = e.stdoutBuf
		stderr = e.stderrBuf

		// Change to subdir and run (don't use e.run which resets to e.dir)
		os.Chdir(subdir)
		e.err = run([]string{"create", "First ticket"})

		e.assertSuccess()
		e.assertOutputMatchesIDPattern()
		// Check .tickets was created in subdir
		if _, err := os.Stat(filepath.Join(subdir, ".tickets")); os.IsNotExist(err) {
			t.Error(".tickets directory was not created in current directory")
		}
	})

	t.Run("error when no tickets directory in any parent", func(t *testing.T) {
		e := newTestEnv(t)
		// Don't create tickets dir, go deep
		subdir := filepath.Join(e.dir, "orphan", "deep", "path")
		os.MkdirAll(subdir, 0755)
		os.Chdir(subdir)
		e.run("ready").assertFail()
		e.assertOutputContains("no .tickets directory found")
	})

	t.Run("show command works from subdirectory", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("test-0001", "Test ticket")
		e.runInSubdir("src", "show", "test-0001").assertSuccess()
		e.assertOutputContains("id: test-0001")
	})

	t.Run("dep command works from subdirectory", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Main task")
		e.createTicket("task-0002", "Dependency")
		e.runInSubdir("lib", "dep", "task-0001", "task-0002").assertSuccess()
		e.assertOutputContains("Added dependency")
	})
}

// ============================================================================
// Edit Tests
// ============================================================================

func TestEdit(t *testing.T) {
	t.Run("edit in non-TTY shows file path", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("edit-0001", "Editable ticket")
		e.run("edit", "edit-0001").assertSuccess()
		e.assertOutputContains("Edit ticket file:")
		e.assertOutputContains(".tickets/edit-0001.md")
	})

	t.Run("edit non-existent ticket", func(t *testing.T) {
		e := newTestEnv(t)
		e.initTicketsDir()
		e.run("edit", "nonexistent").assertFail()
		e.assertOutputContains("Error: ticket 'nonexistent' not found")
	})

	t.Run("edit with partial ID", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("edit-0001", "Editable ticket")
		e.run("edit", "0001").assertSuccess()
		e.assertOutputContains("edit-0001.md")
	})
}

// ============================================================================
// Dep Cycle Tests
// ============================================================================

func TestDepCycle(t *testing.T) {
	t.Run("no cycles", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Task 1")
		e.createTicket("task-0002", "Task 2")
		e.addDep("task-0001", "task-0002")
		e.run("dep", "cycle").assertSuccess()
		e.assertOutputContains("No dependency cycles found")
	})

	t.Run("detects simple cycle", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("task-0001", "Task 1")
		e.createTicket("task-0002", "Task 2")
		e.addDep("task-0001", "task-0002")
		e.addDep("task-0002", "task-0001")
		e.run("dep", "cycle").assertSuccess()
		e.assertOutputContains("Cycle")
		e.assertOutputContains("task-0001")
		e.assertOutputContains("task-0002")
	})
}

func TestJSONFlag(t *testing.T) {
	parseJSON := func(t *testing.T, line string) map[string]interface{} {
		t.Helper()
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid JSON %q: %v", line, err)
		}
		return obj
	}

	parseJSONArray := func(t *testing.T, output string) []map[string]interface{} {
		t.Helper()
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &arr); err != nil {
			t.Fatalf("invalid JSON array %q: %v", output, err)
		}
		return arr
	}

	assertJSONFields := func(t *testing.T, obj map[string]interface{}, id, title, status string) {
		t.Helper()
		fields := []string{"id", "title", "status", "priority", "issue_type", "owner",
			"created_at", "created_by", "updated_at", "dependency_count", "dependent_count", "comment_count"}
		for _, f := range fields {
			if _, ok := obj[f]; !ok {
				t.Errorf("missing field %q in JSON output", f)
			}
		}
		if obj["id"] != id {
			t.Errorf("id: got %v, want %s", obj["id"], id)
		}
		if obj["title"] != title {
			t.Errorf("title: got %v, want %s", obj["title"], title)
		}
		if obj["status"] != status {
			t.Errorf("status: got %v, want %s", obj["status"], status)
		}
	}

	t.Run("list --json outputs all required fields", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("tk-0001", "My Ticket")
		e.run("list", "--json").assertSuccess()
		objs := parseJSONArray(t, e.stdout())
		if len(objs) != 1 {
			t.Fatalf("expected 1 ticket, got %d", len(objs))
		}
		obj := objs[0]
		assertJSONFields(t, obj, "tk-0001", "My Ticket", "open")
	})

	t.Run("list --json dependency_count", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("tk-0001", "Ticket 1")
		e.createTicket("tk-0002", "Ticket 2")
		e.addDep("tk-0001", "tk-0002")
		e.run("list", "--json").assertSuccess()
		objs := parseJSONArray(t, e.stdout())
		if len(objs) != 2 {
			t.Fatalf("expected 2 tickets, got %d", len(objs))
		}
		for _, obj := range objs {
			if obj["id"] == "tk-0001" {
				if obj["dependency_count"].(float64) != 1 {
					t.Errorf("tk-0001 dependency_count: got %v, want 1", obj["dependency_count"])
				}
				if obj["dependent_count"].(float64) != 0 {
					t.Errorf("tk-0001 dependent_count: got %v, want 0", obj["dependent_count"])
				}
			}
			if obj["id"] == "tk-0002" {
				if obj["dependency_count"].(float64) != 0 {
					t.Errorf("tk-0002 dependency_count: got %v, want 0", obj["dependency_count"])
				}
				if obj["dependent_count"].(float64) != 1 {
					t.Errorf("tk-0002 dependent_count: got %v, want 1", obj["dependent_count"])
				}
			}
		}
	})

	t.Run("list --json comment_count", func(t *testing.T) {
		e := newTestEnv(t)
		e.run("create", "Noted Ticket").assertSuccess()
		id := e.lastID
		e.run("add-note", id, "first note").assertSuccess()
		e.run("add-note", id, "second note").assertSuccess()
		e.run("list", "--json").assertSuccess()
		objs := parseJSONArray(t, e.stdout())
		if len(objs) != 1 {
			t.Fatalf("expected 1 ticket, got %d", len(objs))
		}
		obj := objs[0]
		if obj["comment_count"].(float64) != 2 {
			t.Errorf("comment_count: got %v, want 2", obj["comment_count"])
		}
	})

	t.Run("ready --json", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("tk-0001", "Ready Ticket")
		e.run("ready", "--json").assertSuccess()
		objs := parseJSONArray(t, e.stdout())
		if len(objs) != 1 {
			t.Fatalf("expected 1 ticket, got %d", len(objs))
		}
		obj := objs[0]
		assertJSONFields(t, obj, "tk-0001", "Ready Ticket", "open")
	})

	t.Run("closed --json", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("tk-0001", "Closed Ticket")
		e.setStatus("tk-0001", "closed")
		e.run("closed", "--json").assertSuccess()
		objs := parseJSONArray(t, e.stdout())
		if len(objs) != 1 {
			t.Fatalf("expected 1 ticket, got %d", len(objs))
		}
		obj := objs[0]
		assertJSONFields(t, obj, "tk-0001", "Closed Ticket", "closed")
	})

	t.Run("show --json", func(t *testing.T) {
		e := newTestEnv(t)
		e.createTicket("tk-0001", "Show Ticket")
		e.run("show", "tk-0001", "--json").assertSuccess()
		obj := parseJSON(t, strings.TrimSpace(e.stdout()))
		assertJSONFields(t, obj, "tk-0001", "Show Ticket", "open")
	})

	t.Run("show --json issue_type", func(t *testing.T) {
		e := newTestEnv(t)
		e.run("create", "--type", "bug", "Bug Ticket").assertSuccess()
		id := e.lastID
		e.run("show", id, "--json").assertSuccess()
		obj := parseJSON(t, strings.TrimSpace(e.stdout()))
		if obj["issue_type"] != "bug" {
			t.Errorf("issue_type: got %v, want bug", obj["issue_type"])
		}
	})
}
