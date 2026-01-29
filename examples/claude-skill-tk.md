# tk - Ticket Management

Use this skill when the user asks you to work with tickets, tasks, issues, or todos in this project.

## Quick Reference

```bash
# List tickets
tk ls                    # All open tickets
tk ready                 # Open tickets with deps resolved (workable now)
tk blocked               # Open tickets waiting on deps
tk closed --limit=10     # Recently closed tickets

# Create and manage
tk create "Title" -d "Description"
tk start <id>            # Set to in_progress
tk close <id>            # Mark done
tk show <id>             # View details

# Dependencies
tk dep <id> <dep-id>     # id depends on dep-id
tk undep <id> <dep-id>   # Remove dependency
tk dep tree <id>         # Show dependency tree
tk dep cycle             # Find circular dependencies

# Notes and links
tk add-note <id> "note"  # Add timestamped note
tk link <id1> <id2>      # Link related tickets
```

## Workflow Guidelines

### Before Starting Work
1. Run `tk ready` to see actionable tickets
2. Pick a ticket and run `tk start <id>`
3. Read full details with `tk show <id>`

### While Working
- Add notes as you progress: `tk add-note <id> "Discovered X needs Y"`
- If blocked, create dependency: `tk dep <current> <blocker-id>`
- Link related tickets: `tk link <id1> <id2>`

### After Completing Work
1. Run `tk close <id>`
2. Check if this unblocks other tickets: `tk ready`

### Creating New Tickets
```bash
# Basic task
tk create "Implement user auth"

# With details
tk create "Fix login bug" \
  -t bug \
  -p 1 \
  -d "Users getting 500 error on login" \
  --tags security,urgent

# As subtask of epic
tk create "Add OAuth provider" --parent <epic-id>
```

## Partial ID Matching

You don't need full IDs. These work:
- `tk show 5c4` matches `nw-5c46`
- `tk start abc` matches `proj-abc123`

## Filtering

```bash
tk ls --status=in_progress    # By status
tk ls -a john                 # By assignee
tk ls -T backend              # By tag
tk ready -T urgent            # Combine filters
```

## When to Use

- User says "what should I work on" -> `tk ready`
- User says "create a ticket for X" -> `tk create "X"`
- User says "mark that done" -> `tk close <id>`
- User asks about dependencies -> `tk dep tree <id>` or `tk blocked`
- Starting a task -> `tk start <id>` then `tk show <id>`
