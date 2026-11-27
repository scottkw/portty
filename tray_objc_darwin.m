// tray_objc.m — Objective-C implementation for macOS system tray.
// Compiled once by cgo; keeps ObjC class symbols out of Go's per-file
// compilation units, avoiding duplicate-symbol linker errors.
//
// Build constraint: this file is only compiled on Darwin via cgo tags in
// tray.go (cgo CFLAGS: -x objective-c applies only to .go files with the
// darwin build tag; .m files are always compiled as ObjC by cgo on macOS).

#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>

// Forward-declare Go callbacks so they can be invoked from ObjC.
extern void onTrayShow(void);
extern void onTrayQuit(void);
extern void onTraySession(int idx);
extern void onTrayUpdate(void);
extern void onTrayShareDesktop(void);
extern void onTrayStopSharing(void);
extern void onNotificationClick(char *identifier);

// Forward-declare hasValidBundleIdentifier (defined below, near sendNotification)
// so registerNotificationDelegate, which comes before that definition, can call it.
int hasValidBundleIdentifier(void);

NSStatusItem *statusItem = nil;
NSMutableArray *menuSessionNames = nil;
NSMutableArray *menuSessionIDs = nil;
BOOL daemonConnected = YES;
NSString *trayUpdateTitle = nil; // "Update Available…" while a newer version is known; nil or empty hides the item
NSString *trayDesktopTitle = nil; // "Desktop: ..." line while always-available is on; nil or empty hides it
BOOL trayShareActive = NO; // a click-to-share is active (Phase 22): the shared-with line and Stop Sharing replace Share Desktop…
NSString *trayShareTitle = nil; // "Desktop shared with N people" while trayShareActive

// PorttyMenuDelegate rebuilds the menu on every open so it always reflects
// the current session list without a separate polling update path.
@interface PorttyMenuDelegate : NSObject <NSMenuDelegate>
@end

