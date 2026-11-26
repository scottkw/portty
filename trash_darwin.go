package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Foundation

#include <stdlib.h>

// Implemented in trash_darwin.m.
int porttyTrashItem(const char *path, char *errbuf, int errlen);
*/
import "C"

import (
	"errors"
	"testing"
	"unsafe"
)

// trashItem moves path to the user's Trash (trash_darwin.m). A test binary never does: it
// refuses before any cgo call. The error carries macOS's description, never the path, so a
// caller may log it.
func trashItem(path string) error {
	if testing.Testing() {
		return errors.New("refusing to trash from a test binary")
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var buf [512]C.char
	if C.porttyTrashItem(cpath, &buf[0], C.int(len(buf))) != 0 {
		return errors.New("trash: " + C.GoString(&buf[0]))
	}
	return nil
}
