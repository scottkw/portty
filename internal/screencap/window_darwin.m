// window_darwin.m: what the window gate of a window share reads and does: the
// live Accessibility focus, a window's frame and on-screen state, whether it is
// minimized, a raise, and the topmost window at a point. Compiled by cgo with
// ARC on (see window_darwin.go); Core Foundation objects are released by hand.
// Every function returns 0 or an error code and is wrapped in @try/@catch,
// because an uncaught ObjC exception would end the daemon.
//
// The focus is always read from Accessibility, live: NSWorkspace's notion of
// the active application lags a real change, and the gate must never post into
// an app that is no longer focused. A window is matched to its Accessibility
// element by frame (within 2 points, the title only as a tiebreak), never by a
// private window-ID call. Accessibility calls carry a 0.25 s messaging
// timeout, so one hung app cannot stall the daemon's input. This file never
// calls a permission preflight (preflight_darwin.m owns those).

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <CoreGraphics/CoreGraphics.h>

enum { kPCWindowExceptionErr = -90201, kPCWindowNilErr = -90202, kPCRaiseFailed = -90203 };

static const double kPCFrameTolerance = 2.0;
static const float kPCAXTimeout = 0.25f;

// Sets the messaging timeout once, on the system-wide element (which makes it
// the global default).
static void pcAXInit(void) {
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		AXUIElementRef sys = AXUIElementCreateSystemWide();
		if (sys != NULL) {
			AXUIElementSetMessagingTimeout(sys, kPCAXTimeout);
			CFRelease(sys);
		}
	});
}

static BOOL pcNear(CGRect a, CGRect b) {
	return fabs(a.origin.x - b.origin.x) <= kPCFrameTolerance && fabs(a.origin.y - b.origin.y) <= kPCFrameTolerance &&
	       fabs(a.size.width - b.size.width) <= kPCFrameTolerance && fabs(a.size.height - b.size.height) <= kPCFrameTolerance;
}

// The frame of an Accessibility window element, in global points.
static BOOL pcAXFrame(AXUIElementRef win, CGRect *out) {
	CFTypeRef p = NULL, s = NULL;
	BOOL ok = NO;
	if (AXUIElementCopyAttributeValue(win, kAXPositionAttribute, &p) == kAXErrorSuccess && p != NULL &&
	    AXUIElementCopyAttributeValue(win, kAXSizeAttribute, &s) == kAXErrorSuccess && s != NULL &&
	    CFGetTypeID(p) == AXValueGetTypeID() && CFGetTypeID(s) == AXValueGetTypeID()) {
		CGPoint pt;
		CGSize sz;
		if (AXValueGetValue((AXValueRef)p, kAXValueTypeCGPoint, &pt) && AXValueGetValue((AXValueRef)s, kAXValueTypeCGSize, &sz)) {
			*out = CGRectMake(pt.x, pt.y, sz.width, sz.height);
			ok = YES;
		}
	}
	if (p != NULL) CFRelease(p);
	if (s != NULL) CFRelease(s);
	return ok;
}

// Reads the focused application and its focused (else main) window. ok is NO
// when nothing has focus; any other Accessibility failure is returned as is.
static AXError pcFocusRead(pid_t *pid, CGRect *frame, BOOL *ok) {
	*ok = NO;
	AXUIElementRef sys = AXUIElementCreateSystemWide();
	if (sys == NULL) return kAXErrorFailure;
	CFTypeRef app = NULL;
	AXError e = AXUIElementCopyAttributeValue(sys, kAXFocusedApplicationAttribute, &app);
	CFRelease(sys);
	if (e == kAXErrorNoValue) return kAXErrorSuccess;
	if (e != kAXErrorSuccess) return e;
	if (app == NULL) return kAXErrorFailure;
	AXUIElementSetMessagingTimeout((AXUIElementRef)app, kPCAXTimeout);
	pid_t p = 0;
	if (AXUIElementGetPid((AXUIElementRef)app, &p) != kAXErrorSuccess) {
		CFRelease(app);
		return kAXErrorFailure;
	}
	*pid = p;
	CFTypeRef win = NULL;
	AXError we = AXUIElementCopyAttributeValue((AXUIElementRef)app, kAXFocusedWindowAttribute, &win);
	if (we != kAXErrorSuccess || win == NULL) {
		we = AXUIElementCopyAttributeValue((AXUIElementRef)app, kAXMainWindowAttribute, &win);
	}
	AXError result = kAXErrorSuccess;
	if (we == kAXErrorSuccess && win != NULL) {
		*ok = pcAXFrame((AXUIElementRef)win, frame);
	} else if (we != kAXErrorNoValue && we != kAXErrorAttributeUnsupported) {
		result = we; // a hung or failing app: the caller fails closed
	}
	if (win != NULL) CFRelease(win);
	CFRelease(app);
	return result;
}

