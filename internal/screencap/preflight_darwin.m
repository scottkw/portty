// preflight_darwin.m: the only file in the repository that reads the live
// Screen Recording, Accessibility and pointer-event permissions. These are the
// non-prompting reads: no variant that raises a system dialog is used anywhere,
// and a process caches the first answer it gets, so only a fresh child process
// reads them (see preflight_darwin.go). Compiled by cgo with ARC on.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>

// Each result is 1 or 0. Returns 0, or -90001 if an ObjC exception was raised.
int pc_preflight(int *sr, int *ax, int *pe) {
	@try {
		*sr = CGPreflightScreenCaptureAccess() ? 1 : 0;
		*ax = AXIsProcessTrusted() ? 1 : 0;
		*pe = CGPreflightPostEventAccess() ? 1 : 0;
		return 0;
	} @catch (NSException *e) {
		return -90001;
	}
}
