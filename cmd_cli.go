package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
)

// usage prints the command reference to stderr.
func usage() {
	fmt.Fprint(os.Stderr, `Usage: portty [command] [flags]

Run with no arguments to launch the desktop GUI.

Commands:
  new [<path>]                                Create a new login-shell session (default: home directory)
  list [--json] [--local]                     List local and remote sessions
  kill <session>                              Kill a session, given by name or ID
  rename <session> <name>                     Rename a session, given by name or ID
  attach <session>                            Attach to a session on this Mac
  attach <host>:<session>                     Attach to a session shared from another Mac (e.g., office-air:build)
  share <session>                             Share a session with your tailnet
  unshare <session>                           Stop sharing a session and disconnect viewers
  open <file>                                 Show an image, PDF or Markdown file in a preview tab
  health                                      Check Tailscale health status
  version                                     Print the Portty version (also --version, -v)
  settings                                    Open the settings panel
  daemon <subcommand>                         Manage the background daemon
    daemon run                                Run daemon in foreground
    daemon install                            Install as system service
    daemon uninstall                          Uninstall system service
    daemon start                              Start the system service
    daemon stop                               Stop the system service
    daemon status [--json]                    Show daemon status

Remote Sessions:
  list                                        Shows this Mac's sessions and sessions shared from Macs on your Tailscale account
  list --local                                Shows only this Mac's sessions

Updates:
  <command> --restart-daemon [args]           Restart a background daemon left running by an older or newer Portty,
                                              even though that ends its sessions (without it, the command refuses).
                                              The flag goes right after the command, before its arguments
  <command> --keep-daemon [args]              Run the command on that daemon instead and leave it running

Run 'portty <command> --help' for command-specific flags.
`)
}

// restartDaemonFlag is the CLI's consent to a version-mismatch restart that
// ends the old daemon's live sessions (DEBT-08). runCLI takes it out of the
// command's arguments (takeLeadingFlag).
const restartDaemonFlag = "--restart-daemon"

// keepDaemonFlag runs a command on a daemon from another build that has live
// sessions, leaving it running (08-06-REVIEW IN-04), e.g. after a GUI Not Now.
const keepDaemonFlag = "--keep-daemon"

// cliNeedsDaemon reports whether a command line (args[0] is the command) uses
// the daemon. health and version never do, nor does a --help (IN-04), so a
// mismatched daemon never makes them refuse. open talks to the running daemon
// that hosts its shell but must never start or restart it (its ensure path
// could end the session the user is typing in; 11-RESEARCH Pattern 4).
func cliNeedsDaemon(args []string) bool {
	if args[0] == "health" || args[0] == "version" || args[0] == "open" {
		return false
	}
	for _, a := range args[1:] {
		switch a {
		case "--":
			return true
		case "-h", "-help", "--help":
			return false
		}
	}
	return true
}

// takeLeadingFlag removes flag from a command line (args[0] is the command)
// and reports whether it was there. 08-06-REVIEW WR-04: it counts only among
// the flags right after the command, where it cannot be an argument's value
// (every CLI flag is a bool); "daemon status" counts as the command. After
// the first argument it is an error, never consent; after "--" it is a value.
func takeLeadingFlag(args []string, flag string) ([]string, bool, error) {
	out, found, leading := []string{args[0]}, false, true
	for i, a := range args[1:] {
		switch {
		case a == "--":
			return append(out, args[1+i:]...), found, nil
		case a == flag && leading:
			found = true
			continue
		case a == flag:
			return nil, false, fmt.Errorf("%s goes right after the command, before its arguments: portty <command> %s …", flag, flag)
		case !strings.HasPrefix(a, "-") && !(i == 0 && args[0] == "daemon" && a == "status"):
			leading = false
		}
		out = append(out, a)
	}
	return out, found, nil
}

// flagFirstArgs turns `portty --restart-daemon <command> …` into
// `portty <command> --restart-daemon …`, so the flag typed first runs the
// command instead of launching the GUI (08-06-REVIEW WR-04).
func flagFirstArgs(args []string) ([]string, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("%s needs a command: portty <command> %s …", args[0], args[0])
	}
	return append([]string{args[1], args[0]}, args[2:]...), nil
}

