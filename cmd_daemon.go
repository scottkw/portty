package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/screencap"
)

// serviceControlFunc is the function used to control the service. Package-level var
// allows test injection of a mock.
var serviceControlFunc = daemon.AgentControl

// runTakeoverFunc runs `portty daemon --takeover`; a var for tests.
var runTakeoverFunc = daemon.RunTakeover

// runDaemonFunc runs the daemon in this process; a var for tests.
var runDaemonFunc = daemon.RunDaemon

// wireDesktopNotifier gives the daemon the native sender for the desktop's
// once-per-lapse notification. The daemon posts only when no running GUI claims
// the lapse (CONTEXT [Permissions]: the GUI when it runs, otherwise the daemon).
// A var for tests.
var wireDesktopNotifier = func() { daemon.SetDesktopNotifier(sendNotification) }

// permProbeFunc reads the live permissions without prompting; a var for tests.
var permProbeFunc = screencap.Preflight

// cmdDaemon dispatches daemon subcommands: install, uninstall, start, stop, run.
// When called with no arguments, it runs the daemon directly: launchd starts
// `portty daemon` (the LaunchAgent), and so does EnsureDaemon's fallback spawn.
func cmdDaemon(args []string, out io.Writer) error {
	if len(args) == 0 {
		// No subcommand: run daemon directly (backward compat with EnsureDaemon).
		wireDesktopNotifier()
		runDaemonFunc()
		return nil
	}
	switch args[0] {
	case "run":
		wireDesktopNotifier()
		runDaemonFunc()
		return nil
	case "--takeover": // internal, so not in the usage: only EnsureDaemon spawns it (09-05)
		wireDesktopNotifier()
		runTakeoverFunc()
		return nil
	case "perm-probe": // internal, so not in the usage: only the daemon's Prober spawns it (19-09)
		// A fresh process is the only honest reader of the live grants: a long-running
		// one keeps the first answer it got (research C1). One JSON line, no other effect.
		p, err := permProbeFunc()
		if err != nil {
			return fmt.Errorf("portty daemon perm-probe: %w", err)
		}
		return json.NewEncoder(out).Encode(struct {
			SR bool `json:"sr"`
			AX bool `json:"ax"`
			PE bool `json:"pe"`
		}{p.SR, p.AX, p.PE})
	case "install":
		if err := serviceControlFunc("install"); err != nil {
			return fmt.Errorf("portty daemon install: %w", err)
		}
		fmt.Fprintln(out, "daemon service installed")
		return nil
	case "uninstall":
		if err := serviceControlFunc("uninstall"); err != nil {
			return fmt.Errorf("portty daemon uninstall: %w", err)
		}
		fmt.Fprintln(out, "daemon service uninstalled")
		return nil
	case "start":
		if err := serviceControlFunc("start"); err != nil {
			return fmt.Errorf("portty daemon start: %w", err)
		}
		fmt.Fprintln(out, "daemon service started")
		return nil
	case "stop":
		if err := serviceControlFunc("stop"); err != nil {
			return fmt.Errorf("portty daemon stop: %w", err)
		}
		fmt.Fprintln(out, "daemon service stopped")
		return nil
	default:
		return fmt.Errorf("unknown daemon subcommand %q; usage: portty daemon <install|uninstall|start|stop|status|run>", args[0])
	}
}

// cmdDaemonStatus checks daemon reachability and prints status.
func cmdDaemonStatus(client *daemon.DaemonClient, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("daemon-status", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	running := client.Health() == nil

	if *jsonOut {
		type statusResp struct {
			Running bool `json:"running"`
		}
		return json.NewEncoder(out).Encode(statusResp{Running: running})
	}
	fmt.Fprintf(out, "%-12s%v\n", "running:", running)
	return nil
}