@implementation PorttyMenuDelegate
- (void)menuWillOpen:(NSMenu *)menu {
    [menu removeAllItems];

    NSMenuItem *openItem = [[NSMenuItem alloc]
        initWithTitle:@"Open Portty"
        action:@selector(showClicked:)
        keyEquivalent:@""];
    openItem.target = statusItem;
    [menu addItem:openItem];

    // The share slot sits directly under Open Portty: Share Desktop… while no
    // click-to-share is active, otherwise the shared-with line (enabled, it
    // opens the sheet) and Stop Sharing directly under it.
    if (trayShareActive) {
        NSMenuItem *sharedItem = [[NSMenuItem alloc]
            initWithTitle:(trayShareTitle.length > 0 ? trayShareTitle : @"Desktop shared")
            action:@selector(shareDesktopClicked:)
            keyEquivalent:@""];
        sharedItem.target = statusItem;
        [menu addItem:sharedItem];
        [sharedItem release]; // MRC: the menu retains it
        NSMenuItem *stopItem = [[NSMenuItem alloc]
            initWithTitle:@"Stop Sharing"
            action:@selector(stopShareClicked:)
            keyEquivalent:@""];
        stopItem.target = statusItem;
        [menu addItem:stopItem];
        [stopItem release]; // MRC: the menu retains it
    } else {
        NSMenuItem *shareItem = [[NSMenuItem alloc]
            initWithTitle:@"Share Desktop…"
            action:@selector(shareDesktopClicked:)
            keyEquivalent:@""];
        shareItem.target = statusItem;
        [menu addItem:shareItem];
        [shareItem release]; // MRC: the menu retains it
    }

    if (trayUpdateTitle.length > 0) {
        NSMenuItem *updateItem = [[NSMenuItem alloc]
            initWithTitle:trayUpdateTitle
            action:@selector(updateClicked:)
            keyEquivalent:@""];
        updateItem.target = statusItem;
        [menu addItem:updateItem];
        [updateItem release]; // MRC: the menu retains it
    }

    if (menuSessionNames != nil && menuSessionNames.count > 0) {
        [menu addItem:[NSMenuItem separatorItem]];
        for (NSUInteger i = 0; i < menuSessionNames.count; i++) {
            NSMenuItem *item = [[NSMenuItem alloc]
                initWithTitle:menuSessionNames[i]
                action:@selector(sessionClicked:)
                keyEquivalent:@""];
            item.target = statusItem;
            item.tag = (NSInteger)i;
            [menu addItem:item];
        }
    }

    if (trayDesktopTitle.length > 0) {
        // Inert state line: no target or action, disabled so it reads dimmed.
        [menu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *desktopItem = [[NSMenuItem alloc]
            initWithTitle:trayDesktopTitle
            action:NULL
            keyEquivalent:@""];
        desktopItem.enabled = NO;
        [menu addItem:desktopItem];
        [desktopItem release]; // MRC: the menu retains it
    }

    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *quitItem = [[NSMenuItem alloc]
        initWithTitle:@"Quit Portty"
        action:@selector(quitClicked:)
        keyEquivalent:@""];
    quitItem.target = statusItem;
    [menu addItem:quitItem];
}
@end

static PorttyMenuDelegate *menuDelegate = nil;

// PorttyNotificationDelegate presents notifications while Portty is the
// frontmost app (Phase 167-06, M-41 gap closure). Without a registered
// UNUserNotificationCenterDelegate, UNUserNotificationCenter suppresses
// banners/sounds for the foreground app by default, silently swallowing the
// notification even when authorization was granted and delivery succeeded.
// It also routes a click on a notification body to Go (onNotificationClick).
@interface PorttyNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation PorttyNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
        willPresentNotification:(UNNotification *)notification
          withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner |
                       UNNotificationPresentationOptionList |
                       UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
 didReceiveNotificationResponse:(UNNotificationResponse *)response
          withCompletionHandler:(void (^)(void))completionHandler {
    @autoreleasepool {
        if ([response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier]) {
            onNotificationClick((char *)[response.notification.request.identifier UTF8String]);
        }
    }
    completionHandler(); // always, or the system waits on us
}
@end

static PorttyNotificationDelegate *notificationDelegate = nil;

// registerNotificationDelegate installs the foreground-presentation delegate
// on UNUserNotificationCenter. Idempotent (safe to call repeatedly) and
// bundle-guarded — mirrors the 167-05 fail-safe pattern so an unbundled
// process (wails dev / go test) log-and-swallows instead of raising an
// uncaught NSInternalInconsistencyException.
void registerNotificationDelegate(void) {
    if (!hasValidBundleIdentifier()) {
        NSLog(@"Portty: skipping notification delegate registration — no valid app-bundle identifier (unbundled/unsigned process)");
        return;
    }
    @try {
        if (notificationDelegate == nil) {
            notificationDelegate = [[PorttyNotificationDelegate alloc] init];
        }
        [UNUserNotificationCenter currentNotificationCenter].delegate = notificationDelegate;
    } @catch (NSException *e) {
        NSLog(@"Portty: swallowed exception registering notification delegate: %@", e);
    }
}

// makeTrayImage builds one 18×18 pt template image from the 18 px (d1) and
// 36 px (d2) PNGs. Both reps are sized 18×18 pt so AppKit picks the 36 px rep
// on Retina displays. Returns an autoreleased image.
static NSImage *makeTrayImage(NSData *d1, NSData *d2) {
    NSImage *icon = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
    for (NSData *d in @[d1, d2]) {
        NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithData:d];
        [rep setSize:NSMakeSize(18, 18)];
        [icon addRepresentation:rep];
        [rep release];
    }
    [icon setTemplate:YES];  // Adapts to light/dark menu bar.
    return [icon autorelease];
}

