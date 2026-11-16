package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
	qrcode "github.com/skip2/go-qrcode"
)

// usage prints the command reference to stderr.
func usage() {
	fmt.Fprint(os.Stderr, `Usage: portty [command] [flags]

Run with no arguments to launch the desktop GUI.

Commands:
  new [<path>]                                Create a new login-shell session (default: home directory)
  list [--json] [--local]                     List local and remote sessions
  kill <id>                                   Kill a session
  rename <id> <name>                          Rename a session
  attach <id>                                 Attach to a local or remote session
  serve <id>                                  Enable web sharing for a session
  unserve <id>                                Disable web sharing for a session
  web                                         Open the web dashboard
  health                                      Check Tailscale health status
  qr <id>                                     Print QR code for a session URL
  settings                                    Open the settings panel
  daemon <subcommand>                         Manage the background daemon
    daemon run                                Run daemon in foreground
    daemon install                            Install as system service
    daemon uninstall                          Uninstall system service
    daemon start                              Start the system service
    daemon stop                               Stop the system service
    daemon status [--json]                    Show daemon status

Remote Sessions:
  list                                        Shows local + remote sessions grouped by host
  list --local                                Shows only local sessions
  attach <hostname:session-id>                Attach to a remote session (e.g., macbook:abc123)

Run 'portty <command> --help' for command-specific flags.
`)
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

	// Fetch remote sessions (unless --local).
	var remoteGroups []listRemoteGroup
	if !*localOnly {
		peers, _ := client.ListTailnetPeers()
		if len(peers) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			groups := tailnet.FetchAllPeerSessions(ctx, peers)
			for _, g := range groups {
				cliSessions := make([]CLIRemoteSession, 0, len(g.Sessions))
				for _, s := range g.Sessions {
					cliSessions = append(cliSessions, CLIRemoteSession{
						ID:       s.ID,
						Name:     s.Name,
						Status:   s.Status,
						Hostname: s.Hostname,
						FQDN:     s.FQDN,
					})
				}
				remoteGroups = append(remoteGroups, listRemoteGroup{
					Hostname: g.Hostname,
					Sessions: cliSessions,
				})
			}
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

// cmdKill terminates a session silently.
func cmdKill(client *daemon.DaemonClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty kill <session-id>")
	}
	if err := client.KillSession(args[0]); err != nil {
		return fmt.Errorf("portty kill: %w", err)
	}
	return nil
}

// cmdRename renames a session silently.
func cmdRename(client *daemon.DaemonClient, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: portty rename <session-id> <name>")
	}
	if err := client.RenameSession(args[0], args[1]); err != nil {
		return fmt.Errorf("portty rename: %w", err)
	}
	return nil
}

// cmdWeb dispatches web sub-commands: start, stop, status.
func cmdWeb(client *daemon.DaemonClient, args []string, out io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty web <start|stop|status>")
	}
	switch args[0] {
	case "start":
		return cmdWebStart(client, args[1:], out)
	case "stop":
		return cmdWebStop(client)
	case "status":
		return cmdWebStatus(client, args[1:], out)
	default:
		return fmt.Errorf("usage: portty web <start|stop|status>")
	}
}

// cmdWebStart validates Tailscale health then starts the web server, printing the URL.
func cmdWebStart(client *daemon.DaemonClient, _ []string, out io.Writer) error {
	port := 443
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := webserver.CheckHealth(ctx)
	if !h.Connected {
		return fmt.Errorf("Tailscale is not connected")
	}
	if h.IP == "" {
		return fmt.Errorf("Tailscale IP not available")
	}
	if !h.HasCerts {
		return fmt.Errorf("Tailscale HTTPS certificates not enabled")
	}
	url, err := client.StartWebServer(h.IP, port, h.Domain, "tailscale", "")
	if err != nil {
		return fmt.Errorf("portty web start: %w", err)
	}
	fmt.Fprintln(out, url)
	return nil
}

// cmdWebStop stops the web server silently.
func cmdWebStop(client *daemon.DaemonClient) error {
	if err := client.StopWebServer(); err != nil {
		return fmt.Errorf("portty web stop: %w", err)
	}
	return nil
}

// cmdWebStatus prints the web server running/url/addr key-value block, or JSON with --json.
func cmdWebStatus(client *daemon.DaemonClient, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("web-status", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resp, err := client.GetWebServerStatus()
	if err != nil {
		return fmt.Errorf("portty web status: %w", err)
	}
	if *jsonOut {
		return json.NewEncoder(out).Encode(resp)
	}
	fmt.Fprintf(out, "%-12s%v\n", "running:", resp.Running)
	if resp.Running {
		fmt.Fprintf(out, "%-12s%v\n", "url:", resp.URL)
		fmt.Fprintf(out, "%-12s%v\n", "addr:", resp.Addr)
	}
	return nil
}

// cmdServe enables web serving for the given session.
func cmdServe(client *daemon.DaemonClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty serve <session-id>")
	}
	if err := client.ToggleWebServing(args[0], true); err != nil {
		return fmt.Errorf("portty serve: %w", err)
	}
	return nil
}

// cmdUnserve disables web serving for the given session.
func cmdUnserve(client *daemon.DaemonClient, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty unserve <session-id>")
	}
	if err := client.ToggleWebServing(args[0], false); err != nil {
		return fmt.Errorf("portty unserve: %w", err)
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

// cmdQR renders a QR code for the given session URL in the terminal.
func cmdQR(client *daemon.DaemonClient, args []string, out io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: portty qr <session-id>")
	}
	resp, err := client.GetWebServerStatus()
	if err != nil || !resp.Running {
		return fmt.Errorf("web server not running — use 'portty web start' first")
	}
	url := fmt.Sprintf("%s/sessions/%s", resp.URL, args[0])
	q, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("portty qr: %w", err)
	}
	fmt.Fprint(out, q.ToSmallString(false))
	fmt.Fprintln(out, url)
	return nil
}