// ensureCLIDaemon runs ensure for a CLI command. A daemon from another build
// with live sessions is restarted only with --restart-daemon (force); without
// it the command fails with the reason, and the old daemon keeps running.
// IN-04: with --keep-daemon (keep) the command runs on the old daemon instead,
// with no session handoff. UPGR-03: a failed handoff is reported first.
func ensureCLIDaemon(ensure func(context.Context, string, daemon.EnsureOptions) error, socketPath string, force, keep bool) error {
	var refused error
	err := ensure(context.Background(), socketPath, daemon.EnsureOptions{NoHandoff: keep, Start: daemon.StartCLI, Restart: func(q daemon.RestartQuestion) bool {
		ver, n := q.DaemonVersion, q.Sessions
		if q.MoveFailed != "" {
			fmt.Fprintf(os.Stderr, "portty: couldn't move the running sessions to version %s (%s); they are still running on version %s.\n", daemon.BuildVersion, q.MoveFailed, ver)
		}
		switch {
		case keep:
			fmt.Fprintf(os.Stderr, "portty: using the running daemon, version %s (this is %s)\n", ver, daemon.BuildVersion)
		case !force:
			count, them := fmt.Sprintf("%d live sessions", n), "them"
			switch {
			case n == 1:
				count, them = "1 live session", "it"
			case n < 0:
				count = "live sessions"
			}
			refused = fmt.Errorf("the running daemon is version %s with %s; restarting it to version %s would end %s. Re-run as 'portty <command> %s …' to restart anyway, or as 'portty <command> %s …' to use it as it is.",
				ver, count, daemon.BuildVersion, them, restartDaemonFlag, keepDaemonFlag)
		}
		return force
	}})
	if err != nil {
		return err
	}
	if refused != nil {
		return refused
	}
	// UPGR-04: a daemon too old for this build fails here with one line, not
	// on the command's own calls. An unreadable health is no refusal.
	if h, err := daemonHealthOf(socketPath); err == nil && daemon.DaemonAPI(h) < daemon.MinDaemonAPI {
		return fmt.Errorf("the running daemon (version %s, API %d) is too old for this portty (version %s needs API %d): update needed. Re-run as 'portty <command> %s …' to restart it.",
			h.Version, daemon.DaemonAPI(h), daemon.BuildVersion, daemon.MinDaemonAPI, restartDaemonFlag)
	}
	return nil
}

// daemonHealthOf reads the daemon's /health; tests swap it.
var daemonHealthOf = func(socketPath string) (daemon.HealthResponse, error) {
	return daemon.NewDaemonClient(socketPath).HealthInfo()
}

// unknownCommandMessage is what runCLI prints to stderr for an unrecognized command.
func unknownCommandMessage(cmd string) string {
	return fmt.Sprintf("portty: unknown command %q\nRun 'portty --help' for usage.\n", cmd)
}

// cmdNew creates a new login-shell session and writes its UUID to out.
//
//	portty new [<path>]
//
// <path> is optional; if omitted, workDir is "" and the daemon resolves it to
// the home directory. A relative <path> is resolved against the CLI's cwd (the
// daemon's cwd is unrelated). New sessions always run /bin/zsh -l.
//
// Errors go to stderr as `portty new: <err>`.
func cmdNew(client *daemon.DaemonClient, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we emit our own error copy
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "portty new: %v\n", err)
		return err
	}
	positionals := fs.Args()
	if len(positionals) > 1 { // e.g. the removed agent-plus-path form
		err := fmt.Errorf("usage: portty new [<path>] (got %d arguments)", len(positionals))
		fmt.Fprintf(os.Stderr, "portty new: %v\n", err)
		return err
	}
	workDir := ""
	if len(positionals) == 1 {
		abs, err := filepath.Abs(positionals[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "portty new: %v\n", err)
			return err
		}
		workDir = abs
	}
	// Name is the basename of workDir, or "shell" when workDir is empty
	// (the daemon resolves "" to $HOME; the CLI doesn't know $HOME at this layer).
	name := "shell"
	if workDir != "" {
		name = filepath.Base(workDir)
	}
	id, err := client.CreateSession(name, workDir, 0, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "portty new: %v\n", err)
		return err
	}
	fmt.Fprintln(out, id)
	return nil
}

// listOutput is the JSON structure for cmdList --json output.
type listOutput struct {
	Local  []daemon.SessionInfo `json:"local"`
	Remote []listRemoteGroup    `json:"remote,omitempty"`
}

// listRemoteGroup groups remote sessions by peer hostname.
type listRemoteGroup struct {
	Hostname string             `json:"hostname"`
	Sessions []CLIRemoteSession `json:"sessions"`
}

