package ticket

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// setupBenchmarkStore creates a temporary store with N tickets for benchmarking.
// Each ticket has some deps and a body to simulate realistic file sizes.
func setupBenchmarkStore(b *testing.B, numTickets int) *Store {
	b.Helper()

	dir := b.TempDir()
	ticketsDir := filepath.Join(dir, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		b.Fatal(err)
	}

	store := NewStore(ticketsDir)

	// Create tickets with varying content
	for i := 0; i < numTickets; i++ {
		id := fmt.Sprintf("bench-%04d", i)
		status := "open"
		if i%5 == 0 {
			status = "closed"
		} else if i%3 == 0 {
			status = "in_progress"
		}

		// Add some deps (each ticket depends on 0-3 previous tickets)
		var deps []string
		if i > 0 && i%2 == 0 {
			deps = append(deps, fmt.Sprintf("bench-%04d", i-1))
		}
		if i > 5 && i%3 == 0 {
			deps = append(deps, fmt.Sprintf("bench-%04d", i-5))
		}

		// Create ticket content with realistic body size
		content := fmt.Sprintf(`---
id: %s
status: %s
deps: [%s]
links: []
created: 2024-01-01T00:00:00Z
type: task
priority: 2
assignee: test-user
---
# Ticket %d Title

This is the description for ticket %d. It contains some text that represents
a typical ticket description with multiple lines and paragraphs.

## Details

Here are some additional details about this ticket:
- Point one with some explanation
- Point two with more details
- Point three describing requirements

## Notes

Some notes about implementation considerations and other relevant information
that might be included in a real ticket.

More content here to simulate realistic file sizes that would be encountered
in actual usage of the ticket system.
`, id, status, formatDepsForBench(deps), i, i)

		path := filepath.Join(ticketsDir, id+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}

	return store
}

func formatDepsForBench(deps []string) string {
	if len(deps) == 0 {
		return ""
	}
	result := deps[0]
	for _, d := range deps[1:] {
		result += ", " + d
	}
	return result
}

// BenchmarkListTickets benchmarks loading all tickets.
func BenchmarkListTickets(b *testing.B) {
	for _, numTickets := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("tickets=%d", numTickets), func(b *testing.B) {
			store := setupBenchmarkStore(b, numTickets)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				tickets, err := store.ListTickets()
				if err != nil {
					b.Fatal(err)
				}
				if len(tickets) != numTickets {
					b.Fatalf("expected %d tickets, got %d", numTickets, len(tickets))
				}
			}
		})
	}
}

// BenchmarkReadyTickets benchmarks finding ready tickets.
func BenchmarkReadyTickets(b *testing.B) {
	for _, numTickets := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("tickets=%d", numTickets), func(b *testing.B) {
			store := setupBenchmarkStore(b, numTickets)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := store.ReadyTickets(ListOptions{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBlockedTickets benchmarks finding blocked tickets.
func BenchmarkBlockedTickets(b *testing.B) {
	for _, numTickets := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("tickets=%d", numTickets), func(b *testing.B) {
			store := setupBenchmarkStore(b, numTickets)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _, err := store.BlockedTickets(ListOptions{})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkFindCycles benchmarks cycle detection.
func BenchmarkFindCycles(b *testing.B) {
	for _, numTickets := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("tickets=%d", numTickets), func(b *testing.B) {
			store := setupBenchmarkStore(b, numTickets)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := store.FindCycles()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGetDepTree benchmarks dependency tree generation.
func BenchmarkGetDepTree(b *testing.B) {
	for _, numTickets := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("tickets=%d", numTickets), func(b *testing.B) {
			store := setupBenchmarkStore(b, numTickets)
			// Use a ticket in the middle that has deps
			targetID := fmt.Sprintf("bench-%04d", numTickets/2)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := store.GetDepTree(targetID, false)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoadTicket benchmarks loading a single ticket (full parse).
func BenchmarkLoadTicket(b *testing.B) {
	store := setupBenchmarkStore(b, 1)
	path := filepath.Join(store.Dir, "bench-0000.md")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := LoadTicket(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadTicketHeader benchmarks loading only ticket header (frontmatter + title).
func BenchmarkLoadTicketHeader(b *testing.B) {
	store := setupBenchmarkStore(b, 1)
	path := filepath.Join(store.Dir, "bench-0000.md")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := LoadTicketHeader(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