// Entry points called from Go wrap their bodies in @autoreleasepool: cgo
// threads have no pool that ever drains, so autoreleased copies would leak on
// every 5 s tray poll. The copied dispatch block retains what it captures.

// initStatusItem creates a macOS status bar item with a dynamic menu delegate.
void initStatusItem(const void *d1, int n1, const void *d2, int n2) {
    @autoreleasepool {
        // Copy icon data synchronously — d1/d2 are Go pointers only valid during
        // the cgo call. The dispatch_async block runs later when Go may have reclaimed them.
        NSData *data1 = [NSData dataWithBytes:d1 length:n1];
        NSData *data2 = [NSData dataWithBytes:d2 length:n2];
        dispatch_async(dispatch_get_main_queue(), ^{
            NSStatusBar *bar = [NSStatusBar systemStatusBar];
            statusItem = [bar statusItemWithLength:NSVariableStatusItemLength];
            [statusItem retain];

            // Set icon from the 1x and @2x PNG data.
            statusItem.button.image = makeTrayImage(data1, data2);
            statusItem.button.toolTip = @"Portty";

            // Build menu with delegate for dynamic rebuilding.
            NSMenu *menu = [[NSMenu alloc] init];
            menuDelegate = [[PorttyMenuDelegate alloc] init];
            menu.delegate = menuDelegate;
            statusItem.menu = menu;

            // Hide Dock icon — Wails overrides LSUIElement by setting its own
            // activation policy, so we must set accessory mode programmatically.
            [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

            // Register at launch so a click on a notification from an earlier run
            // reaches the delegate; idempotent and bundle-guarded.
            registerNotificationDelegate();
        });
    }
}

// removeStatusItem tears down the status bar item.
void removeStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
            [statusItem release];
            statusItem = nil;
        }
    });
}

// updateTrayIcon swaps the tray icon PNG at runtime (normal vs error state).
void updateTrayIcon(const void *d1, int n1, const void *d2, int n2) {
    @autoreleasepool {
        // Copy before dispatch — d1/d2 are Go pointers only valid during cgo call.
        NSData *data1 = [NSData dataWithBytes:d1 length:n1];
        NSData *data2 = [NSData dataWithBytes:d2 length:n2];
        dispatch_async(dispatch_get_main_queue(), ^{
            if (statusItem == nil) return;
            statusItem.button.image = makeTrayImage(data1, data2);
        });
    }
}

// updateTrayTooltip updates the hover tooltip of the tray icon.
void updateTrayTooltip(const char *tooltip) {
    @autoreleasepool {
        NSString *tip = [NSString stringWithUTF8String:tooltip];
        dispatch_async(dispatch_get_main_queue(), ^{
            if (statusItem != nil) {
                statusItem.button.toolTip = tip;
            }
        });
    }
}

// setDockVisible shows or hides the macOS Dock icon by toggling the
// application activation policy between Regular (visible) and Accessory (hidden).
// When showing, the app is also activated so the window comes forward.
void setDockVisible(int visible) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (visible) {
            BOOL wasAccessory = NSApp.activationPolicy != NSApplicationActivationPolicyRegular;
            [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
            if (wasAccessory) {
                // AppKit leaves the menu bar unresponsive after an Accessory→Regular
                // switch until the app is deactivated and reactivated. Hand focus to
                // the Dock briefly, then take it back.
                NSRunningApplication *dock = [[NSRunningApplication
                    runningApplicationsWithBundleIdentifier:@"com.apple.dock"] firstObject];
                [dock activateWithOptions:0];
                dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 150 * NSEC_PER_MSEC),
                               dispatch_get_main_queue(), ^{
                    [NSApp activateIgnoringOtherApps:YES];
                });
            } else {
                [NSApp activateIgnoringOtherApps:YES];
            }
        } else {
            [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        }
    });
}

