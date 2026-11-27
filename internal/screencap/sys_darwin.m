// sys_darwin.m: the session, power and pointer bridges the desktop state
// machine needs: lock and display state, AC power, power assertions, remote
// user activity and a returning pointer nudge. Compiled by cgo with ARC on (see
// sys_darwin.go). Every function returns 0 or an error code and is wrapped in
// @try/@catch, because an uncaught ObjC exception would end the daemon.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <IOKit/pwr_mgt/IOPMLib.h>
#import <IOKit/ps/IOPowerSources.h>
#import <IOKit/ps/IOPSKeys.h>
#import <unistd.h>

enum { kSysExceptionErr = -90001, kSysNilErr = -90002 };

// The marker input_darwin.m writes into every event Portty posts.
extern int64_t pc_event_marker(void);

// *locked is 1 while the screen is locked. The session dictionary carries the
// lock key only while locked; a NULL dictionary (no window server session) is
// an error, never "unlocked".
int pc_session_locked(int *locked) {
	@try {
		CFDictionaryRef d = CGSessionCopyCurrentDictionary();
		if (d == NULL) return kSysNilErr;
		CFTypeRef v = CFDictionaryGetValue(d, CFSTR("CGSSessionScreenIsLocked"));
		int is = 0;
		if (v != NULL && CFGetTypeID(v) == CFBooleanGetTypeID()) {
			is = CFBooleanGetValue((CFBooleanRef)v) ? 1 : 0;
		} else if (v != NULL && CFGetTypeID(v) == CFNumberGetTypeID()) {
			int n = 0;
			CFNumberGetValue((CFNumberRef)v, kCFNumberIntType, &n);
			is = n != 0;
		}
		CFRelease(d);
		*locked = is;
		return 0;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

int pc_display_asleep(int *asleep) {
	@try {
		*asleep = CGDisplayIsAsleep(CGMainDisplayID()) ? 1 : 0;
		return 0;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

// *ac is 1 when the Mac is powered from the wall (a desktop Mac always is).
int pc_on_ac_power(int *ac) {
	@try {
		CFTypeRef info = IOPSCopyPowerSourcesInfo();
		if (info == NULL) return kSysNilErr;
		CFStringRef type = IOPSGetProvidingPowerSourceType(info); // not owned
		if (type == NULL) {
			CFRelease(info);
			return kSysNilErr;
		}
		*ac = CFStringCompare(type, CFSTR(kIOPMACPowerKey), 0) == kCFCompareEqualTo ? 1 : 0;
		CFRelease(info);
		return 0;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

static int pcHold(CFStringRef type, const char *name, uint32_t *id) {
	@try {
		NSString *n = name != NULL ? [NSString stringWithUTF8String:name] : nil;
		if (n == nil) return kSysNilErr;
		IOPMAssertionID aid = 0;
		IOReturn r = IOPMAssertionCreateWithName(type, kIOPMAssertionLevelOn, (__bridge CFStringRef)n, &aid);
		if (r == kIOReturnSuccess) *id = aid;
		return (int)r;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

// Keeps the display from idling off while held; does not light a dark display.
int pc_hold_display_awake(const char *name, uint32_t *id) {
	return pcHold(kIOPMAssertPreventUserIdleDisplaySleep, name, id);
}

// Keeps the system from idle sleep while held; the display may still dim.
int pc_hold_system_awake(const char *name, uint32_t *id) {
	return pcHold(kIOPMAssertPreventUserIdleSystemSleep, name, id);
}

int pc_release_assertion(uint32_t id) {
	@try {
		return (int)IOPMAssertionRelease((IOPMAssertionID)id);
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

// Declares remote user activity: lights a sleeping display and postpones its
// sleep up to the Energy Saver timer. The returned assertion expires on its own
// and only postpones, so it is released at once; the held display assertion is
// what keeps the display on.
int pc_declare_remote_activity(const char *name) {
	@try {
		NSString *n = name != NULL ? [NSString stringWithUTF8String:name] : nil;
		if (n == nil) return kSysNilErr;
		IOPMAssertionID aid = 0;
		IOReturn r = IOPMAssertionDeclareUserActivity((__bridge CFStringRef)n, kIOPMUserActiveRemote, &aid);
		if (r == kIOReturnSuccess) IOPMAssertionRelease(aid);
		return (int)r;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

// The pointer's x and the main display's horizontal extent, for choosing the
// nudge direction in Go.
int pc_pointer_state(double *x, double *minX, double *maxX) {
	@try {
		CGEventRef e = CGEventCreate(NULL);
		if (e == NULL) return kSysNilErr;
		CGPoint p = CGEventGetLocation(e);
		CFRelease(e);
		CGRect b = CGDisplayBounds(CGMainDisplayID());
		*x = p.x;
		*minX = CGRectGetMinX(b);
		*maxX = CGRectGetMaxX(b);
		return 0;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}

// Posts one pointer move, waits for it to land and reports whether the pointer
// is where it was sent (within half a point).
static int pcMoveTo(CGEventSourceRef src, CGPoint to) {
	CGEventRef move = CGEventCreateMouseEvent(src, kCGEventMouseMoved, to, kCGMouseButtonLeft);
	if (move == NULL) return 0;
	CGEventSetIntegerValueField(move, kCGEventSourceUserData, pc_event_marker());
	CGEventPost(kCGHIDEventTap, move);
	CFRelease(move);
	usleep(50 * 1000);
	CGEventRef now = CGEventCreate(NULL);
	if (now == NULL) return 0;
	CGPoint p = CGEventGetLocation(now);
	CFRelease(now);
	return fabs(p.x - to.x) <= 0.5 && fabs(p.y - to.y) <= 0.5;
}

// Moves the pointer dx points and back. *ok is 1 only when it moved and came
// back to where it started, 0 when the move was blocked or the return failed.
// A private event source keeps the physical modifier state out of the events.
int pc_pointer_nudge(double dx, int *ok) {
	@try {
		CGEventRef here = CGEventCreate(NULL);
		if (here == NULL) return kSysNilErr;
		CGPoint p = CGEventGetLocation(here);
		CFRelease(here);
		CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStatePrivate);
		if (src == NULL) return kSysNilErr;
		int moved = pcMoveTo(src, CGPointMake(p.x + dx, p.y));
		int back = pcMoveTo(src, p); // always attempted, so the pointer is never left displaced
		CFRelease(src);
		*ok = moved && back ? 1 : 0;
		return 0;
	} @catch (NSException *e) {
		return kSysExceptionErr;
	}
}
