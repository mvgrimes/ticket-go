package ticket

import (
	"fmt"
	"sort"
	"strings"
)

// ListOptions contains options for listing tickets.
type ListOptions struct {
	Status   string
	Assignee string
	Tag      string
}

// Matches returns true if the ticket matches the assignee and tag filters.
// Status is checked separately as different commands have different status logic.
func (opts ListOptions) Matches(t *Ticket) bool {
	if opts.Assignee != "" && t.Assignee != opts.Assignee {
		return false
	}
	if opts.Tag != "" && !hasTag(t.Tags, opts.Tag) {
		return false
	}
	return true
}

// ListTicketsFiltered returns tickets matching the filter options.
func (s *Store) ListTicketsFiltered(opts ListOptions) ([]*Ticket, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, err
	}

	var result []*Ticket
	for _, t := range tickets {
		if opts.Status != "" && string(t.Status) != opts.Status {
			continue
		}
		if !opts.Matches(t) {
			continue
		}
		result = append(result, t)
	}

	return result, nil
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// ReadyTickets returns tickets that are open/in_progress with all deps closed.
func (s *Store) ReadyTickets(opts ListOptions) ([]*Ticket, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, err
	}

	// Build status map
	statuses := make(map[string]Status)
	for _, t := range tickets {
		statuses[t.ID] = t.Status
	}

	var result []*Ticket
	for _, t := range tickets {
		// Only open or in_progress
		if t.Status != StatusOpen && t.Status != StatusInProgress {
			continue
		}
		if !opts.Matches(t) {
			continue
		}

		// Check if all deps are closed
		ready := true
		for _, dep := range t.Deps {
			if statuses[dep] != StatusClosed {
				ready = false
				break
			}
		}

		if ready {
			result = append(result, t)
		}
	}

	// Sort by priority, then by ID
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// BlockedTickets returns tickets that are open/in_progress with unclosed deps.
func (s *Store) BlockedTickets(opts ListOptions) ([]*Ticket, [][]string, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, nil, err
	}

	// Build status map
	statuses := make(map[string]Status)
	for _, t := range tickets {
		statuses[t.ID] = t.Status
	}

	var result []*Ticket
	var blockers [][]string

	for _, t := range tickets {
		// Only open or in_progress
		if t.Status != StatusOpen && t.Status != StatusInProgress {
			continue
		}
		if !opts.Matches(t) {
			continue
		}

		// Check for unclosed deps
		var unclosed []string
		for _, dep := range t.Deps {
			if statuses[dep] != StatusClosed {
				unclosed = append(unclosed, dep)
			}
		}

		if len(unclosed) > 0 {
			result = append(result, t)
			blockers = append(blockers, unclosed)
		}
	}

	// Sort by priority, then by ID
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].ID < result[j].ID
	})

	return result, blockers, nil
}

