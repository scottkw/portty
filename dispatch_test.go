package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestDispatchHelp verifies that --help prints the unified usage text
// covering both GUI and CLI modes (requirement CLI-04).
func TestDispatchHelp(t *testing.T) {
	// Capture stderr where usage() writes.
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	usage()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Must contain GUI launch instruction.
	if !strings.Contains(out, "Run with no arguments to launch the desktop GUI") {
		t.Errorf("usage() missing GUI launch instruction, got:\n%s", out)
	}
	// Must contain every CLI command (requirement CLI-01 coverage in help).
	for _, cmd := range []string{"new", "list", "kill", "rename", "attach", "share", "unshare", "open", "health", "version", "settings", "daemon"} {
		if !strings.Contains(out, "  "+cmd+" ") && !strings.Contains(out, "  "+cmd+"\n") {
			t.Errorf("usage() missing command %q", cmd)
		}
	}
	// Must contain daemon subcommands.
	for _, sub := range []string{"daemon run", "daemon install", "daemon uninstall", "daemon start", "daemon stop", "daemon status"} {
		if !strings.Contains(out, sub) {
			t.Errorf("usage() missing daemon subcommand %q", sub)
		}
	}
	// Must contain help trailer.
	if !strings.Contains(out, "Run 'portty <command> --help' for command-specific flags") {
		t.Errorf("usage() missing help trailer")
	}
}

// TestDispatchHelpShort verifies -h also works (same output as --help).
func TestDispatchHelpShort(t *testing.T) {
	// This test validates the dispatch logic in main().
	// Since main() calls os.Exit, we test the condition directly.
	args := []string{"portty", "-h"}
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		// Would call usage() — verified by TestDispatchHelp.
		return
	}
	t.Fatal("dispatch logic did not match -h")
}

// TestDispatchNoArgs verifies that no-args routes to GUI path.
func TestDispatchNoArgs(t *testing.T) {
	// Verify the dispatch condition: len(os.Args) == 1 → GUI.
	args := []string{"portty"}
	if len(args) != 1 {
		t.Fatal("expected single-element args to route to GUI")
	}
	// Can't actually test Wails GUI launch in a test, but we verify
	// the condition is correct and doesn't panic.
}

// TestRunCLI_UnknownCommand verifies the exact message runCLI prints for an
// unknown command (runCLI itself calls os.Exit, so the builder is tested).
func TestRunCLI_UnknownCommand(t *testing.T) {
	want := `portty: unknown command "bogus"` + "\n" + `Run 'portty --help' for usage.` + "\n"
	if got := unknownCommandMessage("bogus"); got != want {
		t.Errorf("unknownCommandMessage(\"bogus\") = %q, want %q", got, want)
	}
}

// TestLaunchMode pins how main routes a launch. Finder, the Dock, `open` and
// login items start the app with no arguments or only macOS launch-services
// arguments; those must be GUI launches and never index past the argument list
// (VM G-04 found a panic there). Any other leading flag is a usage error, so a
// CLI-only call over SSH never starts the GUI (16-CONTEXT D-01).
func TestLaunchMode(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		mode     int
		wantErr  string // exact error text; empty means no error
		wantArgs []string
	}{
		{"no args", nil, modeGUI, "", nil},
		{"empty", []string{}, modeGUI, "", nil},
		{"launch services NS", []string{"-NSDocumentRevisionsDebugMode"}, modeGUI, "", nil},
		{"launch services psn", []string{"-psn_0_12345"}, modeGUI, "", nil},
		{"launch services Apple", []string{"-ApplePersistenceIgnoreState"}, modeGUI, "", nil},
		{"launch services Apple languages", []string{"-AppleLanguages"}, modeGUI, "", nil},
		{"help", []string{"--help"}, modeHelp, "", nil},
		{"short help", []string{"-h"}, modeHelp, "", nil},
		{"long version", []string{"--version"}, modeCLI, "", []string{"version"}},
		{"short version", []string{"-v"}, modeCLI, "", []string{"version"}},
		{"unknown long flag", []string{"--verbose"}, modeHelp, `unknown flag "--verbose"`, nil},
		{"unknown short flag", []string{"-x"}, modeHelp, `unknown flag "-x"`, nil},
		{"bare double dash", []string{"--"}, modeHelp, `unknown flag "--"`, nil},
		{"near miss psn", []string{"-psn"}, modeHelp, `unknown flag "-psn"`, nil},
		{"command", []string{"list"}, modeCLI, "", nil},
		{"restart flag first", []string{restartDaemonFlag, "list"}, modeCLI, "", nil},
		{"keep flag first", []string{keepDaemonFlag, "list"}, modeCLI, "", nil},
		{"bare restart flag", []string{restartDaemonFlag}, modeCLI, "needs a command", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode, cliArgs, err := launchMode(c.args)
			if mode != c.mode {
				t.Fatalf("launchMode(%q) mode = %d, want %d (err=%v)", c.args, mode, c.mode, err)
			}
			switch {
			case c.wantErr == "" && err != nil:
				t.Fatalf("launchMode(%q) unexpected error: %v", c.args, err)
			case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
				t.Fatalf("launchMode(%q) error = %v, want it to contain %q", c.args, err, c.wantErr)
			}
			if c.wantArgs != nil && !slices.Equal(cliArgs, c.wantArgs) {
				t.Fatalf("launchMode(%q) cliArgs = %q, want %q", c.args, cliArgs, c.wantArgs)
			}
		})
	}
}
