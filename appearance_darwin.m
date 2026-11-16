// appearance_darwin.m — native appearance bridges: NSApp.appearance for the
// Settings ▸ Appearance override (Portty 02-13, THEM-01), and the Sidebar
// material on the translucent window's effect view (quick 261115-rc8, LOOK-01).
// Kept in its own .m file, like tray_objc_darwin.m, so ObjC symbols are compiled
// once by cgo.

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

// setSidebarMaterial gives the NSVisualEffectView that Wails v2.10.2 adds for
// WindowIsTranslucent (WailsContext.m CreateWindow, default material) the native
// Sidebar material. dispatch_async for the same reason as setAppAppearance above.
// No @available guard: the Sidebar material exists since 10.11 (target is 10.13).
// Non-ARC, nothing allocated, so no retain/release.
void setSidebarMaterial(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		Class wailsWindow = NSClassFromString(@"WailsWindow");
		int updated = 0;
		for (NSWindow *window in NSApp.windows) {
			// Skips the tray's status-bar window and any sheet.
			if (wailsWindow == nil || ![window isKindOfClass:wailsWindow]) {
				continue;
			}
			for (NSView *view in window.contentView.subviews) {
				if (![view isKindOfClass:[NSVisualEffectView class]]) {
					continue;
				}
				NSVisualEffectView *effect = (NSVisualEffectView *)view;
				effect.material = NSVisualEffectMaterialSidebar;
				effect.blendingMode = NSVisualEffectBlendingModeBehindWindow;
				// Like Finder/Mail: vibrant while key, flat when another app is
				// frontmost (TESTING.md M-61 step 4). NSVisualEffectStateActive
				// keeps it vibrant always.
				effect.state = NSVisualEffectStateFollowsWindowActiveState;
				updated++;
			}
		}
		if (updated == 0) {
			NSLog(@"portty: setSidebarMaterial found no NSVisualEffectView under a WailsWindow");
		}
	});
}