// ClosedTickets returns recently closed tickets.
func (s *Store) ClosedTickets(opts ListOptions, limit int) ([]*Ticket, error) {
	tickets, err := s.ListTicketsByMtime(100) // Get more to filter
	if err != nil {
		return nil, err
	}

	var result []*Ticket
	for _, t := range tickets {
		if t.Status != StatusClosed && t.Status != "done" {
			continue
		}
		if !opts.Matches(t) {
			continue
		}

		result = append(result, t)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

// ShowTicketInfo contains enhanced ticket information for display.
type ShowTicketInfo struct {
	Content    string
	Blockers   []*Ticket
	Blocking   []*Ticket
	Children   []*Ticket
	Linked     []*Ticket
	ParentInfo *Ticket
}

// GetShowInfo returns enhanced ticket information for the show command.
func (s *Store) GetShowInfo(id string) (*ShowTicketInfo, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, err
	}

	// Build maps
	ticketMap := make(map[string]*Ticket)
	for _, t := range tickets {
		ticketMap[t.ID] = t
	}

	target, ok := ticketMap[id]
	if !ok {
		return nil, fmt.Errorf("ticket '%s' not found", id)
	}

	info := &ShowTicketInfo{}

	// Get raw file content
	info.Content, err = s.GetFileContent(id)
	if err != nil {
		return nil, err
	}

	// Get parent info
	if target.Parent != "" {
		info.ParentInfo = ticketMap[target.Parent]
	}

	// Get unclosed blockers
	for _, dep := range target.Deps {
		if t := ticketMap[dep]; t != nil && t.Status != StatusClosed {
			info.Blockers = append(info.Blockers, t)
		}
	}

	// Get tickets this is blocking
	for _, t := range tickets {
		if t.Status == StatusClosed {
			continue
		}
		for _, dep := range t.Deps {
			if dep == id {
				info.Blocking = append(info.Blocking, t)
				break
			}
		}
	}

	// Get children
	for _, t := range tickets {
		if t.Parent == id {
			info.Children = append(info.Children, t)
		}
	}

	// Get linked
	for _, linkID := range target.Links {
		if t := ticketMap[linkID]; t != nil {
			info.Linked = append(info.Linked, t)
		}
	}

	return info, nil
}

// DepTreeNode represents a node in the dependency tree.
type DepTreeNode struct {
	Ticket   *Ticket
	Children []*DepTreeNode
}

// GetDepTree returns the dependency tree for a ticket.
func (s *Store) GetDepTree(id string, fullMode bool) (*DepTreeNode, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, err
	}

	// Build ticket map
	ticketMap := make(map[string]*Ticket)
	for _, t := range tickets {
		ticketMap[t.ID] = t
	}

	root, ok := ticketMap[id]
	if !ok {
		return nil, fmt.Errorf("ticket '%s' not found", id)
	}

	// Track printed nodes to avoid duplicates (unless fullMode)
	printed := make(map[string]bool)

	// Build tree recursively
	var buildTree func(t *Ticket, path map[string]bool, depth int) *DepTreeNode
	buildTree = func(t *Ticket, path map[string]bool, depth int) *DepTreeNode {
		if t == nil {
			return nil
		}

		// Cycle detection
		if path[t.ID] {
			return nil
		}

		// Duplicate detection (unless fullMode)
		if !fullMode && printed[t.ID] {
			return nil
		}

		node := &DepTreeNode{Ticket: t}
		printed[t.ID] = true

		// Add to path for cycle detection
		newPath := make(map[string]bool)
		for k, v := range path {
			newPath[k] = v
		}
		newPath[t.ID] = true

		// Collect and sort children by subtree depth, then by ID
		type childInfo struct {
			ticket *Ticket
			depth  int
		}
		var children []childInfo

		for _, depID := range t.Deps {
			if dep := ticketMap[depID]; dep != nil {
				if !newPath[depID] && (fullMode || !printed[depID]) {
					children = append(children, childInfo{
						ticket: dep,
						depth:  getSubtreeDepth(dep, ticketMap, newPath),
					})
				}
			}
		}

		// Sort children
		sort.Slice(children, func(i, j int) bool {
			if children[i].depth != children[j].depth {
				return children[i].depth < children[j].depth
			}
			return children[i].ticket.ID < children[j].ticket.ID
		})

		for _, c := range children {
			if child := buildTree(c.ticket, newPath, depth+1); child != nil {
				node.Children = append(node.Children, child)
			}
		}

		return node
	}

	return buildTree(root, make(map[string]bool), 0), nil
}

func getSubtreeDepth(t *Ticket, ticketMap map[string]*Ticket, path map[string]bool) int {
	if path[t.ID] {
		return 0
	}

	maxDepth := 0
	for _, depID := range t.Deps {
		if dep := ticketMap[depID]; dep != nil && !path[depID] {
			newPath := make(map[string]bool)
			for k, v := range path {
				newPath[k] = v
			}
			newPath[t.ID] = true
			d := getSubtreeDepth(dep, ticketMap, newPath) + 1
			if d > maxDepth {
				maxDepth = d
			}
		}
	}
	return maxDepth
}

// PrintDepTree formats a dependency tree for output.
func PrintDepTree(node *DepTreeNode, prefix string, isLast bool, isRoot bool) string {
	if node == nil || node.Ticket == nil {
		return ""
	}

	var sb strings.Builder

	if isRoot {
		sb.WriteString(fmt.Sprintf("%s [%s] %s\n",
			node.Ticket.ID, node.Ticket.Status, node.Ticket.Title))
	} else {
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		sb.WriteString(fmt.Sprintf("%s%s%s [%s] %s\n",
			prefix, connector, node.Ticket.ID, node.Ticket.Status, node.Ticket.Title))
	}

	for i, child := range node.Children {
		isChildLast := i == len(node.Children)-1
		newPrefix := prefix
		if !isRoot {
			if isLast {
				newPrefix = prefix + "    "
			} else {
				newPrefix = prefix + "│   "
			}
		}
		sb.WriteString(PrintDepTree(child, newPrefix, isChildLast, false))
	}

	return sb.String()
}

// Cycle represents a dependency cycle.
type Cycle struct {
	Path    []string
	Tickets []*Ticket
}

