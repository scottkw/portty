// input_darwin.m: posts remote pointer input and reads the display geometry it
// maps onto. Compiled by cgo with ARC on (see input_darwin.go). Every function
// returns 0 or an error code and is wrapped in @try/@catch, because an uncaught
// ObjC exception would end the daemon.
//
// Every event comes from one process-lifetime private event source, so the
// Mac's physical modifier and button state never leaks into a remote event, and
// carries a Portty marker in kCGEventSourceUserData so injected input can be
// told from real input. Events are posted at the HID tap; the cursor-warp call
// is never used (it moves the cursor without generating events).

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>

enum { kPCInputExceptionErr = -90101, kPCInputNilErr = -90102, kPCInputRangeErr = -90103 };

// ASCII "Portty"; keep equal to EventMarker in input_darwin.go.
#define kPCMarker 0x506F72747479LL

int64_t pc_event_marker(void) { return kPCMarker; }

// One private source for the life of the process, never released.
static CGEventSourceRef pcInputSource(void) {
	static CGEventSourceRef src = NULL;
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		src = CGEventSourceCreate(kCGEventSourceStatePrivate);
	});
	return src;
}

// Tags, flags and posts ev, then releases it. No sleep on this path.
static void pcPost(CGEventRef ev, uint64_t flags) {
	CGEventSetFlags(ev, (CGEventFlags)flags);
	CGEventSetIntegerValueField(ev, kCGEventSourceUserData, kPCMarker);
	CGEventPost(kCGHIDEventTap, ev);
	CFRelease(ev);
}

// kind: 0 moved, 1 down, 2 up, 3 dragged. button: 0 left, 1 right, 2 other (the
// Go side maps browser buttons). clickState applies to down and up only.
int pc_input_mouse(int kind, double x, double y, int button, int clickState, int dx, int dy, uint64_t flags) {
	@try {
		if (kind < 0 || kind > 3 || button < 0 || button > 2) return kPCInputRangeErr;
		CGEventSourceRef src = pcInputSource();
		if (src == NULL) return kPCInputNilErr;
		static const CGEventType types[4][3] = {
			{kCGEventMouseMoved, kCGEventMouseMoved, kCGEventMouseMoved},
			{kCGEventLeftMouseDown, kCGEventRightMouseDown, kCGEventOtherMouseDown},
			{kCGEventLeftMouseUp, kCGEventRightMouseUp, kCGEventOtherMouseUp},
			{kCGEventLeftMouseDragged, kCGEventRightMouseDragged, kCGEventOtherMouseDragged},
		};
		static const CGMouseButton buttons[3] = {kCGMouseButtonLeft, kCGMouseButtonRight, kCGMouseButtonCenter};
		CGEventRef ev = CGEventCreateMouseEvent(src, types[kind][button], CGPointMake(x, y), buttons[button]);
		if (ev == NULL) return kPCInputNilErr;
		CGEventSetIntegerValueField(ev, kCGMouseEventButtonNumber, buttons[button]);
		if (kind == 1 || kind == 2) CGEventSetIntegerValueField(ev, kCGMouseEventClickState, clickState);
		CGEventSetIntegerValueField(ev, kCGMouseEventDeltaX, dx);
		CGEventSetIntegerValueField(ev, kCGMouseEventDeltaY, dy);
		pcPost(ev, flags);
		return 0;
	} @catch (NSException *e) {
		return kPCInputExceptionErr;
	}
}

// The pointer's current global location, in points.
int pc_input_cursor(double *x, double *y) {
	@try {
		CGEventRef e = CGEventCreate(NULL);
		if (e == NULL) return kPCInputNilErr;
		CGPoint p = CGEventGetLocation(e);
		CFRelease(e);
		*x = p.x;
		*y = p.y;
		return 0;
	} @catch (NSException *e) {
		return kPCInputExceptionErr;
	}
}

// A display's bounds in global points, top-left origin, negative origins kept.
// id 0 is the main display.
int pc_display_bounds(uint32_t id, double *x, double *y, double *w, double *h) {
	@try {
		CGRect b = CGDisplayBounds(id == 0 ? CGMainDisplayID() : (CGDirectDisplayID)id);
		if (CGRectIsEmpty(b) || CGRectIsNull(b)) return kPCInputNilErr;
		*x = b.origin.x;
		*y = b.origin.y;
		*w = b.size.width;
		*h = b.size.height;
		return 0;
	} @catch (NSException *e) {
		return kPCInputExceptionErr;
	}
}
