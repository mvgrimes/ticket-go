package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ticket "github.com/kardianos/ticket"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version = "0.4.7"

// Global writers for output - can be overridden in tests
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// app holds shared state for all commands in a single invocation.
type app struct {
	store *ticket.Store
	out   io.Writer
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

// run is the testable entry point.
func run(args []string) error {
	root := buildRootCmd(stdout, stderr)
	root.SetArgs(args)
	return root.Execute()
}

// buildRootCmd constructs the complete cobra command tree.
func buildRootCmd(out, errOut io.Writer) *cobra.Command {
	a := &app{out: out}

	root := &cobra.Command{
		Use:           "tk",
		Short:         "tk - minimal ticket system with dependency tracking",
		Long:          helpText,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.SetErr(errOut)

	// Initialise the ticket store before every command except the built-in help.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}
		if cmd.Name() == "create" {
			// create is allowed to bootstrap a new .tickets dir
			dir, err := ticket.FindTicketsDir()
			if err != nil {
				dir = ".tickets"
			}
			a.store = ticket.NewStore(dir)
			return nil
		}
		dir, err := ticket.FindTicketsDir()
		if err != nil {
			return fmt.Errorf("Error: %v\nRun 'tk create' to initialize, or set TICKETS_DIR env var", err)
		}
		a.store = ticket.NewStore(dir)
		return nil
	}

	root.AddCommand(
		a.newCreateCmd(),
		a.newStartCmd(),
		a.newCloseCmd(),
		a.newReopenCmd(),
		a.newStatusCmd(),
		a.newDepCmd(),
		a.newUndepCmd(),
		a.newLinkCmd(),
		a.newUnlinkCmd(),
		a.newListCmd(),
		a.newReadyCmd(),
		a.newBlockedCmd(),
		a.newClosedCmd(),
		a.newShowCmd(),
		a.newEditCmd(),
		a.newAddNoteCmd(),
		a.newQueryCmd(),
	)

	return root
}

// ── create ──────────────────────────────────────────────────────────────────

func (a *app) newCreateCmd() *cobra.Command {
	var (
		description string
		design      string
		acceptance  string
		ticketType  string
		priority    int
		assignee    string
		externalRef string
		parent      string
		tagsStr     string
	)

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create ticket, prints ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := ticket.CreateOptions{
				Description: description,
				Design:      design,
				Acceptance:  acceptance,
				Type:        ticketType,
				Assignee:    assignee,
				ExternalRef: externalRef,
				Parent:      parent,
			}
			if len(args) > 0 {
				opts.Title = strings.Join(args, " ")
			}
			if cmd.Flags().Changed("priority") {
				opts.Priority = priority
				opts.PrioritySet = true
			}
			if tagsStr != "" {
				opts.Tags = strings.Split(tagsStr, ",")
			}

			if opts.Title == "" && (!isTerminal(os.Stdin) || !isTerminal(os.Stdout)) {
				return fmt.Errorf("title is required in non-interactive mode")
			}

			t, err := a.store.Create(opts)
			if err != nil {
				return err
			}
			if opts.Title == "" && isTerminal(os.Stdin) && isTerminal(os.Stdout) {
				path, err := a.store.GetTicketPath(t.ID)
				if err != nil {
					return fmt.Errorf("Error: %v", err)
				}
				if err := a.editAndValidateWithTitleRecovery(path); err != nil {
					return err
				}
			}
			fmt.Fprintln(a.out, t.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Description text")
	cmd.Flags().StringVar(&design, "design", "", "Design notes")
	cmd.Flags().StringVar(&acceptance, "acceptance", "", "Acceptance criteria")
	cmd.Flags().StringVarP(&ticketType, "type", "t", "", "Type (bug|feature|task|epic|chore)")
	cmd.Flags().IntVarP(&priority, "priority", "p", 2, "Priority 0-4, 0=highest")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	cmd.Flags().StringVar(&externalRef, "external-ref", "", "External reference (e.g., gh-123, JIRA-456)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent ticket ID")
	cmd.Flags().StringVar(&tagsStr, "tags", "", "Comma-separated tags (e.g., ui,backend,urgent)")

	return cmd
}

// ── status shortcuts ─────────────────────────────────────────────────────────

func (a *app) newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Set status to in_progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.setStatus(args[0], "in_progress")
		},
	}
}

