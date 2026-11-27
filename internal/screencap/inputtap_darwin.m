// inputtap_darwin.m: tells the person at the Mac from Portty's own posts. A
// listen-only session event tap, on its own thread and run loop, stores the
// time of the last event that does not carry Portty's marker in
// kCGEventSourceUserData. Compiled by cgo with ARC on (see inputtap_darwin.go).
//
// The tap is listen-only on purpose: an active tap hangs system-wide input when
// Accessibility is revoked, a listen-only one just goes quiet. The callback
// reads no key code, character or coordinate and logs nothing; it keeps one
// timestamp. HID idle time is not used: every injected event resets it.
// Every entry point is wrapped in @try/@catch, because an uncaught ObjC
// exception would end the daemon.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <Carbon/Carbon.h>
#import <mach/mach_time.h>
#import <pthread.h>
#import <stdatomic.h>

enum {
	kPCTapExceptionErr = -90401,
	kPCTapCreateErr = -90402,   // CGEventTapCreate returned NULL
	kPCTapDisabledErr = -90403, // created but not enabled: no Accessibility
	kPCTapThreadErr = -90404
};

// The marker input_darwin.m writes into every event Portty posts.
extern int64_t pc_event_marker(void);

// Written by the tap thread, read from any thread. 0 means never.
static _Atomic uint64_t g_last_untagged;
static _Atomic uint64_t g_last_disabled;
static _Atomic int g_dead;     // disabled by user input: recreate on the next poll
static _Atomic int g_stop;     // asks the tap thread to leave its run loop
static int64_t g_marker;

// Everything below is guarded by g_mu.
static pthread_mutex_t g_mu = PTHREAD_MUTEX_INITIALIZER;
static CFMachPortRef g_tap;
static CFRunLoopSourceRef g_source;
static CFRunLoopRef g_runloop;
static pthread_t g_thread;
static int g_running;
static int g_wanted; // StartInputTap succeeded and StopInputTap has not run
static dispatch_semaphore_t g_ready;

static CGEventRef tapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef ev, void *ud) {
	if (type == kCGEventTapDisabledByTimeout) {
		atomic_store_explicit(&g_last_disabled, mach_absolute_time(), memory_order_relaxed);
		CGEventTapEnable(g_tap, true);
		return ev;
	}
	if (type == kCGEventTapDisabledByUserInput) {
		atomic_store_explicit(&g_last_disabled, mach_absolute_time(), memory_order_relaxed);
		atomic_store_explicit(&g_dead, 1, memory_order_relaxed);
		return ev;
	}
	if (CGEventGetIntegerValueField(ev, kCGEventSourceUserData) != g_marker) {
		atomic_store_explicit(&g_last_untagged, mach_absolute_time(), memory_order_relaxed);
	}
	return ev; // listen-only: never modified, never dropped
}

static void *tapThread(void *arg) {
	@autoreleasepool {
		CFRunLoopRef rl = CFRunLoopGetCurrent();
		CFRunLoopAddSource(rl, g_source, kCFRunLoopCommonModes);
		g_runloop = (CFRunLoopRef)CFRetain(rl);
		dispatch_semaphore_signal(g_ready);
		// CFRunLoopStop before the loop runs is lost, so the flag is the truth
		// and the 1 s slice only bounds that race.
		while (!atomic_load_explicit(&g_stop, memory_order_acquire)) {
			CFRunLoopRunInMode(kCFRunLoopDefaultMode, 1.0, false);
		}
		CFRunLoopRemoveSource(rl, g_source, kCFRunLoopCommonModes);
	}
	return NULL;
}

// Releases the tap and joins its thread. g_mu held.
static void tapTeardownLocked(void) {
	if (g_running) {
		atomic_store_explicit(&g_stop, 1, memory_order_release);
		if (g_runloop != NULL) CFRunLoopStop(g_runloop);
		pthread_join(g_thread, NULL);
		g_running = 0;
	}
	if (g_tap != NULL) {
		CGEventTapEnable(g_tap, false);
		CFMachPortInvalidate(g_tap);
	}
	if (g_source != NULL) { CFRelease(g_source); g_source = NULL; }
	if (g_tap != NULL) { CFRelease(g_tap); g_tap = NULL; }
	if (g_runloop != NULL) { CFRelease(g_runloop); g_runloop = NULL; }
}