// The Accessibility element of app's window with this frame; a title match
// wins when several windows share the frame. The caller releases the result.
static AXUIElementRef pcFindAXWindow(AXUIElementRef app, CGRect want, const char *title) {
	CFTypeRef wins = NULL;
	if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &wins) != kAXErrorSuccess || wins == NULL) return NULL;
	if (CFGetTypeID(wins) != CFArrayGetTypeID()) {
		CFRelease(wins);
		return NULL;
	}
	NSString *wantTitle = (title != NULL) ? [NSString stringWithUTF8String:title] : nil;
	AXUIElementRef best = NULL;
	CFIndex n = CFArrayGetCount((CFArrayRef)wins);
	for (CFIndex i = 0; i < n; i++) {
		AXUIElementRef w = (AXUIElementRef)CFArrayGetValueAtIndex((CFArrayRef)wins, i);
		CGRect r;
		if (!pcAXFrame(w, &r) || !pcNear(r, want)) continue;
		if (best == NULL) best = w;
		if (wantTitle.length == 0) continue;
		CFTypeRef t = NULL;
		if (AXUIElementCopyAttributeValue(w, kAXTitleAttribute, &t) == kAXErrorSuccess && t != NULL) {
			BOOL same = CFGetTypeID(t) == CFStringGetTypeID() && [(__bridge NSString *)t isEqualToString:wantTitle];
			CFRelease(t);
			if (same) {
				best = w;
				break;
			}
		}
	}
	if (best != NULL) CFRetain(best);
	CFRelease(wins);
	return best;
}

static AXUIElementRef pcApp(pid_t pid) {
	AXUIElementRef app = AXUIElementCreateApplication(pid);
	if (app != NULL) AXUIElementSetMessagingTimeout(app, kPCAXTimeout);
	return app;
}

int pc_ax_focus(int *pid, double *x, double *y, double *w, double *h, int *ok) {
	@autoreleasepool {
		@try {
			pcAXInit();
			pid_t p = 0;
			CGRect f = CGRectZero;
			BOOL have = NO;
			AXError e = pcFocusRead(&p, &f, &have);
			if (e != kAXErrorSuccess) return (int)e;
			*ok = have ? 1 : 0;
			if (have) {
				*pid = p;
				*x = f.origin.x;
				*y = f.origin.y;
				*w = f.size.width;
				*h = f.size.height;
			}
			return 0;
		} @catch (NSException *ex) {
			return kPCWindowExceptionErr;
		}
	}
}

// Reads the element that holds keyboard focus: the system-wide focused UI
// element, not the focused application's focused window, because a
// non-activating panel (Spotlight, Alfred, Raycast) holds key focus without
// becoming the focused application. Reports the element's owner pid and the
// frame of its window. ok is NO when there is no element or no window (fail
// closed); any other Accessibility failure is returned as is.
static AXError pcKeyFocusRead(pid_t *pid, CGRect *frame, BOOL *ok) {
	*ok = NO;
	AXUIElementRef sys = AXUIElementCreateSystemWide();
	if (sys == NULL) return kAXErrorFailure;
	CFTypeRef el = NULL;
	AXError e = AXUIElementCopyAttributeValue(sys, kAXFocusedUIElementAttribute, &el);
	CFRelease(sys);
	if (e == kAXErrorNoValue) return kAXErrorSuccess;
	if (e != kAXErrorSuccess) return e;
	if (el == NULL) return kAXErrorFailure;
	AXUIElementSetMessagingTimeout((AXUIElementRef)el, kPCAXTimeout);
	pid_t p = 0;
	if (AXUIElementGetPid((AXUIElementRef)el, &p) != kAXErrorSuccess) {
		CFRelease(el);
		return kAXErrorFailure;
	}
	*pid = p;
	CFTypeRef win = NULL;
	AXError we = AXUIElementCopyAttributeValue((AXUIElementRef)el, kAXWindowAttribute, &win);
	AXError result = kAXErrorSuccess;
	if (we == kAXErrorSuccess && win != NULL) {
		*ok = pcAXFrame((AXUIElementRef)win, frame);
	} else if (we == kAXErrorNoValue || we == kAXErrorAttributeUnsupported) {
		// The focused element may itself be the window.
		CFTypeRef role = NULL;
		if (AXUIElementCopyAttributeValue((AXUIElementRef)el, kAXRoleAttribute, &role) == kAXErrorSuccess && role != NULL &&
		    CFGetTypeID(role) == CFStringGetTypeID() && CFEqual(role, kAXWindowRole)) {
			*ok = pcAXFrame((AXUIElementRef)el, frame);
		}
		if (role != NULL) CFRelease(role);
	} else {
		result = we; // a hung or failing app: the caller fails closed
	}
	if (win != NULL) CFRelease(win);
	CFRelease(el);
	return result;
}

