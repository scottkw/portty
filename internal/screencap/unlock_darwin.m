// unlock_darwin.m: the native half of remote unlock. It builds a reverse map
// from the current keyboard layout (character to key and modifiers) with
// UCKeyTranslate, reads Caps Lock, and posts the wake key and the typed
// strokes. Compiled by cgo with ARC on (see media_darwin.go). Every function
// returns 0 or an error code and is wrapped in @try/@catch, because an
// uncaught ObjC exception would end the daemon.
//
// Nothing here keeps a password: the UTF-8 bytes are decoded into a scratch
// buffer that is zeroed before pc_unlock_map returns, and no function logs.
// Events are posted through pc_input_key (input_darwin.m), so they carry the
// Portty event marker and go to the HID tap like every other injected key.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <Carbon/Carbon.h>
#import <stdlib.h>
#import <string.h>
#import <unistd.h>

enum {
	kPCUnlockOK = 0,
	kPCUnlockUnmappable = 1, // a character has no key; keep equal to unlockUnmappableRC
	kPCUnlockExceptionErr = -90201,
	kPCUnlockNilErr = -90202,
	kPCUnlockRangeErr = -90203,
};

typedef struct {
	uint16_t vk;
	uint64_t flags;
} pc_stroke;

extern int pc_input_key(uint16_t vk, int down, uint64_t flags, int autorepeat);
extern int pc_declare_remote_activity(const char *name);

// The modifier sets tried for each key, in preference order: a character that
// needs no modifier never maps to one that does. The state is UCKeyTranslate's
// (modifier mask >> 8); the flags are the CGEventFlags the event must carry.
static const struct { uint32_t state; uint64_t flags; } kPCSets[4] = {
	{0, 0},
	{shiftKey >> 8, kCGEventFlagMaskShift},
	{optionKey >> 8, kCGEventFlagMaskAlternate},
	{(shiftKey | optionKey) >> 8, kCGEventFlagMaskShift | kCGEventFlagMaskAlternate},
};

// The current layout's UCKeyboardLayout data. The returned data is retained by
// *src; the caller releases *src.
static const UCKeyboardLayout *pcLayout(TISInputSourceRef *src) {
	*src = TISCopyCurrentKeyboardLayoutInputSource();
	if (*src == NULL) return NULL;
	CFDataRef data = (CFDataRef)TISGetInputSourceProperty(*src, kTISPropertyUnicodeKeyLayoutData);
	if (data == NULL) return NULL;
	return (const UCKeyboardLayout *)CFDataGetBytePtr(data);
}

// One key's character under one modifier state, or 0 when it types none or a
// dead key. Only a single UTF-16 unit counts: a password character is a plain
// BMP key.
static uint32_t pcTranslate(const UCKeyboardLayout *layout, UInt32 kbType, uint16_t vk, uint32_t state) {
	UInt32 dead = 0;
	UniCharCount len = 0;
	UniChar buf[4] = {0, 0, 0, 0};
	OSStatus st = UCKeyTranslate(layout, vk, kUCKeyActionDown, state, kbType,
	                             kUCKeyTranslateNoDeadKeysBit, &dead, 4, &len, buf);
	if (st != noErr || len != 1 || dead != 0) return 0;
	return buf[0];
}

// A character a key can type on purpose: not a control character, not Delete,
// not the private-use range the function keys translate to.
static int pcPrintable(uint32_t c) {
	return c >= 0x20 && c != 0x7F && !(c >= 0xF700 && c <= 0xF8FF);
}

// Decodes strict UTF-8 into cps (room for n). Returns the count, or -1 for
// invalid UTF-8 (overlong, surrogate, past U+10FFFF, truncated).
static int pcDecode(const uint8_t *s, int n, uint32_t *cps) {
	int count = 0;
	for (int i = 0; i < n;) {
		uint8_t b = s[i];
		uint32_t c;
		int extra;
		if (b < 0x80) { c = b; extra = 0; }
		else if (b >= 0xC2 && b <= 0xDF) { c = b & 0x1F; extra = 1; }
		else if (b >= 0xE0 && b <= 0xEF) { c = b & 0x0F; extra = 2; }
		else if (b >= 0xF0 && b <= 0xF4) { c = b & 0x07; extra = 3; }
		else return -1;
		if (i + extra >= n) return -1; // truncated
		for (int k = 1; k <= extra; k++) {
			if ((s[i + k] & 0xC0) != 0x80) return -1;
			c = (c << 6) | (s[i + k] & 0x3F);
		}
		if ((extra == 2 && c < 0x800) || (extra == 3 && c < 0x10000)) return -1; // overlong
		if (c >= 0xD800 && c <= 0xDFFF) return -1;
		if (c > 0x10FFFF) return -1;
		cps[count++] = c;
		i += extra + 1;
	}
	return count;
}