// setTraySessionData updates the session names/IDs used by the menu delegate.
void setTraySessionData(const char **names, const char **ids, int count) {
    @autoreleasepool {
        // Copy strings synchronously — names/ids are Go pointers freed after cgo returns.
        NSMutableArray *nameArr = [NSMutableArray arrayWithCapacity:count];
        NSMutableArray *idArr = [NSMutableArray arrayWithCapacity:count];
        for (int i = 0; i < count; i++) {
            [nameArr addObject:[NSString stringWithUTF8String:names[i]]];
            [idArr addObject:[NSString stringWithUTF8String:ids[i]]];
        }
        dispatch_async(dispatch_get_main_queue(), ^{
            // MRC: the globals own their arrays; release the previous pair.
            [menuSessionNames release];
            [menuSessionIDs release];
            menuSessionNames = [nameArr retain];
            menuSessionIDs = [idArr retain];
        });
    }
}

// setTrayUpdateItem sets the update item's title for the menu delegate; "" hides it.
void setTrayUpdateItem(const char *title) {
    @autoreleasepool {
        // Copy synchronously — title is a Go pointer freed after cgo returns.
        NSString *t = [NSString stringWithUTF8String:title];
        dispatch_async(dispatch_get_main_queue(), ^{
            // MRC: the global owns its string; release the previous one.
            [trayUpdateTitle release];
            trayUpdateTitle = [t retain];
        });
    }
}

// setTrayDesktopTitle sets the desktop line for the menu delegate; "" hides it.
void setTrayDesktopTitle(const char *title) {
    @autoreleasepool {
        // Copy synchronously — title is a Go pointer freed after cgo returns.
        NSString *t = [NSString stringWithUTF8String:title];
        dispatch_async(dispatch_get_main_queue(), ^{
            [trayDesktopTitle release];
            trayDesktopTitle = [t retain];
        });
    }
}

// setTrayShareState sets the click-to-share slot for the menu delegate: active
// swaps Share Desktop… for the shared-with line (title) and Stop Sharing.
void setTrayShareState(int active, const char *title) {
    @autoreleasepool {
        // Copy synchronously — title is a Go pointer freed after cgo returns.
        NSString *t = [NSString stringWithUTF8String:title];
        dispatch_async(dispatch_get_main_queue(), ^{
            trayShareActive = active != 0;
            [trayShareTitle release];
            trayShareTitle = [t retain];
        });
    }
}

// hasValidBundleIdentifier reports whether the running process has a valid
// app-bundle identifier. UNUserNotificationCenter requires one; an unbundled
// process (e.g. `wails dev`, a `go test` binary) has none, and calling into
// UNUserNotificationCenter without this guard raises an uncaught
// NSInternalInconsistencyException ("bundleProxyForCurrentProcess is nil")
// that aborts the whole process (Phase 167 M-41 regression).
int hasValidBundleIdentifier(void) {
    return [[NSBundle mainBundle] bundleIdentifier] != nil ? 1 : 0;
}

// notificationUsesMainQueue is true only while an NSApplication is running, i.e.
// in the GUI: only then is the main dispatch queue serviced. The headless
// launchd daemon (a Go main thread, no run loop) never services it, so a block
// queued there never ran and its lapse notification vanished (VM row V1).
int notificationUsesMainQueue(void) {
    return [NSApp isRunning] ? 1 : 0;
}

// runNotificationBlock queues a notification step where it can run: the main
// queue in the GUI (unchanged), a private serial queue everywhere else.
// UNUserNotificationCenter is not main-thread-only.
static void runNotificationBlock(dispatch_block_t block) {
    if (notificationUsesMainQueue()) {
        dispatch_async(dispatch_get_main_queue(), block);
        return;
    }
    static dispatch_queue_t q;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ q = dispatch_queue_create("app.portty.notifications", DISPATCH_QUEUE_SERIAL); });
    dispatch_async(q, block);
}

