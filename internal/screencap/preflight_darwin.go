package screencap

/*
#include <stdint.h>

int pc_preflight(int *sr, int *ax, int *pe);
*/
import "C"

import (
	"fmt"
	"testing"
)

// Preflight reads the live Screen Recording, Accessibility and pointer-event
// permissions without ever prompting. It is for the `portty daemon perm-probe`
// child only: a long-running process caches the first answer it gets for each
// of these (research C1), so the daemon asks a freshly spawned child instead of
// calling this itself. It refuses under testing.Testing() so no test reads TCC.
func Preflight() (Permissions, error) {
	if testing.Testing() {
		return Permissions{}, ErrRefusedInTest
	}
	var sr, ax, pe C.int
	if rc := C.pc_preflight(&sr, &ax, &pe); rc != 0 {
		return Permissions{}, fmt.Errorf("screencap: permission preflight failed (code %d)", int(rc))
	}
	return Permissions{SR: sr != 0, AX: ax != 0, PE: pe != 0}, nil
}
