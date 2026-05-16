# ticket

A git-backed issue tracker for AI agents. Rooted in the Unix Philosophy, `tk` is inspired by Joe Armstrong's [Minimal Viable Program](https://joearms.github.io/published/2014-06-25-minimal-viable-program.html) with additional quality of life features for managing and querying against complex issue dependency graphs.

This is a Go port of the original [bash implementation](https://github.com/wedow/ticket).

Tickets are markdown files with YAML frontmatter in `.tickets/`. This allows AI agents to easily search them for relevant content without dumping ten thousand character JSONL lines into their context window.

Using ticket IDs as file names also allows IDEs to quickly navigate to the ticket for you. For example, you might run `git log` in your terminal and see something like:

```
nw-5c46: add SSE connection management
```

VS Code allows you to Ctrl+Click or Cmd+Click the ID and jump directly to the file to read the details.

## Install

**From source:**
```bash
go install github.com/kardianos/ticket/cmd/tk@latest
```

## Requirements

Go 1.24 or later.

## Agent Setup

Add this line to your `CLAUDE.md` or `AGENTS.md`:

```
This project uses a CLI ticket system for task management. Run `tk help` when you need to use it.
```

Claude Opus picks it up naturally from there. Other models may need additional guidance.

## Usage

```bash
tk - minimal ticket system with dependency tracking

Usage: tk <command> [args]

Commands:
  create [title] [options] Create ticket (no title opens a temp draft in $EDITOR when interactive)
    -d, --description      Description text
    --design               Design notes
    --acceptance           Acceptance criteria
    -t, --type             Type (bug|feature|task|epic|chore) [default: task]
    -p, --priority         Priority 0-4, 0=highest [default: 2]
    -a, --assignee         Assignee [default: git user.name]
    --external-ref         External reference (e.g., gh-123, JIRA-456)
    --parent               Parent ticket ID
    --tags                 Comma-separated tags (e.g., --tags ui,backend,urgent)
  start <id>               Set status to in_progress
  close <id>               Set status to closed
  reopen <id>              Set status to open
  status <id> <status>     Update status (open|in_progress|closed)
  dep <id> <dep-id>        Add dependency (id depends on dep-id)
  dep tree [--full] <id>   Show dependency tree (--full disables dedup)
  dep cycle                Find dependency cycles in open tickets
  undep <id> <dep-id>      Remove dependency
  link <id> <id> [id...]   Link tickets together (symmetric)
  unlink <id> <target-id>  Remove link between tickets
  ls|list [--status=X] [-a X] [-T X] [--json]   List tickets
  ready [-a X] [-T X] [--json]      List open/in-progress tickets with deps resolved
  blocked [-a X] [-T X]    List open/in-progress tickets with unresolved deps
  closed [--limit=N] [-a X] [-T X] [--json] List recently closed tickets (default 20, by mtime)
  show <id> [--json]       Display ticket
  edit [id]                Open ticket in $EDITOR (or create a new ticket)
  add-note <id> [text]     Append timestamped note (or pipe via stdin)
  query [filter]           Output tickets as JSON (filter: status=open, priority<2, or jq expr)

Searches parent directories for .tickets/ (override with TICKETS_DIR env var)
Supports partial ID matching (e.g., 'tk show 5c4' matches 'nw-5c46')
```

When `create` is run without a title in an interactive terminal, `tk` writes a draft to a temporary file and opens it in `$EDITOR`. If you save changes, the draft is moved into `.tickets/` and the new ticket ID is printed. If unchanged, the draft is discarded. In non-interactive mode, `create` without a title returns an error.

When `edit` is run without an ID, it behaves like `create` without a title and creates a new ticket.

After `edit` returns, `tk` validates the ticket structure (frontmatter, title heading, IDs, status/type, timestamp, and priority) before accepting the result.

## JSON Output

The `list`, `ready`, `closed`, and `show` commands accept a `--json` flag.

- `list`, `ready`, and `closed` output a JSON array of ticket objects.
- `show` outputs a single ticket object.

```bash
tk list --json
tk ready --json
tk closed --json
tk show <id> --json
```

Example `list --json` output:

```json
[
  {
    "id": "mytui-hdj",
    "title": "add login page",
    "status": "open",
    "priority": 2,
    "issue_type": "task",
    "owner": "Mark Grimes",
    "created_at": "2026-05-02T13:00:18Z",
    "created_by": "Mark Grimes",
    "updated_at": "2026-05-02T13:00:18Z",
    "dependency_count": 0,
    "dependent_count": 0,
    "comment_count": 0
  }
]
```

Fields:

| Field | Description |
|-------|-------------|
| `id` | Ticket ID |
| `title` | Ticket title |
| `status` | `open`, `in_progress`, or `closed` |
| `priority` | 0–4, where 0 is highest |
| `issue_type` | `bug`, `feature`, `task`, `epic`, or `chore` |
| `owner` | Assignee |
| `created_at` | Creation timestamp (ISO 8601) |
| `created_by` | Creator (same as assignee at creation time) |
| `updated_at` | Last file modification time (ISO 8601) |
| `dependency_count` | Number of tickets this ticket depends on |
| `dependent_count` | Number of tickets that depend on this ticket |
| `comment_count` | Number of notes added via `add-note` |

The `--json` flag composes with all existing filter flags:

```bash
tk list --status=open --json
tk ready -a "Jane" --json
tk closed --limit=5 --json
```

## Testing

The primary implementation and test suite are in Go:

```bash
go test ./...
```

This runs 114 test cases in `cmd/tk/main_test.go` covering all commands.

### Legacy Tests

The repository also contains:

- **`ticket`** - Original bash implementation (~900 lines)
- **`features/`** - Python BDD tests using [behave](https://behave.readthedocs.io/) (112 scenarios)

## License

MIT
