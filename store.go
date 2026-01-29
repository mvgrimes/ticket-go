package ticket

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store handles ticket storage operations.
type Store struct {
	Dir string
}

// FindTicketsDir finds the .tickets directory by walking parent directories.
// If TICKETS_DIR env var is set, it uses that instead.
func FindTicketsDir() (string, error) {
	// Explicit env var takes priority
	if dir := os.Getenv("TICKETS_DIR"); dir != "" {
		return dir, nil
	}

	// Walk parents looking for .tickets
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for dir != "/" {
		ticketsDir := filepath.Join(dir, ".tickets")
		if info, err := os.Stat(ticketsDir); err == nil && info.IsDir() {
			return ticketsDir, nil
		}
		dir = filepath.Dir(dir)
	}

	// Check root too
	if info, err := os.Stat("/.tickets"); err == nil && info.IsDir() {
		return "/.tickets", nil
	}

	return "", fmt.Errorf("no .tickets directory found (searched parent directories)")
}

// NewStore creates a new store. If the directory doesn't exist and create is true,
// it will be created when needed.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// EnsureDir creates the tickets directory if it doesn't exist.
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.Dir, 0755)
}

// TicketPath returns the full path to a ticket file.
func (s *Store) TicketPath(id string) string {
	return filepath.Join(s.Dir, id+".md")
}

// ResolveID resolves a partial ID to a full ticket ID.
// Returns an error if the ID is ambiguous or not found.
func (s *Store) ResolveID(partial string) (string, error) {
	// Try exact match first
	exactPath := s.TicketPath(partial)
	if _, err := os.Stat(exactPath); err == nil {
		return partial, nil
	}

	// Try partial match (anywhere in filename)
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return "", fmt.Errorf("ticket '%s' not found", partial)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		if strings.Contains(id, partial) {
			matches = append(matches, id)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("ticket '%s' not found", partial)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous ID '%s' matches multiple tickets", partial)
	}
}

// GetTicketPath resolves a partial ID and returns the full path.
func (s *Store) GetTicketPath(partial string) (string, error) {
	id, err := s.ResolveID(partial)
	if err != nil {
		return "", err
	}
	return s.TicketPath(id), nil
}

// Load loads a ticket from file.
func (s *Store) Load(id string) (*Ticket, error) {
	path := s.TicketPath(id)
	return LoadTicket(path)
}

// LoadByPartialID loads a ticket by partial ID.
func (s *Store) LoadByPartialID(partial string) (*Ticket, error) {
	id, err := s.ResolveID(partial)
	if err != nil {
		return nil, err
	}
	return s.Load(id)
}

// Save saves a ticket to file.
func (s *Store) Save(t *Ticket) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	path := s.TicketPath(t.ID)
	return SaveTicket(t, path)
}

// ListTickets returns all tickets in the store.
func (s *Store) ListTickets() ([]*Ticket, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tickets []*Ticket
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(s.Dir, entry.Name())
		t, err := LoadTicket(path)
		if err != nil {
			continue // Skip invalid tickets
		}
		tickets = append(tickets, t)
	}

	return tickets, nil
}

// ListTicketsByMtime returns tickets sorted by modification time (most recent first).
func (s *Store) ListTicketsByMtime(limit int) ([]*Ticket, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type ticketWithMtime struct {
		ticket *Ticket
		mtime  int64
	}

	var tickets []ticketWithMtime
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(s.Dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		t, err := LoadTicket(path)
		if err != nil {
			continue
		}

		tickets = append(tickets, ticketWithMtime{
			ticket: t,
			mtime:  info.ModTime().UnixNano(),
		})
	}

	// Sort by mtime descending
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].mtime > tickets[j].mtime
	})

	// Apply limit
	if limit > 0 && len(tickets) > limit {
		tickets = tickets[:limit]
	}

	result := make([]*Ticket, len(tickets))
	for i, t := range tickets {
		result[i] = t.ticket
	}

	return result, nil
}

// UpdateField updates a single YAML field in a ticket file.
func (s *Store) UpdateField(id, field, value string) error {
	path := s.TicketPath(id)
	return UpdateYAMLField(path, field, value)
}

