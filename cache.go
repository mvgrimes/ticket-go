package ticket

import (
	"os"
	"strconv"
	"sync"
)

// Cache provides cached access to ticket read/write operations.
// It tracks whether a ticket was loaded as header-only or with full body.
type Cache struct {
	mu      sync.RWMutex
	tickets map[string]*cachedTicket
	dirs    map[string][]os.DirEntry // cached directory listings
}

type cachedTicket struct {
	ticket   *Ticket
	fullBody bool // true if entire body was read
}

// NewCache creates a new ticket cache.
func NewCache() *Cache {
	return &Cache{
		tickets: make(map[string]*cachedTicket),
		dirs:    make(map[string][]os.DirEntry),
	}
}

// ReadDir returns cached directory entries, or reads and caches them.
func (c *Cache) ReadDir(dir string) ([]os.DirEntry, error) {
	c.mu.RLock()
	if entries, ok := c.dirs[dir]; ok {
		c.mu.RUnlock()
		return entries, nil
	}
	c.mu.RUnlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.dirs[dir] = entries
	c.mu.Unlock()

	return entries, nil
}

// InvalidateDir clears the cached directory listing.
func (c *Cache) InvalidateDir(dir string) {
	c.mu.Lock()
	delete(c.dirs, dir)
	c.mu.Unlock()
}

// LoadTicket loads a full ticket, using cache if available with full body.
func (c *Cache) LoadTicket(path string) (*Ticket, error) {
	c.mu.RLock()
	if cached, ok := c.tickets[path]; ok && cached.fullBody {
		c.mu.RUnlock()
		return cached.ticket, nil
	}
	c.mu.RUnlock()

	ticket, err := LoadTicket(path)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.tickets[path] = &cachedTicket{ticket: ticket, fullBody: true}
	c.mu.Unlock()

	return ticket, nil
}

// LoadTicketHeader loads just the header, using cache if available.
func (c *Cache) LoadTicketHeader(path string) (*Ticket, error) {
	c.mu.RLock()
	if cached, ok := c.tickets[path]; ok {
		c.mu.RUnlock()
		return cached.ticket, nil
	}
	c.mu.RUnlock()

	ticket, err := LoadTicketHeader(path)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	// Don't overwrite a full body cache with header-only
	if existing, ok := c.tickets[path]; !ok || !existing.fullBody {
		c.tickets[path] = &cachedTicket{ticket: ticket, fullBody: false}
	}
	c.mu.Unlock()

	return ticket, nil
}

// SaveTicket saves a ticket and updates the cache.
// Invalidates directory cache if this is a new file.
func (c *Cache) SaveTicket(ticket *Ticket, path string, dir string) error {
	// Check if file exists before save
	_, err := os.Stat(path)
	isNew := os.IsNotExist(err)

	if err := SaveTicket(ticket, path); err != nil {
		return err
	}

	c.mu.Lock()
	c.tickets[path] = &cachedTicket{ticket: ticket, fullBody: true}
	if isNew {
		delete(c.dirs, dir)
	}
	c.mu.Unlock()

	return nil
}

// UpdateYAMLField updates a field and updates the cache in-place.
func (c *Cache) UpdateYAMLField(path, field, value string) error {
	if err := UpdateYAMLField(path, field, value); err != nil {
		return err
	}

	c.mu.Lock()
	if cached, ok := c.tickets[path]; ok {
		updateTicketField(cached.ticket, field, value)
	}
	c.mu.Unlock()

	return nil
}

// GetYAMLField gets a field value, loading header into cache if needed.
func (c *Cache) GetYAMLField(path, field string) (string, error) {
	t, err := c.LoadTicketHeader(path)
	if err != nil {
		return "", err
	}
	return getTicketField(t, field), nil
}

// AppendBody appends content to a cached ticket's body if it exists with fullBody.
func (c *Cache) AppendBody(path, content string) {
	c.mu.Lock()
	if cached, ok := c.tickets[path]; ok && cached.fullBody {
		cached.ticket.Body += content
	}
	c.mu.Unlock()
}

// updateTicketField updates a specific field in a ticket.
func updateTicketField(t *Ticket, field, value string) {
	switch field {
	case "id":
		t.ID = value
	case "status":
		t.Status = lookupStatus([]byte(value))
	case "type":
		t.Type = value
	case "priority":
		t.Priority, _ = strconv.Atoi(value)
	case "assignee":
		t.Assignee = value
	case "external-ref":
		t.ExternalRef = value
	case "parent":
		t.Parent = value
	case "deps":
		t.Deps = parseArray(value)
	case "links":
		t.Links = parseArray(value)
	case "tags":
		t.Tags = parseArray(value)
	}
}

// getTicketField retrieves a specific field from a ticket.
func getTicketField(t *Ticket, field string) string {
	switch field {
	case "id":
		return t.ID
	case "status":
		return string(t.Status)
	case "type":
		return t.Type
	case "priority":
		return strconv.Itoa(t.Priority)
	case "assignee":
		return t.Assignee
	case "external-ref":
		return t.ExternalRef
	case "parent":
		return t.Parent
	case "deps":
		return formatArray(t.Deps)
	case "links":
		return formatArray(t.Links)
	case "tags":
		return formatArray(t.Tags)
	default:
		return ""
	}
}
