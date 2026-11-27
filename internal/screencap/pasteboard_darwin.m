// pasteboard_darwin.m: the Mac's general pasteboard, read and written only when
// a viewer asks (see internal/desktop/clipboard.go). Compiled by cgo with ARC
// on (media_darwin.go sets the flags). Every function returns 0 or an error
// code and is wrapped in @try/@catch, because an uncaught ObjC exception would
// end the daemon.
//
// A read looks at the item types first and answers "concealed" without reading
// any content when a password manager or a transient flag marks the item.
// The change counter is read only around one read, to make the type check and
// the text read one snapshot (M-06): there is no polling and no background sync.

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

enum { kPCPbExceptionErr = -90301, kPCPbNilErr = -90302, kPCPbWriteErr = -90303, kPCPbChangedErr = -90304 };

// Outcomes of pc_pb_read, mirrored by ReadOutcome in pasteboard_darwin.go.
enum { kPCPbOK = 0, kPCPbConcealed = 1, kPCPbEmpty = 2, kPCPbTooLarge = 3, kPCPbDenied = 4 };

// The system's pasteboard privacy setting for this process: 0 default, 1 ask,
// 2 alwaysAllow, 3 alwaysDeny, -1 when this macOS has no such setting (it
// arrived in 15.4). Read by key so an older SDK still compiles.
int pc_pb_behavior(void) {
	@autoreleasepool {
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
}

// pcPbReadOnce reads one snapshot. The change count is taken before the type
// check and compared after the text read: a write in between (a password
// manager storing a concealed secret) could otherwise be read without its
// marker ever being seen, so a changed count discards the text (*changed = YES).
static int pcPbReadOnce(NSPasteboard *pb, int maxBytes, int *outcome, char **text, int *n, BOOL *changed) {
	*changed = NO;
	NSInteger before = [pb changeCount];
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
	if ([pb changeCount] != before) {
		*changed = YES;
		return 0;
	}
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
}

// Reads the pasteboard's plain text. A snapshot that changed under the read is
// retried once; a second change refuses (kPCPbChangedErr) rather than risk
// handing out content whose type was never checked.
int pc_pb_read(int maxBytes, int *outcome, char **text, int *n) {
	@autoreleasepool {
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
			for (int attempt = 0; attempt < 2; attempt++) {
				BOOL changed = NO;
				int rc = pcPbReadOnce(pb, maxBytes, outcome, text, n, &changed);
				if (rc != 0 || !changed) {
					return rc;
				}
				*outcome = kPCPbEmpty;
			}
			return kPCPbChangedErr;
		} @catch (NSException *e) {
			return kPCPbExceptionErr;
		}
	}
}

// Replaces the pasteboard's contents with one plain string. It never types
// into an app.
int pc_pb_write(const char *utf8, int n) {
	@autoreleasepool {
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
}