func (a *app) newCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <id>",
		Short: "Set status to closed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.setStatus(args[0], "closed")
		},
	}
}

func (a *app) newReopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <id>",
		Short: "Set status to open",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.setStatus(args[0], "open")
		},
	}
}

func (a *app) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id> <status>",
		Short: "Update status (open|in_progress|closed)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.setStatus(args[0], args[1])
		},
	}
}

func (a *app) setStatus(partial, status string) error {
	if !ticket.IsValidStatus(status) {
		return fmt.Errorf("Error: invalid status '%s'. Must be one of: open in_progress closed", status)
	}
	id, err := a.store.ResolveID(partial)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	if err := a.store.UpdateField(id, "status", status); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "Updated %s -> %s\n", id, status)
	return nil
}

// ── dep ──────────────────────────────────────────────────────────────────────

func (a *app) newDepCmd() *cobra.Command {
	depCmd := &cobra.Command{
		Use:   "dep <id> <dep-id>",
		Short: "Add dependency (id depends on dep-id)",
		// ArbitraryArgs lets non-subcommand positional args through to RunE.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("usage: tk dep <id> <dependency-id>\n       tk dep tree <id>\n       tk dep cycle")
			}
			id, err := a.store.ResolveID(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			depID, err := a.store.ResolveID(args[1])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			t, err := a.store.Load(id)
			if err != nil {
				return err
			}
			if t.HasDep(depID) {
				fmt.Fprintln(a.out, "Dependency already exists")
				return nil
			}
			if err := a.store.AddDep(id, depID); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Added dependency: %s -> %s\n", id, depID)
			return nil
		},
	}

	var fullMode bool
	treeCmd := &cobra.Command{
		Use:   "tree <id>",
		Short: "Show dependency tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := a.store.ResolveID(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			tree, err := a.store.GetDepTree(id, fullMode)
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			fmt.Fprint(a.out, ticket.PrintDepTree(tree, "", true, true))
			return nil
		},
	}
	treeCmd.Flags().BoolVar(&fullMode, "full", false, "Disable deduplication in tree output")

	cycleCmd := &cobra.Command{
		Use:   "cycle",
		Short: "Find dependency cycles in open tickets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cycles, err := a.store.FindCycles()
			if err != nil {
				return err
			}
			if len(cycles) == 0 {
				fmt.Fprintln(a.out, "No dependency cycles found")
				return nil
			}
			for i, cycle := range cycles {
				if i > 0 {
					fmt.Fprintln(a.out)
				}
				fmt.Fprintf(a.out, "Cycle %d: %s\n", i+1, strings.Join(cycle.Path, " -> "))
				for _, t := range cycle.Tickets {
					fmt.Fprintf(a.out, "  %-8s [%s] %s\n", t.ID, t.Status, t.Title)
				}
			}
			return nil
		},
	}

	depCmd.AddCommand(treeCmd, cycleCmd)
	return depCmd
}

// ── undep ────────────────────────────────────────────────────────────────────

func (a *app) newUndepCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undep <id> <dep-id>",
		Short: "Remove dependency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := a.store.ResolveID(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			depID, err := a.store.ResolveID(args[1])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			if err := a.store.RemoveDep(id, depID); err != nil {
				if err.Error() == "dependency not found" {
					fmt.Fprintln(a.out, "Dependency not found")
					return fmt.Errorf("")
				}
				return err
			}
			fmt.Fprintf(a.out, "Removed dependency: %s -/-> %s\n", id, depID)
			return nil
		},
	}
}

// ── link / unlink ─────────────────────────────────────────────────────────────

func (a *app) newLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <id> <id> [id...]",
		Short: "Link tickets together (symmetric)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, len(args))
			for i, arg := range args {
				id, err := a.store.ResolveID(arg)
				if err != nil {
					return fmt.Errorf("Error: %v", err)
				}
				ids[i] = id
			}
			count, err := a.store.LinkTickets(ids)
			if err != nil {
				return err
			}
			if count == 0 {
				fmt.Fprintln(a.out, "All links already exist")
			} else {
				fmt.Fprintf(a.out, "Added %d link(s) between %d tickets\n", count, len(ids))
			}
			return nil
		},
	}
}