// Create creates a new ticket with the given options.
func (s *Store) Create(opts CreateOptions) (*Ticket, error) {
	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	// Generate ID from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	id, err := GenerateID(cwd)
	if err != nil {
		return nil, err
	}

	// Resolve parent if specified
	if opts.Parent != "" {
		parentID, err := s.ResolveID(opts.Parent)
		if err != nil {
			return nil, err
		}
		opts.Parent = parentID
	}

	title := opts.Title
	if title == "" {
		title = "Untitled"
	}

	assignee := opts.Assignee
	if assignee == "" {
		assignee = DefaultAssignee()
	}

	priority := opts.Priority
	if !opts.PrioritySet {
		priority = 2
	}

	ticketType := opts.Type
	if ticketType == "" {
		ticketType = "task"
	}

	t := &Ticket{
		ID:          id,
		Status:      StatusOpen,
		Deps:        []string{},
		Links:       []string{},
		Created:     parseTime(ISODate()),
		Type:        ticketType,
		Priority:    priority,
		Assignee:    assignee,
		ExternalRef: opts.ExternalRef,
		Parent:      opts.Parent,
		Tags:        opts.Tags,
		Title:       title,
		Description: opts.Description,
		Design:      opts.Design,
		Acceptance:  opts.Acceptance,
	}

	if err := s.Save(t); err != nil {
		return nil, err
	}

	return t, nil
}

// AddNote appends a timestamped note to a ticket.
func (s *Store) AddNote(id, note string) error {
	path := s.TicketPath(id)

	// Read current content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// Add Notes section if missing
	if !strings.Contains(contentStr, "## Notes") {
		contentStr += "\n## Notes\n"
	}

	// Append timestamped note
	timestamp := ISODate()
	contentStr += fmt.Sprintf("\n**%s**\n\n%s\n", timestamp, note)

	return os.WriteFile(path, []byte(contentStr), 0644)
}

// HasDep checks if a ticket has a specific dependency.
func (t *Ticket) HasDep(depID string) bool {
	for _, d := range t.Deps {
		if d == depID {
			return true
		}
	}
	return false
}

// HasLink checks if a ticket has a specific link.
func (t *Ticket) HasLink(linkID string) bool {
	for _, l := range t.Links {
		if l == linkID {
			return true
		}
	}
	return false
}

// AddDep adds a dependency to a ticket file.
func (s *Store) AddDep(id, depID string) error {
	t, err := s.Load(id)
	if err != nil {
		return err
	}

	if t.HasDep(depID) {
		return nil // Already exists
	}

	t.Deps = append(t.Deps, depID)

	// Update the deps field in the file
	depsStr := formatArray(t.Deps)
	return s.UpdateField(id, "deps", depsStr)
}

// RemoveDep removes a dependency from a ticket file.
func (s *Store) RemoveDep(id, depID string) error {
	t, err := s.Load(id)
	if err != nil {
		return err
	}

	found := false
	var newDeps []string
	for _, d := range t.Deps {
		if d == depID {
			found = true
		} else {
			newDeps = append(newDeps, d)
		}
	}

	if !found {
		return fmt.Errorf("dependency not found")
	}

	depsStr := formatArray(newDeps)
	return s.UpdateField(id, "deps", depsStr)
}

// AddLink adds a link to a ticket file.
func (s *Store) AddLink(id, linkID string) error {
	t, err := s.Load(id)
	if err != nil {
		return err
	}

	if t.HasLink(linkID) {
		return nil // Already exists
	}

	t.Links = append(t.Links, linkID)

	linksStr := formatArray(t.Links)
	return s.UpdateField(id, "links", linksStr)
}

// RemoveLink removes a link from a ticket file.
func (s *Store) RemoveLink(id, linkID string) error {
	t, err := s.Load(id)
	if err != nil {
		return err
	}

	found := false
	var newLinks []string
	for _, l := range t.Links {
		if l == linkID {
			found = true
		} else {
			newLinks = append(newLinks, l)
		}
	}

	if !found {
		return fmt.Errorf("link not found")
	}

	linksStr := formatArray(newLinks)
	return s.UpdateField(id, "links", linksStr)
}

func formatArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	return "[" + strings.Join(arr, ", ") + "]"
}

// GetFileContent returns the raw file content of a ticket.
func (s *Store) GetFileContent(id string) (string, error) {
	path := s.TicketPath(id)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// AppendToFile appends content to a ticket file.
func (s *Store) AppendToFile(id string, content string) error {
	path := s.TicketPath(id)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

// FileHasSection checks if a ticket file contains a specific section header.
func (s *Store) FileHasSection(id, section string) (bool, error) {
	path := s.TicketPath(id)
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), section) {
			return true, nil
		}
	}
	return false, scanner.Err()
}