// remoteListGroups keeps reachable peers and names each by its MagicDNS short
// name, the name `portty attach <host>:<session>` takes.
func remoteListGroups(peers []tailnet.Peer, groups []tailnet.PeerSessionMetaGroup) []listRemoteGroup {
	byHostname := make(map[string]tailnet.Peer, len(peers))
	for _, p := range peers {
		byHostname[p.Hostname] = p
	}
	var out []listRemoteGroup
	for _, g := range groups {
		if !g.Reachable {
			continue
		}
		p := byHostname[g.Hostname]
		fqdn := strings.TrimSuffix(p.DNSName, ".")
		short, _, _ := strings.Cut(fqdn, ".")
		if short == "" {
			short = g.Hostname
		}
		sessions := make([]CLIRemoteSession, 0, len(g.Sessions))
		for _, s := range g.Sessions {
			sessions = append(sessions, CLIRemoteSession{ID: s.ID, Name: s.Name, Status: s.Status, Hostname: short, FQDN: fqdn})
		}
		out = append(out, listRemoteGroup{Hostname: short, Sessions: sessions})
	}
	return out
}

// cmdList lists all sessions in a tabwriter table, or as JSON with --json.
// Includes remote sessions from tailnet peers unless --local is specified.
func cmdList(client *daemon.DaemonClient, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	localOnly := fs.Bool("local", false, "show only local sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	sessions, err := client.ListSessions()
	if err != nil {
		return fmt.Errorf("portty list: %w", err)
	}
	if sessions == nil {
		sessions = []daemon.SessionInfo{}
	}

	// Fetch remote sessions (unless --local). Peers on another Tailscale account
	// are already dropped by FetchAllPeerSessionsMeta.
	var remoteGroups []listRemoteGroup
	if !*localOnly {
		peers, _ := client.ListTailnetPeers()
		if len(peers) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			remoteGroups = remoteListGroups(peers, tailnet.FetchAllPeerSessionsMeta(ctx, peers))
		}
	}

	if *jsonOut {
		output := listOutput{
			Local:  sessions,
			Remote: remoteGroups,
		}
		return json.NewEncoder(out).Encode(output)
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOST\tID\tNAME\tSTATUS")
	for _, s := range sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "(local)", s.ID, s.Name, s.State)
	}
	for _, group := range remoteGroups {
		for _, s := range group.Sessions {
			fmt.Fprintf(w, "%s\t%s:%s\t%s\t%s\n", s.Hostname, s.Hostname, s.ID, s.Name, s.Status)
		}
	}
	w.Flush()
	return nil
}

// cmdKill terminates a session silently. The argument resolves like share and
// attach (exact name, exact ID, unique ID prefix of 8+ characters); an
// ambiguous or unknown name kills nothing.
func cmdKill(client *daemon.DaemonClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty kill <session>")
	}
	s, err := lookupSessionStrict(client, args[0])
	if err != nil {
		return err
	}
	if err := client.KillSession(s.ID); err != nil {
		return fmt.Errorf("portty kill: %w", err)
	}
	return nil
}

// cmdRename renames a session silently. The first argument resolves like
// share and attach (exact name, exact ID, unique ID prefix of 8+ characters); an
// ambiguous or unknown name renames nothing.
func cmdRename(client *daemon.DaemonClient, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: portty rename <session> <name>")
	}
	s, err := lookupSessionStrict(client, args[0])
	if err != nil {
		return err
	}
	if err := client.RenameSession(s.ID, args[1]); err != nil {
		return fmt.Errorf("portty rename: %w", err)
	}
	return nil
}

// cmdHealth prints a 4-line Tailscale health key-value block, or JSON with --json.
func cmdHealth(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := webserver.CheckHealth(ctx)
	if *jsonOut {
		return json.NewEncoder(out).Encode(h)
	}
	fmt.Fprintf(out, "%-12s%v\n", "connected:", h.Connected)
	fmt.Fprintf(out, "%-12s%v\n", "has-certs:", h.HasCerts)
	fmt.Fprintf(out, "%-12s%v\n", "ip:", h.IP)
	fmt.Fprintf(out, "%-12s%v\n", "domain:", h.Domain)
	return nil
}

// cmdSettings prints current configuration values in a human-readable format (read-only).
func cmdSettings(client *daemon.DaemonClient, out io.Writer) error {
	socketPath := daemon.DefaultSocketPath()
	fmt.Fprintf(out, "%-14s%s\n", "socket-path:", socketPath)

	port, err := client.GetRelayPort()
	if err != nil {
		fmt.Fprintf(out, "%-14s%s\n", "relay-port:", "(unavailable)")
	} else {
		fmt.Fprintf(out, "%-14s%d\n", "relay-port:", port)
	}
	return nil
}
