// about_darwin.m — the native AppKit About panel for App menu ▸ About Portty
// (Portty 03-04, BRND-01, BRND-06). Mirrors appearance_darwin.m; kept in its own
// .m file so ObjC symbols are compiled once by cgo.

#import <Cocoa/Cocoa.h>

// showAboutPanel opens the standard About panel. Icon, name, version and
// copyright come from the bundle (Info.plist); only the credits are supplied.
//
// Always dispatch_async to the main queue, never dispatch_sync: menu callbacks
// run off the main thread and a synchronous hop can deadlock Wails (see
// appearance_darwin.m). Compiled without ARC, so alloc'd objects are released.
void showAboutPanel(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSMutableParagraphStyle *para = [[NSMutableParagraphStyle alloc] init];
		para.alignment = NSTextAlignmentCenter;
		NSFont *font = [NSFont systemFontOfSize:[NSFont smallSystemFontSize]];

		NSMutableAttributedString *credits = [[NSMutableAttributedString alloc]
			initWithString:@"A macOS terminal with remote access over Tailscale.\n"
			    attributes:@{
				    NSFontAttributeName : font,
				    NSParagraphStyleAttributeName : para,
				    NSForegroundColorAttributeName : NSColor.secondaryLabelColor,
			    }];
		// AppKit draws NSLinkAttributeName in linkColor and opens it on click.
		NSAttributedString *link = [[NSAttributedString alloc]
			initWithString:@"portty.app"
			    attributes:@{
				    NSFontAttributeName : font,
				    NSParagraphStyleAttributeName : para,
				    NSLinkAttributeName : [NSURL URLWithString:@"https://portty.app"],
			    }];
		[credits appendAttributedString:link];
		[link release];
		[para release];

		[NSApp activateIgnoringOtherApps:YES];
		// Empty build version: no "(build)" after "Version x.y.z".
		[NSApp orderFrontStandardAboutPanelWithOptions:@{
			NSAboutPanelOptionCredits : credits,
			NSAboutPanelOptionVersion : @"",
		}];
		[credits release];
	});
}
