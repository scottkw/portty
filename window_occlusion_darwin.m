// window_occlusion_darwin.m — tells the Go side when the main window becomes fully covered or
// uncovered (Portty 21-08, T-21-26). The web view's document.visibilityState stays "visible"
// for a window that is only covered (21-02 probe K4), so a covered Desktop tab would keep the
// office Mac's display awake without this. Kept in its own .m file, like appearance_darwin.m.

#import <Cocoa/Cocoa.h>

// Implemented in window_occlusion_darwin.go (//export).
extern void porttyWindowOcclusionChanged(int hidden);

// installWindowOcclusionObserver observes NSWindowDidChangeOcclusionStateNotification for the Wails
// window and reports hidden = 1 when no part of it is visible. dispatch_async to the main queue, never
// dispatch_sync, for the reason given in appearance_darwin.m. Installed once; later calls do nothing.
void installWindowOcclusionObserver(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		static BOOL installed = NO;
		if (installed) {
			return;
		}
		installed = YES;
		Class wailsWindow = NSClassFromString(@"WailsWindow");
		[[NSNotificationCenter defaultCenter]
			addObserverForName:NSWindowDidChangeOcclusionStateNotification
			object:nil
			queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *note) {
				NSWindow *window = note.object;
				// Skips the tray's status-bar window and any sheet.
				if (wailsWindow == nil || ![window isKindOfClass:wailsWindow]) {
					return;
				}
				BOOL visible = (window.occlusionState & NSWindowOcclusionStateVisible) != 0;
				porttyWindowOcclusionChanged(visible ? 0 : 1);
			}];
	});
}
