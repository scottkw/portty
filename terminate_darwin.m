// terminate_darwin.m — lets a macOS logout, restart or shutdown quit Portty
// (Phase 9, UPGR-05 neighbour). Kept in its own .m file, like
// appearance_darwin.m, so ObjC symbols are compiled once by cgo.
//
// shortcut: Wails v2.10.2 internal/frontend/desktop/darwin/AppDelegate.m:26-29
// answers every applicationShouldTerminate: with NSTerminateCancel (after
// sending "Q" to the quit modal), so Portty cancelled every logout, restart and
// shutdown. This replaces that one delegate method instead of forking Wails;
// re-check AppDelegate.m on any Wails upgrade.

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

extern void porttySystemWillPowerOff(void);

static BOOL powerOffSeen = NO;
static IMP originalShouldTerminate = NULL;

// systemQuitEvent reports whether the terminate request is loginwindow's quit
// Apple event for a logout, restart or shutdown.
static BOOL systemQuitEvent(void) {
	NSAppleEventDescriptor *event = [[NSAppleEventManager sharedAppleEventManager] currentAppleEvent];
	if (event == nil || [event eventClass] != kCoreEventClass || [event eventID] != kAEQuitApplication) {
		return NO;
	}
	NSAppleEventDescriptor *reason = [event attributeDescriptorForKeyword:kAEQuitReason];
	if (reason == nil) {
		reason = [event paramDescriptorForKeyword:kAEQuitReason];
	}
	switch ([reason enumCodeValue]) {
	case kAELogOut:
	case kAEReallyLogOut:
	case kAEShowRestartDialog:
	case kAERestart:
	case kAEShowShutdownDialog:
	case kAEShutDown:
		return YES;
	default:
		return NO;
	}
}

// porttyShouldTerminate: a logout, restart or shutdown terminates now, with no
// dialog; anything else (Cmd-Q, the menu) keeps Wails' quit modal path.
static NSApplicationTerminateReply porttyShouldTerminate(id self, SEL _cmd, NSApplication *sender) {
	if (powerOffSeen || systemQuitEvent()) {
		porttySystemWillPowerOff();
		return NSTerminateNow;
	}
	return ((NSApplicationTerminateReply (*)(id, SEL, NSApplication *))originalShouldTerminate)(self, _cmd, sender);
}

// installTerminateHook replaces the delegate's applicationShouldTerminate:
// once and observes the power-off notification. dispatch_async to the main
// queue, never dispatch_sync, for the reason given in appearance_darwin.m.
void installTerminateHook(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (originalShouldTerminate != NULL) {
			return;
		}
		id delegate = [NSApp delegate];
		Method m = delegate == nil ? NULL : class_getInstanceMethod([delegate class], @selector(applicationShouldTerminate:));
		if (m == NULL) {
			return;
		}
		[[[NSWorkspace sharedWorkspace] notificationCenter] addObserverForName:NSWorkspaceWillPowerOffNotification
		                                                                object:nil
		                                                                 queue:nil
		                                                            usingBlock:^(NSNotification *note) { powerOffSeen = YES; }];
		originalShouldTerminate = method_setImplementation(m, (IMP)porttyShouldTerminate);
	});
}
