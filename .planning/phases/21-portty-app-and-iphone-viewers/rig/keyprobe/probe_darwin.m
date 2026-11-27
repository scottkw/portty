// THROWAWAY Phase 21-02 key-routing probe (scratch copy only, never the shipped tree).
// probeBlankMenuKeys(1) clears every NSMenuItem key equivalent in NSApp.mainMenu, remembering
// (item, key, modifier mask); probeBlankMenuKeys(0) puts them back. Idempotent both ways.
#import <Cocoa/Cocoa.h>

static NSMutableArray *saved = nil;

static void blankMenu(NSMenu *menu) {
	for (NSMenuItem *item in [menu itemArray]) {
		if ([[item keyEquivalent] length] > 0) {
			[saved addObject:@[item, [item keyEquivalent], @([item keyEquivalentModifierMask])]];
			[item setKeyEquivalent:@""];
		}
		if ([item submenu] != nil) {
			blankMenu([item submenu]);
		}
	}
}

static void applyBlank(int on) {
	if (saved == nil) {
		saved = [NSMutableArray array];
	}
	if (on) {
		if ([saved count] == 0) {
			blankMenu([NSApp mainMenu]);
		}
		return;
	}
	for (NSArray *e in saved) {
		NSMenuItem *item = e[0];
		[item setKeyEquivalent:e[1]];
		[item setKeyEquivalentModifierMask:[e[2] unsignedIntegerValue]];
	}
	[saved removeAllObjects];
}

void probeBlankMenuKeys(int on) {
	if ([NSThread isMainThread]) {
		applyBlank(on);
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{ applyBlank(on); });
	}
}
