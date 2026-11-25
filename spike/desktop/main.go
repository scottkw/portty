// Command spike is the throwaway desktop-feasibility binary: a pion server
// (serve) and its loopback receiver (selftest). It is a separate module and is
// never imported by the app.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const usage = `usage: spike <command> [flags]

commands:
  serve     run the pion server (token-gated viewer, /offer, /report)
  selftest  run synthetic frames through the encoder and pion to a Go receiver
`

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch args[0] {
	case "serve":
		err = serve(ctx, args[1:])
	case "selftest":
		err = selftest(ctx, args[1:])
	default:
		_, _ = fmt.Fprintf(stderr, "spike: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errBindRefused), errors.Is(err, errUsage):
		_, _ = fmt.Fprintf(stderr, "spike %s: %v\n", args[0], err)
		return 2
	default:
		_, _ = fmt.Fprintf(stderr, "spike %s: %v\n", args[0], err)
		return 1
	}
}

func serve(ctx context.Context, args []string) error {
	fs := newFlagSet("serve")
	var o serveOpts
	fs.StringVar(&o.bind, "bind", "", "IPv4 address to bind (loopback, 100.64.0.0/10, or any with --allow-lan)")
	fs.IntVar(&o.httpPort, "http-port", 47800, "HTTP port")
	fs.IntVar(&o.udpPort, "udp-port", 47801, "UDP port for the ICE mux")
	fs.StringVar(&o.token, "token", "", "32 hex characters (default: random, printed once)")
	fs.BoolVar(&o.allowLAN, "allow-lan", false, "allow a bind address outside loopback and 100.64.0.0/10")
	fs.StringVar(&o.source, "source", "synthetic", "synthetic or sck")
	fs.IntVar(&o.widthCap, "width-cap", 2560, "frame width cap in pixels")
	fs.IntVar(&o.fps, "fps", 30, "frames per second")
	fs.IntVar(&o.bitrate, "bitrate", 8_000_000, "average bitrate in bits per second")
	fs.StringVar(&o.evidenceDir, "evidence-dir", "", "directory for /report files")
	fs.DurationVar(&o.duration, "duration", 0, "stop after this long (0 = until signalled)")
	fs.StringVar(&o.tlsCert, "tls-cert", "", "TLS certificate (with --tls-key serves HTTPS)")
	fs.StringVar(&o.tlsKey, "tls-key", "", "TLS key")
	fs.BoolVar(&o.spsPatch, "sps-patch", false, "rewrite the SPS constraint byte 0xc0 to 0xe0 (browser fallback)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if o.bind == "" {
		return fmt.Errorf("%w: --bind is required", errUsage)
	}
	if o.fps <= 0 || o.widthCap < 16 || o.bitrate <= 0 || o.duration < 0 {
		return fmt.Errorf("%w: --fps, --width-cap and --bitrate must be positive", errUsage)
	}
	return runServe(ctx, o, nil)
}