// sendNotification sends a macOS notification using UNUserNotificationCenter.
// Requests permission lazily on first call; no-ops if the user denies.
// The identifier is caller-supplied so concurrent notifications (e.g. two
// sessions transitioning to waiting close together) do not collide and
// silently replace one another (RESEARCH Pitfall 2).
//
// Fail-safe (Phase 167-05 gap closure, M-41): if the process has no valid
// app-bundle identifier, UNUserNotificationCenter is unusable and calling it
// aborts the process. Guard synchronously BEFORE dispatch so the crash-prone
// API is never reached in that case — log-and-swallow, mirroring the beeep
// Windows/Linux wrappers' contract. On a properly bundled/signed .app this
// guard passes and the notification fires exactly as before.
void sendNotification(const char *identifier, const char *title, const char *body) {
    if (!hasValidBundleIdentifier()) {
        NSLog(@"Portty: skipping native notification — no valid app-bundle identifier (unbundled/unsigned process)");
        return;
    }
    NSString *nsIdentifier = [NSString stringWithUTF8String:identifier];
    NSString *nsTitle = [NSString stringWithUTF8String:title];
    NSString *nsBody  = [NSString stringWithUTF8String:body];
    runNotificationBlock(^{
        NSLog(@"Portty: dispatching native notification for identifier %@ (main queue: %d)", nsIdentifier, notificationUsesMainQueue());
        registerNotificationDelegate();
        @try {
            UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
            [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                completionHandler:^(BOOL granted, NSError *error) {
                if (!granted) {
                    NSLog(@"Portty: notification authorization NOT granted for identifier %@ (error=%@)", nsIdentifier, error);
                    return;
                }
                runNotificationBlock(^{
                    @try {
                        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
                        content.title = nsTitle;
                        content.body  = nsBody;
                        // Phase 167 (M-41 UAT follow-up): attach the default sound so the
                        // alert is audible even when the banner auto-dismisses within a few
                        // seconds. macOS cannot extend banner duration app-side; a sound is
                        // the entitlement-free way to make "your session needs input"
                        // noticeable. The foreground delegate already returns
                        // UNNotificationPresentationOptionSound so this also plays when
                        // Portty is frontmost. Persistence-until-dismissed remains the
                        // user's Alert-style choice (System Settings → Notifications).
                        content.sound = [UNNotificationSound defaultSound];
                        UNNotificationRequest *req = [UNNotificationRequest
                            requestWithIdentifier:nsIdentifier
                            content:content
                            trigger:nil];
                        [center addNotificationRequest:req withCompletionHandler:^(NSError * _Nullable error) {
                            if (error != nil) {
                                NSLog(@"Portty: notification delivery FAILED for identifier %@: %@", nsIdentifier, error);
                            } else {
                                NSLog(@"Portty: notification delivery accepted for identifier %@", nsIdentifier);
                            }
                        }];
                    } @catch (NSException *e) {
                        NSLog(@"Portty: swallowed exception adding notification request: %@", e);
                    }
                });
            }];
        } @catch (NSException *e) {
            NSLog(@"Portty: swallowed exception requesting notification authorization: %@", e);
        }
    });
}

// Action handlers — forward to Go callbacks.
@interface NSStatusItem (PorttyTray)
- (void)showClicked:(id)sender;
- (void)quitClicked:(id)sender;
- (void)sessionClicked:(NSMenuItem *)sender;
- (void)updateClicked:(id)sender;
- (void)shareDesktopClicked:(id)sender;
- (void)stopShareClicked:(id)sender;
@end

@implementation NSStatusItem (PorttyTray)
- (void)showClicked:(id)sender { onTrayShow(); }
- (void)quitClicked:(id)sender { onTrayQuit(); }
- (void)sessionClicked:(NSMenuItem *)sender { onTraySession((int)sender.tag); }
- (void)updateClicked:(id)sender { onTrayUpdate(); }
- (void)shareDesktopClicked:(id)sender { onTrayShareDesktop(); }
- (void)stopShareClicked:(id)sender { onTrayStopSharing(); }
@end
