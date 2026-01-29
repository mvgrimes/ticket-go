package ticket

import (
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
)

// QueryTicketsFiltered returns tickets matching the jq filter expression.
// If filter is empty, returns all tickets.
// Supports simple syntax (status=open, priority<2) or full jq expressions.
func (s *Store) QueryTicketsFiltered(filter string) ([]TicketJSON, error) {
	tickets, err := s.ListTickets()
	if err != nil {
		return nil, err
	}

	// Convert to JSON format
	result := make([]TicketJSON, 0, len(tickets))
	for _, t := range tickets {
		result = append(result, t.ToJSON())
	}

	if filter == "" {
		return result, nil
	}

	// Build jq expression: .[] | select(...)
	jqExpr := ConvertToJQExpr(filter)

	query, err := gojq.Parse(jqExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid query: %v", err)
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile query: %v", err)
	}

	// Convert result slice to []any for gojq
	input := make([]any, len(result))
	for i, t := range result {
		input[i] = ticketJSONToMap(t)
	}

	// Run query on the array
	var filtered []TicketJSON
	iter := code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			// Skip errors (e.g., select returned false)
			_ = err
			continue
		}
		if m, ok := v.(map[string]any); ok {
			if t, err := mapToTicketJSON(m); err == nil {
				filtered = append(filtered, t)
			}
		}
	}

	return filtered, nil
}

// ticketJSONToMap converts TicketJSON to map[string]any for gojq processing.
func ticketJSONToMap(t TicketJSON) map[string]any {
	m := map[string]any{
		"id":       t.ID,
		"status":   t.Status,
		"deps":     toAnySlice(t.Deps),
		"links":    toAnySlice(t.Links),
		"created":  t.Created,
		"type":     t.Type,
		"priority": t.Priority,
	}
	if t.Assignee != "" {
		m["assignee"] = t.Assignee
	}
	if t.ExternalRef != "" {
		m["external-ref"] = t.ExternalRef
	}
	if t.Parent != "" {
		m["parent"] = t.Parent
	}
	if len(t.Tags) > 0 {
		m["tags"] = toAnySlice(t.Tags)
	}
	return m
}

func toAnySlice(s []string) []any {
	if s == nil {
		return []any{}
	}
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

// mapToTicketJSON converts a map back to TicketJSON.
func mapToTicketJSON(m map[string]any) (TicketJSON, error) {
	t := TicketJSON{}

	if v, ok := m["id"].(string); ok {
		t.ID = v
	}
	if v, ok := m["status"].(string); ok {
		t.Status = v
	}
	if v, ok := m["deps"].([]any); ok {
		t.Deps = toStringSlice(v)
	}
	if v, ok := m["links"].([]any); ok {
		t.Links = toStringSlice(v)
	}
	if v, ok := m["created"].(string); ok {
		t.Created = v
	}
	if v, ok := m["type"].(string); ok {
		t.Type = v
	}
	if v, ok := m["priority"].(int); ok {
		t.Priority = v
	}
	if v, ok := m["assignee"].(string); ok {
		t.Assignee = v
	}
	if v, ok := m["external-ref"].(string); ok {
		t.ExternalRef = v
	}
	if v, ok := m["parent"].(string); ok {
		t.Parent = v
	}
	if v, ok := m["tags"].([]any); ok {
		t.Tags = toStringSlice(v)
	}

	return t, nil
}

func toStringSlice(a []any) []string {
	if a == nil {
		return nil
	}
	result := make([]string, 0, len(a))
	for _, v := range a {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ConvertToJQExpr converts simple filter syntax to jq expressions.
// Examples:
//   - "status=open"         -> ".[] | select(.status == \"open\")"
//   - "priority<2"          -> ".[] | select(.priority < 2)"
//   - ".status"             -> ".[] | .status"
//   - ".status == \"open\"" -> ".[] | select(.status == \"open\")"
//   - "select(...)"         -> ".[] | select(...)"
func ConvertToJQExpr(filter string) string {
	inner := convertFilterToSelect(filter)
	return ".[] | " + inner
}

// convertFilterToSelect converts a filter expression to the inner part after ".[] |"
func convertFilterToSelect(filter string) string {
	// Already wrapped in select, pass through
	if strings.HasPrefix(filter, "select(") {
		return filter
	}

	// If it starts with "." and contains a comparison, wrap in select
	if strings.HasPrefix(filter, ".") {
		for _, op := range []string{"!=", "<=", ">=", "==", "<", ">"} {
			if strings.Contains(filter, op) {
				return fmt.Sprintf("select(%s)", filter)
			}
		}
		// Simple field access like ".status", pass through
		return filter
	}

	// Check for simple field=value syntax
	for _, op := range []string{"!=", "<=", ">=", "==", "=", "<", ">"} {
		if idx := strings.Index(filter, op); idx > 0 {
			field := strings.TrimSpace(filter[:idx])
			value := strings.TrimSpace(filter[idx+len(op):])

			// Normalize = to ==
			jqOp := op
			if op == "=" {
				jqOp = "=="
			}

			// Quote string values, leave numbers as-is
			if _, err := fmt.Sscanf(value, "%f", new(float64)); err != nil {
				// Not a number, quote it
				value = fmt.Sprintf("%q", value)
			}

			return fmt.Sprintf("select(.%s %s %s)", field, jqOp, value)
		}
	}

	// Default: wrap in select
	return fmt.Sprintf("select(%s)", filter)
}
