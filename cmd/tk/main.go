package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/kardianos/ticket"
	"golang.org/x/term"
)

var version = "dev"

// Global writers for output - can be overridden in tests
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

// run executes the CLI with the given arguments.
// This is the main entry point, extracted for testability.
func run(args []string) error {
	if len(args) < 1 {
		printHelp()
		return nil
	}

	cmd := args[0]

	// Help doesn't need tickets dir
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		printHelp()
		return nil
	}

	// Initialize tickets directory
	writeCommands := map[string]bool{"create": true}

	var store *ticket.Store

	if writeCommands[cmd] {
		// Write commands can create .tickets if not found
		dir, err := ticket.FindTicketsDir()
		if err != nil {
			dir = ".tickets"
		}
		store = ticket.NewStore(dir)
	} else {
		// Read commands need existing .tickets
		dir, err := ticket.FindTicketsDir()
		if err != nil {
			return fmt.Errorf("Error: %v\nRun 'tk create' to initialize, or set TICKETS_DIR env var", err)
		}
		store = ticket.NewStore(dir)
	}

	cmdArgs := args[1:]

	switch cmd {
	case "create":
		return cmdCreate(store, cmdArgs)
	case "start":
		return cmdStart(store, cmdArgs)
	case "close":
		return cmdClose(store, cmdArgs)
	case "reopen":
		return cmdReopen(store, cmdArgs)
	case "status":
		return cmdStatus(store, cmdArgs)
	case "dep":
		return cmdDep(store, cmdArgs)
	case "undep":
		return cmdUndep(store, cmdArgs)
	case "link":
		return cmdLink(store, cmdArgs)
	case "unlink":
		return cmdUnlink(store, cmdArgs)
	case "ls", "list":
		return cmdList(store, cmdArgs)
	case "ready":
		return cmdReady(store, cmdArgs)
	case "blocked":
		return cmdBlocked(store, cmdArgs)
	case "closed":
		return cmdClosed(store, cmdArgs)
	case "show":
		return cmdShow(store, cmdArgs)
	case "edit":
		return cmdEdit(store, cmdArgs)
	case "add-note":
		return cmdAddNote(store, cmdArgs)
	case "query":
		return cmdQuery(store, cmdArgs)
	default:
		printHelp()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func cmdCreate(store *ticket.Store, args []string) error {
	opts := ticket.CreateOptions{}

	// Parse args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-d" || arg == "--description":
			if i+1 < len(args) {
				opts.Description = args[i+1]
				i++
			}
		case arg == "--design":
			if i+1 < len(args) {
				opts.Design = args[i+1]
				i++
			}
		case arg == "--acceptance":
			if i+1 < len(args) {
				opts.Acceptance = args[i+1]
				i++
			}
		case arg == "-p" || arg == "--priority":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &opts.Priority)
				opts.PrioritySet = true
				i++
			}
		case arg == "-t" || arg == "--type":
			if i+1 < len(args) {
				opts.Type = args[i+1]
				i++
			}
		case arg == "-a" || arg == "--assignee":
			if i+1 < len(args) {
				opts.Assignee = args[i+1]
				i++
			}
		case arg == "--external-ref":
			if i+1 < len(args) {
				opts.ExternalRef = args[i+1]
				i++
			}
		case arg == "--parent":
			if i+1 < len(args) {
				opts.Parent = args[i+1]
				i++
			}
		case arg == "--tags":
			if i+1 < len(args) {
				opts.Tags = strings.Split(args[i+1], ",")
				i++
			}
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if opts.Title == "" {
				opts.Title = arg
			}
		}
	}

	t, err := store.Create(opts)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, t.ID)
	return nil
}

func cmdStart(store *ticket.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tk start <id>")
	}
	return setStatus(store, args[0], "in_progress")
}

func cmdClose(store *ticket.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tk close <id>")
	}
	return setStatus(store, args[0], "closed")
}

func cmdReopen(store *ticket.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tk reopen <id>")
	}
	return setStatus(store, args[0], "open")
}

func cmdStatus(store *ticket.Store, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tk status <id> <status>\nValid statuses: open in_progress closed")
	}
	return setStatus(store, args[0], args[1])
}

func setStatus(store *ticket.Store, partial, status string) error {
	if !ticket.IsValidStatus(status) {
		return fmt.Errorf("Error: invalid status '%s'. Must be one of: open in_progress closed", status)
	}

	id, err := store.ResolveID(partial)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	if err := store.UpdateField(id, "status", status); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Updated %s -> %s\n", id, status)
	return nil
}

