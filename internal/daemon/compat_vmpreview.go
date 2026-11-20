//go:build vmpreview

// VM-only hook for the UPGR-04 "update needed" check in the Tart VM (09-07).
// Never in a release build: the vmpreview tag must not appear in .github,
// build.sh or scripts (tests/check-brand.sh enforces this in CI).

package daemon

import (
	"os"
	"strconv"
)

// PORTTY_VMPREVIEW_MIN_DAEMON_API raises MinDaemonAPI so a GUI or CLI built
// with vmpreview treats this release's daemon as too old.
func init() {
	if n, err := strconv.Atoi(os.Getenv("PORTTY_VMPREVIEW_MIN_DAEMON_API")); err == nil && n > 0 {
		MinDaemonAPI = n
	}
}
