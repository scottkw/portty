// hostrun_darwin.m: the AppKit and process-accounting side of `spike host-run`.
// Compiled by cgo with ARC on (see bridge_darwin.go). The test chart is a
// full-screen borderless window above the menu bar, so every frame ScreenCaptureKit
// captures during the run is the chart and never the person's desktop. All
// AppKit work is queued to the main queue; Go runs the NSApplication loop on the
// process's main thread through spk_app_run.

#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>
#import <libproc.h>
#import <mach/mach_time.h>
#import <unistd.h>

static NSWindow *gChartWin;
static NSTimer *gMotionTimer;
static BOOL gMotionWanted;

@interface SpkChartView : NSView
@end

@implementation SpkChartView

- (BOOL)isFlipped {
	return YES;
}

- (BOOL)isOpaque {
	return YES;
}

// The right-hand third of the top of the screen: the only part the 60 Hz timer redraws.
- (NSRect)motionRect {
	NSRect b = self.bounds;
	CGFloat w = NSWidth(b) * 0.30;
	return NSMakeRect(NSMaxX(b) - w - 20, 20, w, NSHeight(b) * 0.55);
}

// A nil font or color would throw in the @{} literal inside the CA commit, where AppKit
// traps (+[NSApplication _crashOnException:], SIGTRAP) instead of reporting: the 17-11
// host run died that way. Skip the text and say so once on stderr.
- (void)drawText:(NSString *)s at:(NSPoint)p font:(NSFont *)f color:(NSColor *)c {
	if (f == nil || c == nil) {
		static BOOL logged;
		if (!logged) fprintf(stderr, "spike: chart text skipped, nil %s\n", f == nil ? "font" : "color");
		logged = YES;
		return;
	}
	[s drawAtPoint:p withAttributes:@{NSFontAttributeName : f, NSForegroundColorAttributeName : c}];
}

// Sample text at 9, 11, 13, 16 and 20 pt, black on white and white on black.
- (void)drawPanel:(NSRect)r fg:(NSColor *)fg bg:(NSColor *)bg {
	[NSGraphicsContext saveGraphicsState];
	[NSBezierPath clipRect:r];
	[bg setFill];
	NSRectFill(r);
	CGFloat y = NSMinY(r) + 8;
	for (NSNumber *size in @[ @9, @11, @13, @16, @20 ]) {
		CGFloat pt = size.doubleValue;
		NSFont *prop = [NSFont systemFontOfSize:pt];
		NSFont *mono = [NSFont monospacedSystemFontOfSize:pt weight:NSFontWeightRegular];
		for (int line = 0; line < 2; line++) {
			NSString *t = line == 0 ? [NSString stringWithFormat:@"%.0f pt  The quick brown fox jumps over the lazy dog 0123456789", pt]
			                        : [NSString stringWithFormat:@"%.0f pt  func main() { x := a[i] + b[j]; } // 1Il0O {}[]<>/\\|", pt];
			[self drawText:t at:NSMakePoint(NSMinX(r) + 8, y) font:line == 0 ? prop : mono color:fg];
			y += pt * 1.5;
		}
	}
	[NSGraphicsContext restoreGraphicsState];
}

- (void)drawStatic {
	[[NSColor whiteColor] setFill];
	NSRectFill(self.bounds);
	NSRect b = self.bounds;
	CGFloat panelW = NSWidth(b) * 0.30;
	CGFloat panelH = 250; // the five sizes, two lines each
	[self drawPanel:NSMakeRect(20, 20, panelW, panelH) fg:[NSColor blackColor] bg:[NSColor whiteColor]];
	[self drawPanel:NSMakeRect(20 + panelW + 20, 20, panelW, panelH) fg:[NSColor whiteColor] bg:[NSColor blackColor]];
	CGFloat bottom = 20 + panelH;

	// A one-pixel line grid, every 20 points, lines aligned to device pixels.
	CGFloat px = 1.0 / (self.window.backingScaleFactor > 0 ? self.window.backingScaleFactor : 1.0);
	NSRect grid = NSMakeRect(20, bottom + 20, 400, 200);
	[[NSColor blackColor] setFill];
	for (CGFloat x = 0; x <= NSWidth(grid); x += 20) NSRectFill(NSMakeRect(NSMinX(grid) + x, NSMinY(grid), px, NSHeight(grid)));
	for (CGFloat y = 0; y <= NSHeight(grid); y += 20) NSRectFill(NSMakeRect(NSMinX(grid), NSMinY(grid) + y, NSWidth(grid), px));

	// A gray ramp of 16 steps.
	NSRect ramp = NSMakeRect(NSMaxX(grid) + 40, NSMinY(grid), 400, 60);
	for (int i = 0; i < 16; i++) {
		[[NSColor colorWithCalibratedWhite:i / 15.0 alpha:1.0] setFill];
		NSRectFill(NSMakeRect(NSMinX(ramp) + i * NSWidth(ramp) / 16, NSMinY(ramp), NSWidth(ramp) / 16 + 1, NSHeight(ramp)));
	}
	[self drawText:@"PORTTY SPIKE TEST CHART - this window covers the display while the spike runs"
	            at:NSMakePoint(NSMinX(grid), NSMaxY(grid) + 20)
	          font:[NSFont boldSystemFontOfSize:16]
	         color:[NSColor blackColor]];
}

