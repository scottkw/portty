// pasteboard_darwin.m: the Mac's general pasteboard, read and written only when
// a viewer asks (see internal/desktop/clipboard.go). Compiled by cgo with ARC
// on (media_darwin.go sets the flags). Every function returns 0 or an error
// code and is wrapped in @try/@catch, because an uncaught ObjC exception would
// end the daemon.
//
// A read looks at the item types first and answers "concealed" without reading
// any content when a password manager or a transient flag marks the item.
// Nothing here looks at the pasteboard's change counter: there is no polling
// and no background sync.

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

enum { kPCPbExceptionErr = -90301, kPCPbNilErr = -90302, kPCPbWriteErr = -90303 };

// Outcomes of pc_pb_read, mirrored by ReadOutcome in pasteboard_darwin.go.
enum { kPCPbOK = 0, kPCPbConcealed = 1, kPCPbEmpty = 2, kPCPbTooLarge = 3, kPCPbDenied = 4 };

// The system's pasteboard privacy setting for this process: 0 default, 1 ask,
// 2 alwaysAllow, 3 alwaysDeny, -1 when this macOS has no such setting (it
// arrived in 15.4). Read by key so an older SDK still compiles.
int pc_pb_behavior(void) {
	@try {
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		if (pb == nil || ![pb respondsToSelector:NSSelectorFromString(@"accessBehavior")]) {
			return -1;
		}
		id v = [pb valueForKey:@"accessBehavior"];
		if (![v isKindOfClass:[NSNumber class]]) {
			return -1;
		}
		return (int)[(NSNumber *)v integerValue];
	} @catch (NSException *e) {
		return -1;
	}
}

int pc_pb_read(int maxBytes, int *outcome, char **text, int *n) {
	*outcome = kPCPbEmpty;
	*text = NULL;
	*n = 0;
	@try {
		if (pc_pb_behavior() == 3) {
			*outcome = kPCPbDenied;
			return 0;
		}
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		if (pb == nil) {
			return kPCPbNilErr;
		}
		NSArray<NSPasteboardType> *types = [pb types];
		for (NSString *t in @[ @"org.nspasteboard.ConcealedType", @"org.nspasteboard.TransientType", @"com.agilebits.onepassword" ]) {
			if ([types containsObject:t]) {
				*outcome = kPCPbConcealed;
				return 0;
			}
		}
		if (![types containsObject:NSPasteboardTypeString]) {
			*outcome = kPCPbEmpty;
			return 0;
		}
		NSString *s = [pb stringForType:NSPasteboardTypeString];
		if (s == nil || [s length] == 0) {
			*outcome = kPCPbEmpty;
			return 0;
		}
		NSUInteger bytes = [s lengthOfBytesUsingEncoding:NSUTF8StringEncoding];
		if (bytes > (NSUInteger)maxBytes) {
			*outcome = kPCPbTooLarge;
			return 0;
		}
		char *buf = malloc(bytes);
		if (buf == NULL) {
			return kPCPbNilErr;
		}
		memcpy(buf, [s UTF8String], bytes);
		*text = buf;
		*n = (int)bytes;
		*outcome = kPCPbOK;
		return 0;
	} @catch (NSException *e) {
		return kPCPbExceptionErr;
	}
}

// Replaces the pasteboard's contents with one plain string. It never types
// into an app.
int pc_pb_write(const char *utf8, int n) {
	@try {
		NSString *s = [[NSString alloc] initWithBytes:utf8 length:(NSUInteger)n encoding:NSUTF8StringEncoding];
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		if (s == nil || pb == nil) {
			return kPCPbNilErr;
		}
		[pb clearContents];
		return [pb setString:s forType:NSPasteboardTypeString] ? 0 : kPCPbWriteErr;
	} @catch (NSException *e) {
		return kPCPbExceptionErr;
	}
}