func cmdDep(store *ticket.Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tk dep <id> <dependency-id>\n       tk dep tree <id>\n       tk dep cycle")
	}

	// Handle subcommands
	if args[0] == "tree" {
		return cmdDepTree(store, args[1:])
	}
	if args[0] == "cycle" {
		return cmdDepCycle(store)
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: tk dep <id> <dependency-id>")
	}

	id, err := store.ResolveID(args[0])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	depID, err := store.ResolveID(args[1])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	// Check if already exists
	t, err := store.Load(id)
	if err != nil {
		return err
	}

	if t.HasDep(depID) {
		fmt.Fprintln(stdout, "Dependency already exists")
		return nil
	}

	if err := store.AddDep(id, depID); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Added dependency: %s -> %s\n", id, depID)
	return nil
}

func cmdDepTree(store *ticket.Store, args []string) error {
	fullMode := false
	var rootID string

	for _, arg := range args {
		if arg == "--full" {
			fullMode = true
		} else {
			rootID = arg
		}
	}

	if rootID == "" {
		return fmt.Errorf("usage: tk dep tree [--full] <id>")
	}

	id, err := store.ResolveID(rootID)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	tree, err := store.GetDepTree(id, fullMode)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	output := ticket.PrintDepTree(tree, "", true, true)
	fmt.Fprint(stdout, output)
	return nil
}

func cmdDepCycle(store *ticket.Store) error {
	cycles, err := store.FindCycles()
	if err != nil {
		return err
	}

	if len(cycles) == 0 {
		fmt.Fprintln(stdout, "No dependency cycles found")
		return nil
	}

	for i, cycle := range cycles {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "Cycle %d: %s\n", i+1, strings.Join(cycle.Path, " -> "))
		for _, t := range cycle.Tickets {
			fmt.Fprintf(stdout, "  %-8s [%s] %s\n", t.ID, t.Status, t.Title)
		}
	}

	return nil
}

func cmdUndep(store *ticket.Store, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tk undep <id> <dependency-id>")
	}

	id, err := store.ResolveID(args[0])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	depID, err := store.ResolveID(args[1])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	if err := store.RemoveDep(id, depID); err != nil {
		if err.Error() == "dependency not found" {
			fmt.Fprintln(stdout, "Dependency not found")
			return fmt.Errorf("")
		}
		return err
	}

	fmt.Fprintf(stdout, "Removed dependency: %s -/-> %s\n", id, depID)
	return nil
}

func cmdLink(store *ticket.Store, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tk link <id> <id> [id...]")
	}

	// Resolve all IDs first to check they exist
	ids := make([]string, len(args))
	for i, arg := range args {
		id, err := store.ResolveID(arg)
		if err != nil {
			return fmt.Errorf("Error: %v", err)
		}
		ids[i] = id
	}

	count, err := store.LinkTickets(ids)
	if err != nil {
		return err
	}

	if count == 0 {
		fmt.Fprintln(stdout, "All links already exist")
	} else {
		fmt.Fprintf(stdout, "Added %d link(s) between %d tickets\n", count, len(ids))
	}

	return nil
}

func cmdUnlink(store *ticket.Store, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tk unlink <id> <target-id>")
	}

	id, err := store.ResolveID(args[0])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	targetID, err := store.ResolveID(args[1])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	// Check if link exists
	t, err := store.Load(id)
	if err != nil {
		return err
	}

	if !t.HasLink(targetID) {
		fmt.Fprintln(stdout, "Link not found")
		return fmt.Errorf("")
	}

	// Remove from both
	if err := store.RemoveLink(id, targetID); err != nil {
		return err
	}
	if err := store.RemoveLink(targetID, id); err != nil {
		// Ignore if not found in target
	}

	fmt.Fprintf(stdout, "Removed link: %s <-> %s\n", id, targetID)
	return nil
}

func parseListOpts(args []string) ticket.ListOptions {
	opts := ticket.ListOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--status="):
			opts.Status = strings.TrimPrefix(arg, "--status=")
		case arg == "-a":
			if i+1 < len(args) {
				opts.Assignee = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--assignee="):
			opts.Assignee = strings.TrimPrefix(arg, "--assignee=")
		case arg == "-T":
			if i+1 < len(args) {
				opts.Tag = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--tag="):
			opts.Tag = strings.TrimPrefix(arg, "--tag=")
		}
	}

	return opts
}

func cmdList(store *ticket.Store, args []string) error {
	opts := parseListOpts(args)

	tickets, err := store.ListTicketsFiltered(opts)
	if err != nil {
		return err
	}

	for _, t := range tickets {
		depsDisplay := "[]"
		if len(t.Deps) > 0 {
			depsDisplay = "[" + strings.Join(t.Deps, ", ") + "]"
		}

		depStr := ""
		if depsDisplay != "[]" {
			depStr = " <- " + depsDisplay
		}

		fmt.Fprintf(stdout, "%-8s [%s] - %s%s\n", t.ID, t.Status, t.Title, depStr)
	}

	return nil
}

