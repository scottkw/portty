// sys_darwin.m: permission and session bridges for the desktop feasibility
// spike. Compiled by cgo with ARC on (see bridge_darwin.go). Every function is
// one C-linkage capability so the Go probes stay thin. Nothing here prompts
// unless its name says request; the preflight calls only read state.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#import <IOKit/pwr_mgt/IOPMLib.h>
#import <ServiceManagement/ServiceManagement.h>
#import <unistd.h>

// Called once per run: macOS 27 can raise an alert per call, so the probes
// never poll this in a loop.
int spk_sr_preflight(void) {
	return CGPreflightScreenCaptureAccess() ? 1 : 0;
}

// Raises the Screen Recording prompt for this process's responsible app.
int spk_sr_request(void) {
	return CGRequestScreenCaptureAccess() ? 1 : 0;
}

// Returned to Go as C strings; each call replaces the previous result, which
// Go copies at once.
static NSData *gSessData, *gSmappData;

static const char *jsonC(id obj, NSData *__strong *slot) {
	NSError *e = nil;
	NSData *d = [NSJSONSerialization dataWithJSONObject:obj options:NSJSONWritingSortedKeys error:&e];
	if (d == nil) {
		NSDictionary *err = @{@"error" : @{@"domain" : e.domain ?: @"spike.json", @"code" : @(e.code),
		                                   @"message" : e.localizedDescription ?: @"json encode failed"}};
		d = [NSJSONSerialization dataWithJSONObject:err options:0 error:nil];
	}
	NSMutableData *m = [d mutableCopy];
	[m appendBytes:"" length:1];
	*slot = m;
	return (const char *)m.bytes;
}

// 1 when Accessibility is granted to this process's responsible app. With
// prompt set it raises the system prompt (the probe's --request).
int spk_ax_trusted(int prompt) {
	if (prompt) {
		NSDictionary *opts = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt : @YES};
		return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts) ? 1 : 0;
	}
	return AXIsProcessTrusted() ? 1 : 0;
}

int spk_post_event_preflight(void) {
	return CGPreflightPostEventAccess() ? 1 : 0;
}

int spk_post_event_request(void) {
	return CGRequestPostEventAccess() ? 1 : 0;
}

// Moves the pointer one point to the right and reports whether it moved: 1
// moved, 0 did not (blocked, or at the right screen edge), -1 no event.
int spk_mouse_nudge(void) {
	CGEventRef here = CGEventCreate(NULL);
	if (here == NULL) return -1;
	CGPoint p = CGEventGetLocation(here);
	CFRelease(here);
	CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, CGPointMake(p.x + 1, p.y), kCGMouseButtonLeft);
	if (move == NULL) return -1;
	CGEventPost(kCGHIDEventTap, move);
	CFRelease(move);
	usleep(100 * 1000);
	CGEventRef after = CGEventCreate(NULL);
	if (after == NULL) return -1;
	CGPoint q = CGEventGetLocation(after);
	CFRelease(after);
	return q.x != p.x || q.y != p.y ? 1 : 0;
}

// Key down then key up per code, 60 ms apart. 0 on success, -1 when an event
// could not be created (the keys before it were already posted).
int spk_post_keycodes(const int *codes, int n) {
	for (int i = 0; i < n; i++) {
		for (int down = 1; down >= 0; down--) {
			CGEventRef e = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)codes[i], down == 1);
			if (e == NULL) return -1;
			CGEventPost(kCGHIDEventTap, e);
			CFRelease(e);
			usleep(60 * 1000);
		}
	}
	return 0;
}

// The CGSession dictionary, every key verbatim. Of interest are
// CGSSessionScreenIsLocked (present only while locked), kCGSSessionOnConsoleKey
// and kCGSessionLoginDoneKey; a NULL dictionary (no window server session) is
// reported as an error object, never as an empty one.
const char *spk_session_dict(void) {
	CFDictionaryRef d = CGSessionCopyCurrentDictionary();
	if (d == NULL) {
		return jsonC(@{@"error" : @{@"domain" : @"spike.cgsession", @"code" : @(-1),
		                            @"message" : @"CGSessionCopyCurrentDictionary returned NULL (no window server session)"}},
		             &gSessData);
	}
	NSMutableDictionary *out = [NSMutableDictionary dictionary];
	for (id key in (__bridge NSDictionary *)d) {
		id v = ((__bridge NSDictionary *)d)[key];
		out[[key description]] = ([v isKindOfClass:[NSString class]] || [v isKindOfClass:[NSNumber class]]) ? v : [v description];
	}
	CFRelease(d);
	return jsonC(out, &gSessData);
}

static IOPMAssertionID gUserAct, gDisplayAssert;

// Tells the system the user is active, which wakes a sleeping display. Returns
// the IOReturn (0 on success).
int spk_user_activity(void) {
	return (int)IOPMAssertionDeclareUserActivity(CFSTR("portty spike: wake"), kIOPMUserActiveLocal, &gUserAct);
}

// on: hold a no-display-sleep assertion; off: release it. Returns the IOReturn.
int spk_display_awake(int on) {
	if (on) {
		if (gDisplayAssert != 0) return 0;
		return (int)IOPMAssertionCreateWithName(kIOPMAssertPreventUserIdleDisplaySleep, kIOPMAssertionLevelOn,
		                                        CFSTR("portty spike: display awake"), &gDisplayAssert);
	}
	if (gDisplayAssert == 0) return 0;
	IOReturn r = IOPMAssertionRelease(gDisplayAssert);
	gDisplayAssert = 0;
	return (int)r;
}

// SMAppService agent registration. verb is register, unregister or status; the
// result is {"status": Enabled|RequiresApproval|NotRegistered|NotFound} plus an
// "error" object (NSError domain, code, message) when the verb failed.
const char *spk_smapp(const char *verb, const char *plistName) {
	NSString *v = @(verb), *name = @(plistName);
	SMAppService *svc = [SMAppService agentServiceWithPlistName:name];
	NSError *err = nil;
	if ([v isEqualToString:@"register"]) {
		[svc registerAndReturnError:&err];
	} else if ([v isEqualToString:@"unregister"]) {
		[svc unregisterAndReturnError:&err];
	} else if (![v isEqualToString:@"status"]) {
		err = [NSError errorWithDomain:@"spike.smapp" code:-1
		                      userInfo:@{NSLocalizedDescriptionKey : [NSString stringWithFormat:@"unknown verb %@", v]}];
	}
	NSString *status = @"NotRegistered";
	switch (svc.status) {
	case SMAppServiceStatusEnabled: status = @"Enabled"; break;
	case SMAppServiceStatusRequiresApproval: status = @"RequiresApproval"; break;
	case SMAppServiceStatusNotRegistered: status = @"NotRegistered"; break;
	case SMAppServiceStatusNotFound: status = @"NotFound"; break;
	}
	NSMutableDictionary *o = [NSMutableDictionary dictionaryWithObject:status forKey:@"status"];
	if (err != nil) {
		o[@"error"] = @{@"domain" : err.domain, @"code" : @(err.code), @"message" : err.localizedDescription};
	}
	return jsonC(o, &gSmappData);
}