func (a *app) newUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <id> <target-id>",
		Short: "Remove link between tickets",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := a.store.ResolveID(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			targetID, err := a.store.ResolveID(args[1])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			t, err := a.store.Load(id)
			if err != nil {
				return err
			}
			if !t.HasLink(targetID) {
				fmt.Fprintln(a.out, "Link not found")
				return fmt.Errorf("")
			}
			if err := a.store.RemoveLink(id, targetID); err != nil {
				return err
			}
			// Best-effort removal from target side; ignore if already absent.
			_ = a.store.RemoveLink(targetID, id)
			fmt.Fprintf(a.out, "Removed link: %s <-> %s\n", id, targetID)
			return nil
		},
	}
}

// ── list / ls / ready / blocked / closed ─────────────────────────────────────

// addListFlags registers the shared filter flags onto cmd and writes the
// parsed values into opts.
func addListFlags(cmd *cobra.Command, opts *ticket.ListOptions) {
	cmd.Flags().StringVar(&opts.Status, "status", "", "Filter by status (open|in_progress|closed)")
	cmd.Flags().StringVarP(&opts.Assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().StringVarP(&opts.Tag, "tag", "T", "", "Filter by tag")
	cmd.Flags().StringVarP(&opts.Type, "type", "t", "", "Filter by type")
}

func (a *app) printTicketsJSON(tickets []*ticket.Ticket) error {
	dependentMap, err := a.store.BuildDependentMap()
	if err != nil {
		return err
	}

	summaries := make([]ticket.TicketSummaryJSON, 0, len(tickets))
	for _, t := range tickets {
		full, err := a.store.Load(t.ID)
		if err != nil {
			full = t
		}
		mtime := a.store.GetMtime(t.ID)
		summary := ticket.NewTicketSummaryJSON(full, dependentMap[t.ID], ticket.CountComments(full.Body), mtime)
		summaries = append(summaries, summary)
	}

	data, err := json.Marshal(summaries)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, string(data))
	return nil
}

func (a *app) newListCmd() *cobra.Command {
	opts := ticket.ListOptions{}
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tickets",
		RunE: func(cmd *cobra.Command, args []string) error {
			tickets, err := a.store.ListTicketsFiltered(opts)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printTicketsJSON(tickets)
			}
			for _, t := range tickets {
				depStr := ""
				if len(t.Deps) > 0 {
					depStr = " <- [" + strings.Join(t.Deps, ", ") + "]"
				}
				fmt.Fprintf(a.out, "%-8s [%s] - %s%s\n", t.ID, t.Status, t.Title, depStr)
			}
			return nil
		},
	}
	addListFlags(cmd, &opts)
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func (a *app) newReadyCmd() *cobra.Command {
	opts := ticket.ListOptions{}
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List open/in-progress tickets with deps resolved",
		RunE: func(cmd *cobra.Command, args []string) error {
			tickets, err := a.store.ReadyTickets(opts)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printTicketsJSON(tickets)
			}
			for _, t := range tickets {
				fmt.Fprintf(a.out, "%-8s [P%d][%s] - %s\n", t.ID, t.Priority, t.Status, t.Title)
			}
			return nil
		},
	}
	addListFlags(cmd, &opts)
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func (a *app) newBlockedCmd() *cobra.Command {
	opts := ticket.ListOptions{}
	cmd := &cobra.Command{
		Use:   "blocked",
		Short: "List open/in-progress tickets with unresolved deps",
		RunE: func(cmd *cobra.Command, args []string) error {
			tickets, blockers, err := a.store.BlockedTickets(opts)
			if err != nil {
				return err
			}
			for i, t := range tickets {
				blockerStr := "[" + strings.Join(blockers[i], ", ") + "]"
				fmt.Fprintf(a.out, "%-8s [P%d][%s] - %s <- %s\n", t.ID, t.Priority, t.Status, t.Title, blockerStr)
			}
			return nil
		},
	}
	addListFlags(cmd, &opts)
	return cmd
}

