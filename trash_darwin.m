// trash_darwin.m — moves a path to the user's Trash (Phase 18, UPD-05/UPD-06).
// Kept in its own .m file, like terminate_darwin.m, so ObjC symbols are
// compiled once by cgo.
//
// NSFileManager trashItemAtURL, not a Finder AppleEvent: no Automation prompt
// on an unattended Mac, and the item stays recoverable by hand from the Trash
// (CONTEXT: the old Portty.app goes to the Trash, never deleted). It needs only
// Foundation, so the installer process, which never starts NSApplication, can
// call it synchronously.

#import <Foundation/Foundation.h>
#include <string.h>

// porttyTrashItem moves path to the Trash. 0 on success; otherwise 1 with the
// error's description in errbuf (always NUL-terminated).
int porttyTrashItem(const char *path, char *errbuf, int errlen) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSError *err = nil;
		if ([[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:nil error:&err]) {
			return 0;
		}
		const char *msg = err == nil ? "trash failed" : [[err localizedDescription] UTF8String];
		if (errlen > 0) {
			strlcpy(errbuf, msg == NULL ? "trash failed" : msg, (size_t)errlen);
		}
		return 1;
	}
}