int pc_ax_key_focus(int *pid, double *x, double *y, double *w, double *h, int *ok) {
	@autoreleasepool {
		@try {
			pcAXInit();
			pid_t p = 0;
			CGRect f = CGRectZero;
			BOOL have = NO;
			AXError e = pcKeyFocusRead(&p, &f, &have);
			if (e != kAXErrorSuccess) return (int)e;
			*ok = have ? 1 : 0;
			if (have) {
				*pid = p;
				*x = f.origin.x;
				*y = f.origin.y;
				*w = f.size.width;
				*h = f.size.height;
			}
			return 0;
		} @catch (NSException *ex) {
			return kPCWindowExceptionErr;
		}
	}
}

// The frame, owner and on-screen flag of one window by its CGWindowID; a window
// that no longer exists is kPCWindowNilErr.
int pc_window_info(uint32_t wid, int *pid, double *x, double *y, double *w, double *h, int *onScreen) {
	@autoreleasepool {
		@try {
			CFArrayRef raw = CGWindowListCopyWindowInfo(kCGWindowListOptionIncludingWindow, wid);
			if (raw == NULL) return kPCWindowNilErr;
			NSArray *list = CFBridgingRelease(raw);
			for (NSDictionary *d in list) {
				if ([d[(id)kCGWindowNumber] unsignedIntValue] != wid) continue;
				CGRect r;
				CFDictionaryRef bounds = (__bridge CFDictionaryRef)d[(id)kCGWindowBounds];
				if (bounds == NULL || !CGRectMakeWithDictionaryRepresentation(bounds, &r)) return kPCWindowNilErr;
				*pid = [d[(id)kCGWindowOwnerPID] intValue];
				*x = r.origin.x;
				*y = r.origin.y;
				*w = r.size.width;
				*h = r.size.height;
				*onScreen = [d[(id)kCGWindowIsOnscreen] boolValue] ? 1 : 0;
				return 0;
			}
			return kPCWindowNilErr;
		} @catch (NSException *ex) {
			return kPCWindowExceptionErr;
		}
	}
}

int pc_window_minimized(int pid, double x, double y, double w, double h, const char *title, int *minimized) {
	@autoreleasepool {
		@try {
			pcAXInit();
			AXUIElementRef app = pcApp((pid_t)pid);
			if (app == NULL) return kPCWindowNilErr;
			AXUIElementRef win = pcFindAXWindow(app, CGRectMake(x, y, w, h), title);
			CFRelease(app);
			if (win == NULL) return kPCWindowNilErr;
			CFTypeRef v = NULL;
			AXError e = AXUIElementCopyAttributeValue(win, kAXMinimizedAttribute, &v);
			CFRelease(win);
			if (e != kAXErrorSuccess || v == NULL) return e != kAXErrorSuccess ? (int)e : kPCWindowNilErr;
			*minimized = (v == kCFBooleanTrue) ? 1 : 0;
			CFRelease(v);
			return 0;
		} @catch (NSException *ex) {
			return kPCWindowExceptionErr;
		}
	}
}

// Focus is confirmed when the focused window is the wanted one; read up to 3
// times, 50 ms apart, because focus moves a moment after the request.
static BOOL pcFocusConfirmed(pid_t pid, CGRect want) {
	for (int i = 0; i < 3; i++) {
		pid_t p = 0;
		CGRect f = CGRectZero;
		BOOL ok = NO;
		if (pcFocusRead(&p, &f, &ok) == kAXErrorSuccess && ok && p == pid && pcNear(f, want)) return YES;
		if (i < 2) [NSThread sleepForTimeInterval:0.05];
	}
	return NO;
}