func (a *app) newClosedCmd() *cobra.Command {
	opts := ticket.ListOptions{}
	var jsonOut bool
	limit := 20
	cmd := &cobra.Command{
		Use:   "closed",
		Short: "List recently closed tickets (default 20, sorted by mtime)",
		RunE: func(cmd *cobra.Command, args []string) error {
			tickets, err := a.store.ClosedTickets(opts, limit)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printTicketsJSON(tickets)
			}
			for _, t := range tickets {
				fmt.Fprintf(a.out, "%-8s [%s] - %s\n", t.ID, t.Status, t.Title)
			}
			return nil
		},
	}
	addListFlags(cmd, &opts)
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum tickets to return")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

// ── show ─────────────────────────────────────────────────────────────────────

func (a *app) newShowCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := a.store.ResolveID(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			if jsonOut {
				t, err := a.store.Load(id)
				if err != nil {
					return fmt.Errorf("Error: %v", err)
				}
				dependentMap, err := a.store.BuildDependentMap()
				if err != nil {
					return err
				}
				mtime := a.store.GetMtime(id)
				summary := ticket.NewTicketSummaryJSON(t, dependentMap[id], ticket.CountComments(t.Body), mtime)
				data, err := json.Marshal(summary)
				if err != nil {
					return err
				}
				fmt.Fprintln(a.out, string(data))
				return nil
			}
			info, err := a.store.GetShowInfo(id)
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			output := ticket.FormatShowInfo(info)
			pager := os.Getenv("TICKET_PAGER")
			if pager == "" {
				pager = os.Getenv("PAGER")
			}
			if pager != "" && isTerminal(os.Stdout) {
				return runWithPager(output, pager)
			}
			fmt.Fprint(a.out, output)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

// ── edit ─────────────────────────────────────────────────────────────────────

func (a *app) newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit [id]",
		Short: "Open ticket in $EDITOR (or create a new ticket)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				t, err := a.store.Create(ticket.CreateOptions{})
				if err != nil {
					return err
				}
				path, err := a.store.GetTicketPath(t.ID)
				if err != nil {
					return fmt.Errorf("Error: %v", err)
				}
				if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
					if err := a.editAndValidateWithTitleRecovery(path); err != nil {
						return err
					}
				} else if err := a.validateEditedTicket(path); err != nil {
					return err
				}
				fmt.Fprintln(a.out, t.ID)
				return nil
			}

			path, err := a.store.GetTicketPath(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			return a.editAndValidateWithTitleRecovery(path)
		},
	}
}

// ── add-note ──────────────────────────────────────────────────────────────────

func (a *app) newAddNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-note <id> [text]",
		Short: "Append timestamped note (or pipe via stdin)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := a.store.ResolveID(args[0])
			if err != nil {
				return fmt.Errorf("Error: %v", err)
			}
			var note string
			if len(args) > 1 {
				note = strings.Join(args[1:], " ")
			} else if !isTerminal(os.Stdin) {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				note = strings.TrimSpace(string(data))
			} else {
				return fmt.Errorf("Error: no note provided")
			}
			if err := a.store.AddNote(id, note); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Note added to %s\n", id)
			return nil
		},
	}
}

// ── query ─────────────────────────────────────────────────────────────────────

func (a *app) newQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query [filter]",
		Short: "Output tickets as JSON (filter: status=open, priority<2, or jq expr)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}
			tickets, err := a.store.QueryTicketsFiltered(filter)
			if err != nil {
				return err
			}
			for _, t := range tickets {
				data, err := json.Marshal(t)
				if err != nil {
					continue
				}
				fmt.Fprintln(a.out, string(data))
			}
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

func openEditor(path string, out io.Writer) error {
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
	fmt.Fprintf(out, "Edit ticket file: %s\n", path)
	return nil
}

func (a *app) editAndValidateWithTitleRecovery(path string) error {
	interactive := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	for {
		if err := openEditor(path, a.out); err != nil {
			return err
		}
		err := a.validateEditedTicket(path)
		if err == nil {
			return nil
		}
		if !interactive || !isMissingTitleError(err) {
			return err
		}

		fmt.Fprintln(a.out, err)
		choice, promptErr := promptTitleRecoveryChoice()
		if promptErr != nil {
			return promptErr
		}
		if choice == "edit" {
			continue
		}
		if err := setTicketHeading(path, "Untitled"); err != nil {
			return fmt.Errorf("Error: failed to apply default title: %w", err)
		}
		return a.validateEditedTicket(path)
	}
}