// A scrolling text column and a bouncing square, both a function of the clock.
- (void)drawMotion:(NSRect)m {
	[NSGraphicsContext saveGraphicsState];
	[NSBezierPath clipRect:m];
	[[NSColor colorWithCalibratedWhite:0.92 alpha:1.0] setFill];
	NSRectFill(m);
	double t = [NSProcessInfo processInfo].systemUptime;
	CGFloat lineH = 18;
	long first = (long)(t * 120 / lineH);
	CGFloat off = fmod(t * 120, lineH);
	static NSFont *f; // made once and held, not looked up 60 times a second; retried while nil
	if (f == nil) f = [NSFont monospacedSystemFontOfSize:13 weight:NSFontWeightRegular];
	for (int i = 0; i < (int)(NSHeight(m) / lineH) + 2; i++) {
		NSString *s = [NSString stringWithFormat:@"line %06ld  scrolling text column", first + i];
		[self drawText:s at:NSMakePoint(NSMinX(m) + 8, NSMinY(m) + i * lineH - off) font:f color:[NSColor blackColor]];
	}
	CGFloat side = 80, span = NSWidth(m) - side;
	double phase = fmod(t * 0.6, 2.0);
	CGFloat x = NSMinX(m) + span * (phase < 1 ? phase : 2 - phase);
	[[NSColor redColor] setFill];
	NSRectFill(NSMakeRect(x, NSMaxY(m) - side - 20, side, side));
	[NSGraphicsContext restoreGraphicsState];
}

- (void)drawRect:(NSRect)dirty {
	NSRect m = [self motionRect];
	// AppKit rounds a dirty rect out to whole pixels, so allow a little slack.
	if (!NSContainsRect(NSInsetRect(m, -2, -2), dirty)) [self drawStatic];
	if (NSIntersectsRect(m, dirty)) [self drawMotion:m];
}

@end

static void motionApply(void) {
	NSView *v = gChartWin.contentView;
	if (gMotionWanted && v != nil && gMotionTimer == nil) {
		gMotionTimer = [NSTimer timerWithTimeInterval:1.0 / 60.0
		                                      repeats:YES
		                                        block:^(NSTimer *t) {
			                                        [v setNeedsDisplayInRect:[(SpkChartView *)v motionRect]];
		                                        }];
		[[NSRunLoop mainRunLoop] addTimer:gMotionTimer forMode:NSRunLoopCommonModes];
	} else if (!gMotionWanted && gMotionTimer != nil) {
		[gMotionTimer invalidate];
		gMotionTimer = nil;
	}
}

// Shows or removes the chart window over the primary display (the one with the
// menu bar, which is the display ScreenCaptureKit captures).
void spk_chart_show(int show) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApplication sharedApplication];
		if (show) {
			if (gChartWin == nil) {
				NSRect frame = [NSScreen screens].firstObject.frame;
				gChartWin = [[NSWindow alloc] initWithContentRect:frame
				                                        styleMask:NSWindowStyleMaskBorderless
				                                          backing:NSBackingStoreBuffered
				                                            defer:NO];
				gChartWin.releasedWhenClosed = NO;
				gChartWin.level = NSScreenSaverWindowLevel;
				gChartWin.opaque = YES;
				gChartWin.hasShadow = NO;
				gChartWin.ignoresMouseEvents = YES;
				gChartWin.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary |
				                               NSWindowCollectionBehaviorIgnoresCycle;
				gChartWin.contentView = [[SpkChartView alloc] initWithFrame:NSMakeRect(0, 0, NSWidth(frame), NSHeight(frame))];
			}
			[gChartWin orderFrontRegardless];
			[gChartWin.contentView setNeedsDisplay:YES];
			motionApply();
		} else if (gChartWin != nil) {
			gMotionWanted = NO;
			motionApply();
			[gChartWin orderOut:nil];
			gChartWin = nil;
		}
	});
}

void spk_chart_motion(int on) {
	dispatch_async(dispatch_get_main_queue(), ^{
		gMotionWanted = on != 0;
		motionApply();
	});
}

// Runs NSApplication on the calling thread, which Go has locked to the main
// thread, until spk_app_stop. The accessory policy keeps the spike out of the Dock.
void spk_app_run(void) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		[NSApp run];
	}
}

void spk_app_stop(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp stop:nil];
		// stop: only takes effect after the next event, so post one.
		NSEvent *e = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
		                                location:NSZeroPoint
		                           modifierFlags:0
		                               timestamp:0
		                            windowNumber:0
		                                 context:nil
		                                 subtype:0
		                                   data1:0
		                                   data2:0];
		[NSApp postEvent:e atStart:YES];
	});
}

// 1 when the primary display is asleep.
int spk_display_asleep(void) {
	return CGDisplayIsAsleep(CGMainDisplayID()) ? 1 : 0;
}

// This process's user and system CPU time and resident size, from
// proc_pid_rusage. The times are mach ticks, not nanoseconds: the caller
// converts them with spk_mach_timebase. 0 on success.
int spk_rusage_ticks(uint64_t *user, uint64_t *sys, uint64_t *rss) {
	struct rusage_info_v4 ri;
	if (proc_pid_rusage(getpid(), RUSAGE_INFO_V4, (rusage_info_t *)&ri) != 0) return -1;
	*user = ri.ri_user_time;
	*sys = ri.ri_system_time;
	*rss = ri.ri_resident_size;
	return 0;
}

void spk_mach_timebase(uint32_t *numer, uint32_t *denom) {
	mach_timebase_info_data_t tb;
	mach_timebase_info(&tb);
	*numer = tb.numer;
	*denom = tb.denom;
}
