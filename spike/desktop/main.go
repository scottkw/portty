// Command spike is the throwaway desktop-feasibility binary: a pion server
// (serve), its loopback receiver (selftest), and the R00 and permission probes
// every launch path runs. It is a separate module and is never imported by the
// app.
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
  serve       run the pion server (token-gated viewer, /offer, /report)
  selftest    run synthetic frames through the encoder and pion to a Go receiver
  vmprobe     R00: ScreenCaptureKit frames, IOSurface and VideoToolbox encoders (--encoders-only is host-safe)
  probe       one probe record: ids, session, TCC answers, capture (--no-tcc is host-safe; --preflight-only never prompts)
  probe-loop  a probe record every --interval, appended as JSONL (--stream keeps one SCStream)
  opusprobe   R21: AudioConverter Opus encode of a sine (host-safe) or of ScreenCaptureKit system audio (--source sck)
  agent       run the subcommand named by --config <json> (the launchd entry of both plists)
  spawn       re-exec this binary detached in its own session: spawn [--quit-after] -- <argv>
  register    SMAppService register of the helper agent plist
  unregister  SMAppService unregister of the helper agent plist
  status      SMAppService status of the helper agent plist (no TCC call)
  swap        exchange two paths atomically (RENAME_SWAP): swap <a> <b>
`

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// dispatch runs one subcommand; found is false for an unknown name.
func dispatch(ctx context.Context, cmd string, args []string) (found bool, err error) {
	switch cmd {
	case "serve":
		err = serve(ctx, args)
	case "selftest":
		err = selftest(ctx, args)
	case "vmprobe":
		err = vmprobe(ctx, args)
	case "probe":
		err = probeCmd(ctx, args)
	case "probe-loop":
		err = probeLoop(ctx, args)
	case "opusprobe":
		err = opusProbe(ctx, args)
	case "agent":
		err = agent(ctx, args)
	case "spawn":
		err = spawn(ctx, args)
	case "register", "unregister", "status":
		err = smappCmd(cmd)(ctx, args)
	case "swap":
		err = swap(ctx, args)
	default:
		return false, nil
	}
	return true, err
}

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		// A bare invocation is a request for help, so it exits 0.
		_, _ = fmt.Fprint(stderr, usage)
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	found, err := dispatch(ctx, args[0], args[1:])
	if !found {
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
