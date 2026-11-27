package screencap

// pasteboard_darwin.go: the cgo side of pasteboard_darwin.m, the Mac's general
// pasteboard for the viewer's Copy from Mac and Paste to Mac. Every method
// refuses under testing.Testing(), so no test reads or writes the host's
// clipboard.

/*
#cgo darwin LDFLAGS: -framework AppKit
#include <stdlib.h>

int pc_pb_read(int maxBytes, int *outcome, char **text, int *n);
int pc_pb_write(const char *utf8, int n);
int pc_pb_behavior(void);
*/
import "C"

import (
	"testing"
	"unsafe"
)

// ReadOutcome is what a pasteboard read found.
type ReadOutcome int

// The read outcomes, in the order pasteboard_darwin.m answers them.
const (
	ReadOK        ReadOutcome = iota // text was read
	ReadConcealed                    // a password manager or transient item: nothing was read
	ReadEmpty                        // no plain text on the pasteboard
	ReadTooLarge                     // the text is over the limit: nothing was copied
	ReadDenied                       // the system's setting denies this process
)

// Pasteboard reads and writes the general pasteboard. The zero value works.
type Pasteboard struct{}

// NewPasteboard returns the bridge to the real pasteboard.
func NewPasteboard() *Pasteboard { return &Pasteboard{} }

// ReadText reads the pasteboard's plain text, at most maxBytes of UTF-8. Item
// types are checked first: a concealed or transient item answers ReadConcealed
// with no content read. It may wait on a macOS privacy prompt, so a caller
// bounds it.
func (*Pasteboard) ReadText(maxBytes int) (text string, outcome ReadOutcome, err error) {
	if testing.Testing() {
		return "", ReadEmpty, ErrRefusedInTest
	}
	var out, n C.int
	var buf *C.char
	if rc := C.pc_pb_read(C.int(maxBytes), &out, &buf, &n); rc != 0 {
		return "", ReadEmpty, bridgeErr("reading the pasteboard", rc)
	}
	if buf != nil {
		defer C.free(unsafe.Pointer(buf))
		text = C.GoStringN(buf, n)
	}
	return text, ReadOutcome(out), nil
}

// WriteText replaces the pasteboard's contents with one plain string.
func (*Pasteboard) WriteText(s string) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	b := []byte(s)
	var p *C.char
	if len(b) > 0 {
		p = (*C.char)(unsafe.Pointer(&b[0]))
	}
	if rc := C.pc_pb_write(p, C.int(len(b))); rc != 0 {
		return bridgeErr("writing the pasteboard", rc)
	}
	return nil
}

// Behavior names the system's pasteboard access setting for this process:
// default, ask, alwaysAllow, alwaysDeny, or unavailable before macOS 15.4.
func (*Pasteboard) Behavior() (string, error) {
	if testing.Testing() {
		return "", ErrRefusedInTest
	}
	switch C.pc_pb_behavior() {
	case 0:
		return "default", nil
	case 1:
		return "ask", nil
	case 2:
		return "alwaysAllow", nil
	case 3:
		return "alwaysDeny", nil
	}
	return "unavailable", nil
}