func promptTitleRecoveryChoice() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(stdout, "Ticket title is missing. [e]dit again or [c]reate anyway with title 'Untitled'? ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return "edit", nil
			}
			return "", fmt.Errorf("Error: failed to read response: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "e", "edit":
			return "edit", nil
		case "c", "create":
			return "create", nil
		}
	}
}

func isMissingTitleError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "first content line must be a non-empty '# ' heading")
}

func setTicketHeading(path, title string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("missing frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return fmt.Errorf("unclosed frontmatter")
	}

	insertAt := end + 1
	for i := end + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		insertAt = i
		break
	}
	if insertAt >= len(lines) {
		lines = append(lines, "# "+title)
	} else {
		lines[insertAt] = "# " + title
	}

	updated := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(updated), 0644)
}

func (a *app) validateEditedTicket(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error: failed to validate edited ticket: %w", err)
	}

	raw := string(content)
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("Error: edited ticket is invalid: missing frontmatter")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return fmt.Errorf("Error: edited ticket is invalid: unclosed frontmatter")
	}

	for i := 1; i < end; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			return fmt.Errorf("Error: edited ticket is invalid: malformed frontmatter line %d", i+1)
		}
	}

	t, err := ticket.ParseTicket(raw)
	if err != nil {
		return fmt.Errorf("Error: edited ticket is invalid: %v", err)
	}

	expectedID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if t.ID == "" || t.ID != expectedID {
		return fmt.Errorf("Error: edited ticket is invalid: id must match file name (%s)", expectedID)
	}

	if !ticket.IsValidStatus(string(t.Status)) {
		return fmt.Errorf("Error: edited ticket is invalid: invalid status %q", t.Status)
	}

	validType := map[string]bool{"bug": true, "feature": true, "task": true, "epic": true, "chore": true}
	if !validType[t.Type] {
		return fmt.Errorf("Error: edited ticket is invalid: invalid type %q", t.Type)
	}

	if t.Created.IsZero() {
		return fmt.Errorf("Error: edited ticket is invalid: invalid created timestamp")
	}

	if t.Priority < 0 || t.Priority > 4 {
		return fmt.Errorf("Error: edited ticket is invalid: priority must be between 0 and 4")
	}

	for _, dep := range t.Deps {
		if !isLikelyTicketID(dep) {
			return fmt.Errorf("Error: edited ticket is invalid: invalid dependency id %q", dep)
		}
		if _, err := a.store.ResolveID(dep); err != nil {
			return fmt.Errorf("Error: edited ticket is invalid: dependency %q not found", dep)
		}
	}

	for _, link := range t.Links {
		if !isLikelyTicketID(link) {
			return fmt.Errorf("Error: edited ticket is invalid: invalid linked id %q", link)
		}
		if _, err := a.store.ResolveID(link); err != nil {
			return fmt.Errorf("Error: edited ticket is invalid: link %q not found", link)
		}
	}

	headline := ""
	for i := end + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		headline = line
		break
	}
	if !strings.HasPrefix(headline, "# ") || strings.TrimSpace(strings.TrimPrefix(headline, "# ")) == "" {
		return fmt.Errorf("Error: edited ticket is invalid: first content line must be a non-empty '# ' heading")
	}

	return nil
}

func isLikelyTicketID(id string) bool {
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		return false
	}
	if parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, ch := range parts[0] {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	for _, ch := range parts[1] {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

var helpText = `tk - minimal ticket system with dependency tracking

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
  ls|list [--status=X] [-a X] [-T X] [--json]   List tickets
  ready [-a X] [-T X] [--json]      List open/in-progress tickets with deps resolved
  blocked [-a X] [-T X]    List open/in-progress tickets with unresolved deps
  closed [--limit=N] [-a X] [-T X] [--json] List recently closed tickets (default 20, by mtime)
  show <id> [--json]       Display ticket
  edit [id]                Open ticket in $EDITOR (or create a new ticket)
  add-note <id> [text]     Append timestamped note (or pipe via stdin)
  query [filter]           Output tickets as JSON (filter: status=open, priority<2, or jq expr)

Tickets stored as markdown files in .tickets/
Supports partial ID matching (e.g., 'tk show 5c4' matches 'nw-5c46')
`
