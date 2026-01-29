package ticket

import (
	"strings"
	"testing"
)

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{"open", true},
		{"in_progress", true},
		{"closed", true},
		{"invalid", false},
		{"", false},
		{"OPEN", false}, // case sensitive
		{"Open", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := IsValidStatus(tt.status)
			if got != tt.valid {
				t.Errorf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name        string
		dirPath     string
		wantPrefix  string
		description string
	}{
		{
			name:        "hyphenated directory",
			dirPath:     "/home/user/my-project",
			wantPrefix:  "mp-",
			description: "should use first letter of each segment",
		},
		{
			name:        "underscored directory",
			dirPath:     "/home/user/my_project",
			wantPrefix:  "mp-",
			description: "should handle underscores like hyphens",
		},
		{
			name:        "single word directory",
			dirPath:     "/home/user/ticket",
			wantPrefix:  "tic-",
			description: "should use first 3 chars for single word",
		},
		{
			name:        "short directory name",
			dirPath:     "/home/user/ab",
			wantPrefix:  "ab-",
			description: "should handle short names",
		},
		{
			name:        "complex hyphenated",
			dirPath:     "/projects/my-cool-app",
			wantPrefix:  "mca-",
			description: "should extract from multiple segments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := GenerateID(tt.dirPath)
			if err != nil {
				t.Fatalf("GenerateID(%q) error = %v", tt.dirPath, err)
			}

			if !strings.HasPrefix(id, tt.wantPrefix) {
				t.Errorf("GenerateID(%q) = %q, want prefix %q", tt.dirPath, id, tt.wantPrefix)
			}

			// Check that suffix is 4 alphanumeric characters
			parts := strings.SplitN(id, "-", 2)
			if len(parts) != 2 {
				t.Errorf("GenerateID(%q) = %q, expected format prefix-suffix", tt.dirPath, id)
				return
			}

			suffix := parts[1]
			if len(suffix) != 4 {
				t.Errorf("GenerateID(%q) suffix = %q, want 4 chars", tt.dirPath, suffix)
			}

			for _, c := range suffix {
				if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
					t.Errorf("GenerateID(%q) suffix %q contains invalid char %c", tt.dirPath, suffix, c)
				}
			}
		})
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	// Generate multiple IDs and ensure they're unique
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := GenerateID("/test/project")
		if err != nil {
			t.Fatalf("GenerateID error: %v", err)
		}
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestISODate(t *testing.T) {
	date := ISODate()

	// Check format: 2006-01-02T15:04:05Z
	if len(date) != 20 {
		t.Errorf("ISODate() = %q, want length 20", date)
	}

	if !strings.HasSuffix(date, "Z") {
		t.Errorf("ISODate() = %q, should end with Z", date)
	}

	if date[4] != '-' || date[7] != '-' || date[10] != 'T' || date[13] != ':' || date[16] != ':' {
		t.Errorf("ISODate() = %q, wrong format", date)
	}
}

func TestTicket_HasDep(t *testing.T) {
	ticket := &Ticket{
		Deps: []string{"dep-001", "dep-002"},
	}

	tests := []struct {
		depID string
		want  bool
	}{
		{"dep-001", true},
		{"dep-002", true},
		{"dep-003", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.depID, func(t *testing.T) {
			got := ticket.HasDep(tt.depID)
			if got != tt.want {
				t.Errorf("HasDep(%q) = %v, want %v", tt.depID, got, tt.want)
			}
		})
	}
}

func TestTicket_HasLink(t *testing.T) {
	ticket := &Ticket{
		Links: []string{"link-001", "link-002"},
	}

	tests := []struct {
		linkID string
		want   bool
	}{
		{"link-001", true},
		{"link-002", true},
		{"link-003", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.linkID, func(t *testing.T) {
			got := ticket.HasLink(tt.linkID)
			if got != tt.want {
				t.Errorf("HasLink(%q) = %v, want %v", tt.linkID, got, tt.want)
			}
		})
	}
}

func TestTicket_HasDep_EmptyDeps(t *testing.T) {
	ticket := &Ticket{
		Deps: []string{},
	}

	if ticket.HasDep("any") {
		t.Error("HasDep should return false for empty deps")
	}
}

func TestTicket_HasLink_EmptyLinks(t *testing.T) {
	ticket := &Ticket{
		Links: []string{},
	}

	if ticket.HasLink("any") {
		t.Error("HasLink should return false for empty links")
	}
}
