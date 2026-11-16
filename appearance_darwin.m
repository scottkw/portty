// appearance_darwin.m — NSApp.appearance for the Settings ▸ Appearance override
// (Portty 02-13, THEM-01). Kept in its own .m file, like tray_objc_darwin.m, so
// ObjC symbols are compiled once by cgo.

#import <Cocoa/Cocoa.h>

// setAppAppearance: 0 = follow the system (nil), 1 = Aqua (light), 2 = DarkAqua.
// Windows and views without their own appearance (ours: macWindowOptions leaves
// Appearance unset) inherit it, including the sidebar vibrancy and WKWebView.
//
// Always dispatch_async to the main queue, never dispatch_sync: Wails runs
// OnStartup on a goroutine while the main thread runs the AppKit loop, and the
// main thread can block on a full Wails message channel whose consumer waits on
// startup, so a synchronous hop could deadlock. The main queue is serial, so a
// later runtime.WindowShow (itself dispatch_async) still runs after this.
void setAppAppearance(int mode) {
	dispatch_async(dispatch_get_main_queue(), ^{
		// The build's deployment target is 10.13; NSApp.appearance needs 10.14.
		if (@available(macOS 10.14, *)) {
			NSAppearance *appearance = nil;
			if (mode == 1) {
				appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
			} else if (mode == 2) {
				appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
			}
			NSApp.appearance = appearance;
		}
	});
}