func cmdReady(store *ticket.Store, args []string) error {
	opts := parseListOpts(args)

	tickets, err := store.ReadyTickets(opts)
	if err != nil {
		return err
	}

	for _, t := range tickets {
		fmt.Fprintf(stdout, "%-8s [P%d][%s] - %s\n", t.ID, t.Priority, t.Status, t.Title)
	}

	return nil
}

func cmdBlocked(store *ticket.Store, args []string) error {
	opts := parseListOpts(args)

	tickets, blockers, err := store.BlockedTickets(opts)
	if err != nil {
		return err
	}

	for i, t := range tickets {
		blockerStr := "[" + strings.Join(blockers[i], ", ") + "]"
		fmt.Fprintf(stdout, "%-8s [P%d][%s] - %s <- %s\n", t.ID, t.Priority, t.Status, t.Title, blockerStr)
	}

	return nil
}

func cmdClosed(store *ticket.Store, args []string) error {
	opts := parseListOpts(args)
	limit := 20

	// Parse limit
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--limit=") {
			fmt.Sscanf(strings.TrimPrefix(args[i], "--limit="), "%d", &limit)
		}
	}

	tickets, err := store.ClosedTickets(opts, limit)
	if err != nil {
		return err
	}

	for _, t := range tickets {
		fmt.Fprintf(stdout, "%-8s [%s] - %s\n", t.ID, t.Status, t.Title)
	}

	return nil
}

func cmdShow(store *ticket.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tk show <id>")
	}

	id, err := store.ResolveID(args[0])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	info, err := store.GetShowInfo(id)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	output := ticket.FormatShowInfo(info)

	// Check for pager
	pager := os.Getenv("TICKET_PAGER")
	if pager == "" {
		pager = os.Getenv("PAGER")
	}

	// Only use pager if stdout is a terminal
	if pager != "" && isTerminal(os.Stdout) {
		return runWithPager(output, pager)
	}

	fmt.Fprint(stdout, output)
	return nil
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func runWithPager(content, pager string) error {
	cmd := exec.Command("sh", "-c", pager)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprint(stdout, content)
		return nil
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprint(stdout, content)
		return nil
	}

	io.WriteString(stdin, content)
	stdin.Close()

	return cmd.Wait()
}

func cmdEdit(store *ticket.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tk edit <id>")
	}

	path, err := store.GetTicketPath(args[0])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	// Check if running in TTY
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			for _, e := range []string{"micro", "nano", "vi"} {
				if _, err := exec.LookPath(e); err == nil {
					editor = e
					break
				}
			}
		}
		if editor == "" {
			return fmt.Errorf("no editor found (set EDITOR or install micro, nano, or vi)")
		}

		cmd := exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	fmt.Fprintf(stdout, "Edit ticket file: %s\n", path)
	return nil
}

func cmdAddNote(store *ticket.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tk add-note <id> [note text]")
	}

	id, err := store.ResolveID(args[0])
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	var note string
	if len(args) > 1 {
		note = strings.Join(args[1:], " ")
	} else if !isTerminal(os.Stdin) {
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		note = strings.TrimSpace(string(data))
	} else {
		return fmt.Errorf("Error: no note provided")
	}

	if err := store.AddNote(id, note); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Note added to %s\n", id)
	return nil
}

func cmdQuery(store *ticket.Store, args []string) error {
	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}

	tickets, err := store.QueryTicketsFiltered(filter)
	if err != nil {
		return err
	}

	for _, t := range tickets {
		data, err := json.Marshal(t)
		if err != nil {
			continue
		}
		fmt.Fprintln(stdout, string(data))
	}

	return nil
}

func printHelp() {
	fmt.Fprint(stdout, `tk - minimal ticket system with dependency tracking

Usage: tk <command> [args]

Commands:
  create [title] [options] Create ticket, prints ID
    -d, --description      Description text
    --design               Design notes
    --acceptance           Acceptance criteria
    -t, --type             Type (bug|feature|task|epic|chore) [default: task]
    -p, --priority         Priority 0-4, 0=highest [default: 2]
    -a, --assignee         Assignee
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
  ls|list [--status=X] [-a X] [-T X]   List tickets
  ready [-a X] [-T X]      List open/in-progress tickets with deps resolved
  blocked [-a X] [-T X]    List open/in-progress tickets with unresolved deps
  closed [--limit=N] [-a X] [-T X] List recently closed tickets (default 20, by mtime)
  show <id>                Display ticket
  edit <id>                Open ticket in $EDITOR
  add-note <id> [text]     Append timestamped note (or pipe via stdin)
  query [filter]           Output tickets as JSON (filter: status=open, priority<2, or jq expr)

Tickets stored as markdown files in .tickets/
Supports partial ID matching (e.g., 'tk show 5c4' matches 'nw-5c46')
`)
}
