// sys_darwin.m: permission and session bridges for the desktop feasibility
// spike. Compiled by cgo with ARC on (see bridge_darwin.go). Every function is
// one C-linkage capability so the Go probes stay thin. Nothing here prompts
// unless its name says request; the preflight calls only read state.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>

// Called once per run: macOS 27 can raise an alert per call, so the probes
// never poll this in a loop.
int spk_sr_preflight(void) {
	return CGPreflightScreenCaptureAccess() ? 1 : 0;
}

// Raises the Screen Recording prompt for this process's responsible app.
int spk_sr_request(void) {
	return CGRequestScreenCaptureAccess() ? 1 : 0;
}