// Maps n bytes of UTF-8 to one stroke per character in out (room for n). On
// success *count is the stroke count. Returns kPCUnlockUnmappable, before any
// stroke is valid, for a character with no key, invalid UTF-8 or a
// non-printable one.
int pc_unlock_map(const uint8_t *utf8, int n, pc_stroke *out, int *count) {
	@autoreleasepool {
		uint32_t *cps = NULL;
		TISInputSourceRef src = NULL;
		int rc = kPCUnlockOK;
		@try {
			if (utf8 == NULL || out == NULL || count == NULL || n < 1) return kPCUnlockRangeErr;
			cps = (uint32_t *)calloc((size_t)n, sizeof(uint32_t));
			if (cps == NULL) return kPCUnlockNilErr;
			int m = pcDecode(utf8, n, cps);
			if (m < 1) { rc = kPCUnlockUnmappable; goto done; }
			const UCKeyboardLayout *layout = pcLayout(&src);
			if (layout == NULL) { rc = kPCUnlockNilErr; goto done; }
			UInt32 kbType = LMGetKbdType();
			for (int i = 0; i < m; i++) {
				if (!pcPrintable(cps[i])) { rc = kPCUnlockUnmappable; goto done; }
				int found = 0;
				// Modifier set outermost, key inner: the first hit is the plainest
				// stroke, and of equal ones the lowest key (the main block, not the keypad).
				for (int s = 0; s < 4 && !found; s++) {
					for (uint16_t vk = 0; vk < 128 && !found; vk++) {
						if (pcTranslate(layout, kbType, vk, kPCSets[s].state) == cps[i]) {
							out[i].vk = vk;
							out[i].flags = kPCSets[s].flags;
							found = 1;
						}
					}
				}
				if (!found) { rc = kPCUnlockUnmappable; goto done; }
			}
			*count = m;
		done:;
		} @catch (NSException *e) {
			rc = kPCUnlockExceptionErr;
		}
		if (cps != NULL) {
			memset_s(cps, (size_t)n * sizeof(uint32_t), 0, (size_t)n * sizeof(uint32_t));
			free(cps);
		}
		if (src != NULL) CFRelease(src);
		if (rc != kPCUnlockOK) memset_s(out, (size_t)n * sizeof(pc_stroke), 0, (size_t)n * sizeof(pc_stroke));
		return rc;
	}
}

// What one stroke types, through UCKeyTranslate: the round-trip check of the map.
int pc_unlock_translate(uint16_t vk, uint64_t flags, uint32_t *cp) {
	@autoreleasepool {
		TISInputSourceRef src = NULL;
		@try {
			if (cp == NULL) return kPCUnlockRangeErr;
			const UCKeyboardLayout *layout = pcLayout(&src);
			if (layout == NULL) return kPCUnlockNilErr;
			UInt32 kbType = LMGetKbdType();
			uint32_t state = 0;
			if (flags & kCGEventFlagMaskShift) state |= shiftKey >> 8;
			if (flags & kCGEventFlagMaskAlternate) state |= optionKey >> 8;
			*cp = pcTranslate(layout, kbType, vk, state);
			CFRelease(src);
			return 0;
		} @catch (NSException *e) {
			if (src != NULL) CFRelease(src);
			return kPCUnlockExceptionErr;
		}
	}
}

// *on is 1 while Caps Lock is on at the Mac (the alpha-shift flag of the
// hardware state). A read only.
int pc_unlock_caps_lock(int *on) {
	@try {
		if (on == NULL) return kPCUnlockRangeErr;
		CGEventFlags f = CGEventSourceFlagsState(kCGEventSourceStateHIDSystemState);
		*on = (f & kCGEventFlagMaskAlphaShift) != 0;
		return 0;
	} @catch (NSException *e) {
		return kPCUnlockExceptionErr;
	}
}

// Lights a dark display (a user-activity declaration), then presses vk once. A
// Shift key goes down with the Shift flag set, so it reads as a modifier press.
int pc_unlock_wake(uint16_t vk) {
	@try {
		int rc = pc_declare_remote_activity("Portty remote unlock");
		if (rc != 0) return rc;
		uint64_t downFlags = (vk == kVK_Shift || vk == kVK_RightShift) ? (uint64_t)kCGEventFlagMaskShift : 0;
		rc = pc_input_key(vk, 1, downFlags, 0);
		if (rc != 0) return rc;
		usleep(60 * 1000);
		return pc_input_key(vk, 0, 0, 0);
	} @catch (NSException *e) {
		return kPCUnlockExceptionErr;
	}
}

// One typed key: down with the stroke's flags, 60 ms, up. The up always posts
// when the down did, so no key is left held on an unattended Mac.
int pc_unlock_key(uint16_t vk, uint64_t flags) {
	@try {
		int rc = pc_input_key(vk, 1, flags, 0);
		if (rc != 0) return rc;
		usleep(60 * 1000);
		return pc_input_key(vk, 0, flags, 0);
	} @catch (NSException *e) {
		return kPCUnlockExceptionErr;
	}
}