// Unminimizes, raises and makes main the window with this frame; front also
// makes its application frontmost.
static void pcAXRaise(pid_t pid, CGRect want, const char *title, BOOL front) {
	AXUIElementRef app = pcApp(pid);
	if (app == NULL) return;
	if (front) AXUIElementSetAttributeValue(app, kAXFrontmostAttribute, kCFBooleanTrue);
	AXUIElementRef win = pcFindAXWindow(app, want, title);
	if (win != NULL) {
		AXUIElementSetAttributeValue(win, kAXMinimizedAttribute, kCFBooleanFalse);
		AXUIElementPerformAction(win, kAXRaiseAction);
		AXUIElementSetAttributeValue(win, kAXMainAttribute, kCFBooleanTrue);
		CFRelease(win);
	}
	CFRelease(app);
}

// The raise ladder; each rung is followed by a read-back of the real focus.
// Returns the first rung (1 Accessibility, 2 NSWorkspace, 3 NSRunningApplication)
// that confirmed focus, else kPCRaiseFailed.
int pc_window_raise(int pid, double x, double y, double w, double h, const char *title) {
	@autoreleasepool {
		@try {
			pcAXInit();
			CGRect want = CGRectMake(x, y, w, h);
			pcAXRaise((pid_t)pid, want, title, YES);
			if (pcFocusConfirmed((pid_t)pid, want)) return 1;

			NSRunningApplication *ra = [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
			NSURL *url = ra.bundleURL;
			if (url != nil) {
				NSWorkspaceOpenConfiguration *cfg = [NSWorkspaceOpenConfiguration configuration];
				cfg.activates = YES;
				dispatch_semaphore_t sem = dispatch_semaphore_create(0);
				[[NSWorkspace sharedWorkspace] openApplicationAtURL:url
				                                      configuration:cfg
				                                  completionHandler:^(NSRunningApplication *app, NSError *err) {
					dispatch_semaphore_signal(sem);
				}];
				dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 150 * NSEC_PER_MSEC));
				pcAXRaise((pid_t)pid, want, title, NO);
				if (pcFocusConfirmed((pid_t)pid, want)) return 2;
			}

			if (ra != nil) {
				[ra activateWithOptions:NSApplicationActivateAllWindows];
				pcAXRaise((pid_t)pid, want, title, NO);
				if (pcFocusConfirmed((pid_t)pid, want)) return 3;
			}
			return kPCRaiseFailed;
		} @catch (NSException *ex) {
			return kPCWindowExceptionErr;
		}
	}
}

// The topmost on-screen window whose bounds contain the point, front to back,
// skipping fully transparent windows and the window server's own. outID is 0
// when nothing is there. Owner pid, layer and frame come from the same entry.
int pc_window_topmost(double x, double y, uint32_t *outID, int *pid, int *layer, double *fx, double *fy, double *fw, double *fh) {
	@autoreleasepool {
		@try {
			*outID = 0;
			CFArrayRef raw = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
			if (raw == NULL) return kPCWindowNilErr;
			NSArray *list = CFBridgingRelease(raw);
			for (NSDictionary *d in list) {
				NSNumber *alpha = d[(id)kCGWindowAlpha];
				if (alpha != nil && alpha.doubleValue <= 0) continue;
				if ([d[(id)kCGWindowOwnerName] isEqualToString:@"Window Server"]) continue;
				CGRect r;
				CFDictionaryRef bounds = (__bridge CFDictionaryRef)d[(id)kCGWindowBounds];
				if (bounds == NULL || !CGRectMakeWithDictionaryRepresentation(bounds, &r)) continue;
				if (x >= CGRectGetMinX(r) && x < CGRectGetMaxX(r) && y >= CGRectGetMinY(r) && y < CGRectGetMaxY(r)) {
					*outID = [d[(id)kCGWindowNumber] unsignedIntValue];
					*pid = [d[(id)kCGWindowOwnerPID] intValue];
					*layer = [d[(id)kCGWindowLayer] intValue];
					*fx = r.origin.x;
					*fy = r.origin.y;
					*fw = r.size.width;
					*fh = r.size.height;
					return 0;
				}
			}
			return 0;
		} @catch (NSException *ex) {
			return kPCWindowExceptionErr;
		}
	}
}
