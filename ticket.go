// Package ticket provides a minimal ticket system with dependency tracking.
// Tickets are stored as markdown files with YAML frontmatter in a .tickets directory.
package ticket

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status represents the status of a ticket.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// ValidStatuses contains all valid status values.
var ValidStatuses = []Status{StatusOpen, StatusInProgress, StatusClosed}

// IsValidStatus checks if a status string is valid.
func IsValidStatus(s string) bool {
	for _, valid := range ValidStatuses {
		if Status(s) == valid {
			return true
		}
	}
	return false
}

// Ticket represents a ticket with its metadata and content.
type Ticket struct {
	ID          string    `yaml:"id"`
	Status      Status    `yaml:"status"`
	Deps        []string  `yaml:"deps"`
	Links       []string  `yaml:"links"`
	Created     time.Time `yaml:"created"`
	Type        string    `yaml:"type"`
	Priority    int       `yaml:"priority"`
	Assignee    string    `yaml:"assignee,omitempty"`
	ExternalRef string    `yaml:"external-ref,omitempty"`
	Parent      string    `yaml:"parent,omitempty"`
	Tags        []string  `yaml:"tags,omitempty"`

	// Content fields (not in frontmatter)
	Title       string
	Description string
	Design      string
	Acceptance  string
	Body        string // Full body content after frontmatter
}

// CreateOptions contains options for creating a new ticket.
type CreateOptions struct {
	Title       string
	Description string
	Design      string
	Acceptance  string
	Type        string
	Priority    int
	PrioritySet bool // True if priority was explicitly set (to handle 0 as valid value)
	Assignee    string
	ExternalRef string
	Parent      string
	Tags        []string
}

// GenerateID generates a ticket ID from the directory name prefix + random string.
func GenerateID(dirPath string) (string, error) {
	dirName := filepath.Base(dirPath)

	// Extract first letter of each hyphenated/underscored segment
	var prefix strings.Builder
	segments := strings.FieldsFunc(dirName, func(r rune) bool {
		return r == '-' || r == '_'
	})

	for _, seg := range segments {
		if len(seg) > 0 {
			prefix.WriteByte(seg[0])
		}
	}

	// Fallback to first 3 chars if single segment (prefix too short)
	p := strings.ToLower(prefix.String())
	if len(p) < 2 {
		if len(dirName) >= 3 {
			p = strings.ToLower(dirName[:3])
		} else {
			p = strings.ToLower(dirName)
		}
	}

	// 4-char random lower case alphanumeric string
	hash, err := randomAlphanumeric(4)
	if err != nil {
		return "", fmt.Errorf("generating random suffix: %w", err)
	}

	return fmt.Sprintf("%s-%s", p, hash), nil
}

func randomAlphanumeric(n int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}

// ISODate returns the current time in ISO 8601 format.
func ISODate() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// DefaultAssignee returns the default assignee from git config.
func DefaultAssignee() string {
	// Try to get git user.name
	home, _ := os.UserHomeDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	content, err := os.ReadFile(gitconfig)
	if err != nil {
		return ""
	}

	// Simple parsing - look for name = value under [user]
	lines := strings.Split(string(content), "\n")
	inUser := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "[user]" {
			inUser = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inUser = false
			continue
		}
		if inUser && strings.HasPrefix(line, "name") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}
