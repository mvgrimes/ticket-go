# Changelog

## [Unreleased]

### Changed
- Rewrote implementation in Go (`cmd/tk`) for improved performance and maintainability
- Migrated Python BDD tests to Go table-driven tests in `cmd/tk/main_test.go` (114 test cases)
- Editor fallback now searches for micro, nano, vi in order when `$EDITOR` is not set
- `--json` output for `list`, `ready`, and `closed` now returns a JSON array; `show --json` remains a single object

### Added
- Go test coverage for all commands including dependency cycles, partial ID resolution, and directory walking
- `--json` flag for `list`, `ready`, `closed`, and `show` commands with fields: `id`, `title`, `status`, `priority`, `issue_type`, `owner`, `created_at`, `created_by`, `updated_at`, `dependency_count`, `dependent_count`, `comment_count`

### Fixed
- `create` without a title now uses a temporary draft file in `$EDITOR` and only creates a ticket when the draft is modified
- `edit` now accepts an empty ID and creates a new ticket (matching `create` without a title)
- `edit` now validates edited tickets and rejects invalid frontmatter, headings, IDs, status/type, timestamps, priorities, and dependency/link IDs

## [0.3.1] - 2026-01-28

### Added
- `list` command alias for `ls`
- `TICKET_PAGER` environment variable for `show` command (only when stdout is a TTY; falls back to `PAGER`)

### Changed
- Walk parent directories to find `.tickets/` directory, enabling commands from any subdirectory
- Ticket ID suffix now uses full alphanumeric (a-z0-9) instead of hex for increased entropy

### Fixed
- `dep` command now resolves partial IDs for the dependency argument
- `undep` command now resolves partial IDs and validates dependency exists
- `unlink` command now resolves partial IDs for both arguments
- `create --parent` now validates and resolves parent ticket ID
- `generate_id` now uses 3-char prefix for single-segment directory names (e.g., "plan" → "pla" instead of "p")

## [0.3.0] - 2026-01-18

### Added
- Support `TICKETS_DIR` environment variable for custom tickets directory location
- `dep cycle` command to detect dependency cycles in open tickets
- `add-note` command for appending timestamped notes to tickets
- `-a, --assignee` filter flag for `ls`, `ready`, `blocked`, and `closed` commands
- `--tags` flag for `create` command to add comma-separated tags
- `-T, --tag` filter flag for `ls`, `ready`, `blocked`, and `closed` commands

## [0.2.0] - 2026-01-04

### Added
- `--parent` flag for `create` command to set parent ticket
- `link`/`unlink` commands for symmetric ticket relationships
- `show` command displays parent title and linked tickets
- `migrate-beads` now imports parent-child and related dependencies

## [0.1.1] - 2026-01-02

### Fixed
- `edit` command no longer hangs when run in non-TTY environments

## [0.1.0] - 2026-01-02

Initial release.
