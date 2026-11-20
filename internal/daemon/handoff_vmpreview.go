//go:build vmpreview

// VM-only fault injection for the handoff (09-06 integration run, 09-07 VM
// rows). Never in a release build: the vmpreview tag must not appear in
// .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).
//
// PORTTY_VMPREVIEW_HANDOFF_FAULT=<side>:<step>:<action>, with side old or new,
// a step name from handoff.go, and action exit (os.Exit(1) at once, no
// cleanup) or pause<N> (log the pid and sleep N seconds, so the VM run can
// kill -9 the process at a known step). An old-side fault needs the old
// daemon itself to be a vmpreview build started with the variable.

package daemon

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	spec := os.Getenv("PORTTY_VMPREVIEW_HANDOFF_FAULT")
	if spec == "" {
		return
	}
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) != 3 {
		log.Printf("handoff-fault: ignoring malformed PORTTY_VMPREVIEW_HANDOFF_FAULT=%q", spec)
		return
	}
	side, step, action := parts[0], parts[1], parts[2]
	handoffFault = func(s, st string) error {
		if s != side || st != step {
			return nil
		}
		switch {
		case action == "exit":
			log.Printf("handoff-fault: exiting at %s:%s pid=%d", s, st, os.Getpid())
			os.Exit(1)
		case strings.HasPrefix(action, "pause"):
			n, _ := strconv.Atoi(strings.TrimPrefix(action, "pause"))
			log.Printf("handoff-fault: paused at %s:%s pid=%d", s, st, os.Getpid())
			time.Sleep(time.Duration(n) * time.Second)
		}
		return nil
	}
}