// Creates the tap and starts its thread. g_mu held. Keeps the last-seen times.
static int tapCreateLocked(void) {
	g_marker = pc_event_marker();
	CGEventMask mask =
		CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp) |
		CGEventMaskBit(kCGEventFlagsChanged) | CGEventMaskBit(kCGEventMouseMoved) |
		CGEventMaskBit(kCGEventLeftMouseDown) | CGEventMaskBit(kCGEventLeftMouseUp) |
		CGEventMaskBit(kCGEventRightMouseDown) | CGEventMaskBit(kCGEventRightMouseUp) |
		CGEventMaskBit(kCGEventOtherMouseDown) | CGEventMaskBit(kCGEventOtherMouseUp) |
		CGEventMaskBit(kCGEventLeftMouseDragged) | CGEventMaskBit(kCGEventRightMouseDragged) |
		CGEventMaskBit(kCGEventOtherMouseDragged) | CGEventMaskBit(kCGEventScrollWheel);
	g_tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionListenOnly,
		mask, tapCallback, NULL);
	if (g_tap == NULL) return kPCTapCreateErr;
	CGEventTapEnable(g_tap, true);
	// A non-NULL tap without Accessibility is created disabled.
	if (!CGEventTapIsEnabled(g_tap)) {
		tapTeardownLocked();
		return kPCTapDisabledErr;
	}
	g_source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, g_tap, 0);
	if (g_source == NULL) {
		tapTeardownLocked();
		return kPCTapCreateErr;
	}
	atomic_store(&g_stop, 0);
	atomic_store(&g_dead, 0);
	g_ready = dispatch_semaphore_create(0);
	if (pthread_create(&g_thread, NULL, tapThread, NULL) != 0) {
		tapTeardownLocked();
		return kPCTapThreadErr;
	}
	g_running = 1;
	dispatch_semaphore_wait(g_ready, DISPATCH_TIME_FOREVER);
	return 0;
}

// Starts the tap (no-op when running). 0 or an error code.
int pc_tap_start(void) {
	@try {
		pthread_mutex_lock(&g_mu);
		int rc = 0;
		if (!g_running) {
			atomic_store(&g_last_untagged, 0);
			atomic_store(&g_last_disabled, 0);
			rc = tapCreateLocked();
		}
		g_wanted = (rc == 0);
		pthread_mutex_unlock(&g_mu);
		return rc;
	} @catch (NSException *e) {
		pthread_mutex_unlock(&g_mu);
		return kPCTapExceptionErr;
	}
}

// Stops the tap and forgets what it saw.
void pc_tap_stop(void) {
	@try {
		pthread_mutex_lock(&g_mu);
		g_wanted = 0;
		tapTeardownLocked();
		atomic_store(&g_last_untagged, 0);
		atomic_store(&g_last_disabled, 0);
		pthread_mutex_unlock(&g_mu);
	} @catch (NSException *e) {
		pthread_mutex_unlock(&g_mu);
	}
}

// *running: a tap thread exists and no disable-by-user-input is pending.
// *enabled: CGEventTapIsEnabled now. A dead tap is recreated here (the next
// poll after the disable), and a tap that could not be recreated is retried on
// every poll while one is wanted.
void pc_tap_state(int *running, int *enabled) {
	*running = 0;
	*enabled = 0;
	@try {
		pthread_mutex_lock(&g_mu);
		if (g_wanted && (!g_running || atomic_load(&g_dead))) {
			tapTeardownLocked();
			tapCreateLocked();
		}
		if (g_running && !atomic_load(&g_dead)) {
			*running = 1;
			*enabled = (g_tap != NULL && CGEventTapIsEnabled(g_tap)) ? 1 : 0;
		}
		pthread_mutex_unlock(&g_mu);
	} @catch (NSException *e) {
		pthread_mutex_unlock(&g_mu);
		*running = 0;
		*enabled = 0;
	}
}

static uint64_t machToNs(uint64_t t) {
	static mach_timebase_info_data_t tb;
	if (tb.denom == 0) mach_timebase_info(&tb);
	return t * tb.numer / tb.denom;
}

// Nanoseconds on the tap's clock (since boot, not counting sleep).
uint64_t pc_tap_now_ns(void) { return machToNs(mach_absolute_time()); }

// Time of the last untagged event on that clock; 0 when none was seen.
uint64_t pc_tap_last_untagged_ns(void) {
	return machToNs(atomic_load_explicit(&g_last_untagged, memory_order_relaxed));
}

// Time the callback last saw a disable event; 0 if never.
uint64_t pc_tap_last_disabled_ns(void) {
	return machToNs(atomic_load_explicit(&g_last_disabled, memory_order_relaxed));
}

// 1 while Secure Event Input is on: the tap then gets no key events.
int pc_secure_input_on(void) {
	@try {
		return IsSecureEventInputEnabled() ? 1 : 0;
	} @catch (NSException *e) {
		return 0;
	}
}