// FindCycles finds all dependency cycles among open tickets.
func (s *Store) FindCycles() ([]Cycle, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, err
	}

	// Build maps for open tickets only
	ticketMap := make(map[string]*Ticket)
	for _, t := range tickets {
		if t.Status != StatusClosed {
			ticketMap[t.ID] = t
		}
	}

	// DFS cycle detection with color marking
	// 0 = white (unvisited), 1 = gray (visiting), 2 = black (done)
	state := make(map[string]int)
	var cycles []Cycle
	seenCycles := make(map[string]bool)

	var dfs func(node string, path []string) *Cycle
	dfs = func(node string, path []string) *Cycle {
		t, ok := ticketMap[node]
		if !ok {
			return nil
		}

		if state[node] == 2 {
			return nil // Already fully visited
		}

		if state[node] == 1 {
			// Found a cycle - extract it
			cycleStart := -1
			for i, id := range path {
				if id == node {
					cycleStart = i
					break
				}
			}
			if cycleStart == -1 {
				return nil
			}

			cyclePath := append(path[cycleStart:], node)

			// Normalize cycle to detect duplicates
			minIdx := 0
			for i, id := range cyclePath[:len(cyclePath)-1] {
				if id < cyclePath[minIdx] {
					minIdx = i
				}
			}

			var normalized []string
			for i := 0; i < len(cyclePath)-1; i++ {
				normalized = append(normalized, cyclePath[(minIdx+i)%(len(cyclePath)-1)])
			}
			normKey := strings.Join(normalized, ",")

			if seenCycles[normKey] {
				return nil
			}
			seenCycles[normKey] = true

			var cycleTickets []*Ticket
			for _, id := range normalized {
				if t := ticketMap[id]; t != nil {
					cycleTickets = append(cycleTickets, t)
				}
			}

			return &Cycle{Path: cyclePath, Tickets: cycleTickets}
		}

		state[node] = 1 // gray
		path = append(path, node)

		for _, dep := range t.Deps {
			if cycle := dfs(dep, path); cycle != nil {
				return cycle
			}
		}

		state[node] = 2 // black
		return nil
	}

	for id := range ticketMap {
		if state[id] == 0 {
			if cycle := dfs(id, nil); cycle != nil {
				cycles = append(cycles, *cycle)
			}
		}
	}

	return cycles, nil
}

// TicketJSON represents a ticket in JSON format for the query command.
type TicketJSON struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Deps        []string `json:"deps"`
	Links       []string `json:"links"`
	Created     string   `json:"created"`
	Type        string   `json:"type"`
	Priority    int      `json:"priority"`
	Assignee    string   `json:"assignee,omitempty"`
	ExternalRef string   `json:"external-ref,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ToJSON converts a ticket to JSON format.
func (t *Ticket) ToJSON() TicketJSON {
	return TicketJSON{
		ID:          t.ID,
		Status:      string(t.Status),
		Deps:        t.Deps,
		Links:       t.Links,
		Created:     t.Created.Format("2006-01-02T15:04:05Z"),
		Type:        t.Type,
		Priority:    t.Priority,
		Assignee:    t.Assignee,
		ExternalRef: t.ExternalRef,
		Parent:      t.Parent,
		Tags:        t.Tags,
	}
}

// FormatShowInfo formats ShowTicketInfo for display output.
func FormatShowInfo(info *ShowTicketInfo) string {
	var output strings.Builder

	// Enhance content: add parent title as comment if parent exists
	content := info.Content
	if info.ParentInfo != nil {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "parent:") {
				lines[i] = line + "  # " + info.ParentInfo.Title
				break
			}
		}
		content = strings.Join(lines, "\n")
	}
	output.WriteString(content)

	// Add blockers section
	if len(info.Blockers) > 0 {
		output.WriteString("\n## Blockers\n\n")
		for _, b := range info.Blockers {
			fmt.Fprintf(&output, "- %s [%s] %s\n", b.ID, b.Status, b.Title)
		}
	}

	// Add blocking section
	if len(info.Blocking) > 0 {
		output.WriteString("\n## Blocking\n\n")
		for _, b := range info.Blocking {
			fmt.Fprintf(&output, "- %s [%s] %s\n", b.ID, b.Status, b.Title)
		}
	}

	// Add children section
	if len(info.Children) > 0 {
		output.WriteString("\n## Children\n\n")
		for _, c := range info.Children {
			fmt.Fprintf(&output, "- %s [%s] %s\n", c.ID, c.Status, c.Title)
		}
	}

	// Add linked section
	if len(info.Linked) > 0 {
		output.WriteString("\n## Linked\n\n")
		for _, l := range info.Linked {
			fmt.Fprintf(&output, "- %s [%s] %s\n", l.ID, l.Status, l.Title)
		}
	}

	return output.String()
}

// LinkTickets links multiple tickets together symmetrically.
func (s *Store) LinkTickets(ids []string) (int, error) {
	// Resolve all IDs first
	resolvedIDs := make([]string, len(ids))
	for i, id := range ids {
		resolved, err := s.ResolveID(id)
		if err != nil {
			return 0, err
		}
		resolvedIDs[i] = resolved
	}

	// Add links between all pairs
	count := 0
	for i, id1 := range resolvedIDs {
		t, err := s.Load(id1)
		if err != nil {
			return count, err
		}

		for j, id2 := range resolvedIDs {
			if i == j {
				continue
			}
			if !t.HasLink(id2) {
				if err := s.AddLink(id1, id2); err != nil {
					return count, err
				}
				count++
			}
		}
	}

	return count, nil
}

