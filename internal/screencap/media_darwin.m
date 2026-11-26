// media_darwin.m: the VideoToolbox H.264 encoder behind screencap.Source and
// the ScreenCaptureKit capture that feeds it. Compiled by cgo with ARC on (see
// media_darwin.go). Every entry point and callback is wrapped in @try/@catch
// and returns an error code, because an uncaught ObjC exception would end the
// daemon and every terminal session with it (research C9). Finished access
// units cross into Go once each through goScreencapAccessUnit.
//
// One capture stream per process: the stream functions share file-level state
// and the Go caller (DisplaySource) serialises them.

#import <Foundation/Foundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <VideoToolbox/VideoToolbox.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <ImageIO/ImageIO.h>
#import <CoreGraphics/CoreGraphics.h>
#import <unistd.h>

// Exported from media_darwin.go. The handle is a runtime/cgo.Handle.
extern void goScreencapAccessUnit(uintptr_t handle, void *avcc, int avccLen, void *sps, int spsLen,
                                  void *pps, int ppsLen, int keyframe, int width, int height);
extern void goScreencapStreamError(uintptr_t handle, int code, char *msg);
extern void goScreencapFrameSeen(uintptr_t handle);

enum { kPCFPS = 30, kPCQueueDepth = 5 };

// Codes of our own, outside the OSStatus and SCError ranges we pass through.
enum {
	kPCExceptionErr = -90001,
	kPCNilErr = -90002,
	kPCParamSetsErr = -90003,
	kPCTimeoutErr = -90004,
	kSCKNoDisplay = -3815, // SCStreamErrorNoDisplayList: also what we report when the main display is not listed
};

// A one-entry dictionary, or nil when either side is nil (a nil literal value
// would raise NSInvalidArgumentException).
static NSDictionary *pcDict1(id key, id value) {
	if (key == nil || value == nil) return nil;
	return @{key : value};
}

static void pcReport(uintptr_t handle, int code, const char *msg) {
	goScreencapStreamError(handle, code, (char *)msg);
}

// Converts nothing: hands the AVCC bytes and the parameter sets to Go, which
// builds the Annex-B unit (frames.go avccToAnnexB).
static void pcOutput(void *refcon, void *sourceFrameRefCon, OSStatus status, VTEncodeInfoFlags flags,
                     CMSampleBufferRef sb) {
	uintptr_t handle = (uintptr_t)refcon;
	@try {
		if (status != noErr) {
			pcReport(handle, (int)status, "encoder output callback failed");
			return;
		}
		if (sb == NULL || !CMSampleBufferDataIsReady(sb) || (flags & kVTEncodeInfo_FrameDropped)) return;
		// A sample is a keyframe unless it carries the NotSync attachment.
		BOOL key = YES;
		CFArrayRef atts = CMSampleBufferGetSampleAttachmentsArray(sb, false);
		if (atts != NULL && CFArrayGetCount(atts) > 0) {
			CFDictionaryRef d = (CFDictionaryRef)CFArrayGetValueAtIndex(atts, 0);
			key = !CFDictionaryContainsKey(d, kCMSampleAttachmentKey_NotSync);
		}
		CMFormatDescriptionRef fd = CMSampleBufferGetFormatDescription(sb);
		if (fd == NULL) {
			pcReport(handle, kPCNilErr, "encoded sample has no format description");
			return;
		}
		CMVideoDimensions dim = CMVideoFormatDescriptionGetDimensions(fd);
		const uint8_t *sps = NULL, *pps = NULL;
		size_t spsLen = 0, ppsLen = 0, count = 0;
		if (key) {
			OSStatus st = CMVideoFormatDescriptionGetH264ParameterSetAtIndex(fd, 0, &sps, &spsLen, &count, NULL);
			if (st == noErr && count >= 2) {
				st = CMVideoFormatDescriptionGetH264ParameterSetAtIndex(fd, 1, &pps, &ppsLen, NULL, NULL);
			} else if (st == noErr) {
				st = kPCParamSetsErr;
			}
			if (st != noErr) {
				pcReport(handle, (int)st, "cannot read SPS and PPS from a keyframe");
				return;
			}
		}
		CMBlockBufferRef bb = CMSampleBufferGetDataBuffer(sb);
		size_t total = 0, avail = 0;
		char *ptr = NULL;
		OSStatus st = bb == NULL ? kPCNilErr : CMBlockBufferGetDataPointer(bb, 0, &avail, &total, &ptr);
		if (st != noErr || avail != total) {
			pcReport(handle, st != noErr ? (int)st : (int)kCMBlockBufferStructureAllocationFailedErr,
			         "encoded sample is not contiguous");
			return;
		}
		goScreencapAccessUnit(handle, ptr, (int)total, (void *)sps, (int)spsLen, (void *)pps, (int)ppsLen,
		                      key ? 1 : 0, (int)dim.width, (int)dim.height);
	} @catch (NSException *e) {
		pcReport(handle, kPCExceptionErr, "exception in encoder output callback");
	}
}

static OSStatus pcSet(VTCompressionSessionRef s, CFStringRef key, CFTypeRef value) {
	if (key == NULL || value == NULL) return kPCNilErr;
	return VTSessionSetProperty(s, key, value);
}

// PCEncoder owns one VTCompressionSession and the last pixel buffer it was fed,
// so a keyframe can be forced on a still screen (frames arrive only on change).
// Every method takes the lock itself; capture threads and Go threads both call in.
@interface PCEncoder : NSObject {
	NSLock *_lock;
	VTCompressionSessionRef _session;
	CVPixelBufferRef _cached;
	int _w, _h, _fps, _bitrate;
	double _keySecs;
	CMTime _lastPts;
	BOOL _forceNext;
	int _hardware; // 1 hardware, 0 software, -1 unknown
}
@property(nonatomic) uintptr_t handle;
- (OSStatus)startWidth:(int)w height:(int)h fps:(int)fps bitrate:(int)bitrate keySecs:(double)keySecs;
- (OSStatus)encode:(CVPixelBufferRef)pb pts:(CMTime)pts;
- (OSStatus)forceKeyframe;
- (OSStatus)rebuildWidth:(int)w height:(int)h;
- (int)hardware;
- (void)stop;
@end

@implementation PCEncoder
- (instancetype)init {
	if ((self = [super init])) {
		_lock = [[NSLock alloc] init];
		_lastPts = kCMTimeInvalid;
		_hardware = -1;
	}
	return self;
}

- (void)dealloc {
	[self stop];
}

// Complete and invalidate the session and drop the cached frame. Lock held.
- (void)teardownLocked {
	if (_session != NULL) {
		VTCompressionSessionCompleteFrames(_session, kCMTimeInvalid);
		VTCompressionSessionInvalidate(_session);
		CFRelease(_session);
		_session = NULL;
	}
	if (_cached != NULL) {
		CVPixelBufferRelease(_cached);
		_cached = NULL;
	}
}

// Constrained Baseline, realtime, no reordering, average bitrate, a keyframe at
// least every _keySecs, hardware preferred (not required). Lock held.
- (OSStatus)createSessionLocked {
	NSDictionary *spec = pcDict1((__bridge id)kVTVideoEncoderSpecification_EnableHardwareAcceleratedVideoEncoder, @YES);
	if (spec == nil) return kPCNilErr;
	VTCompressionSessionRef s = NULL;
	OSStatus st = VTCompressionSessionCreate(NULL, _w, _h, kCMVideoCodecType_H264, (__bridge CFDictionaryRef)spec, NULL,
	                                         NULL, pcOutput, (void *)self.handle, &s);
	if (st != noErr) return st;
	_session = s;
#define PC_SET(key, value)                                  \
	if ((st = pcSet(s, (key), (value))) != noErr) {         \
		[self teardownLocked];                              \
		return st;                                          \
	}
	PC_SET(kVTCompressionPropertyKey_RealTime, kCFBooleanTrue)
	PC_SET(kVTCompressionPropertyKey_AllowFrameReordering, kCFBooleanFalse)
	PC_SET(kVTCompressionPropertyKey_ProfileLevel, kVTProfileLevel_H264_ConstrainedBaseline_AutoLevel)
	PC_SET(kVTCompressionPropertyKey_AverageBitRate, (__bridge CFTypeRef)@(_bitrate))
	PC_SET(kVTCompressionPropertyKey_ExpectedFrameRate, (__bridge CFTypeRef)@(_fps))
	PC_SET(kVTCompressionPropertyKey_MaxKeyFrameIntervalDuration, (__bridge CFTypeRef)@(_keySecs))
	PC_SET(kVTCompressionPropertyKey_ColorPrimaries, kCVImageBufferColorPrimaries_ITU_R_709_2)
	PC_SET(kVTCompressionPropertyKey_TransferFunction, kCVImageBufferTransferFunction_ITU_R_709_2)
	PC_SET(kVTCompressionPropertyKey_YCbCrMatrix, kCVImageBufferYCbCrMatrix_ITU_R_709_2)
#undef PC_SET
	if ((st = VTCompressionSessionPrepareToEncodeFrames(s)) != noErr) {
		[self teardownLocked];
		return st;
	}
	// Read back what the encoder really is; Go logs and surfaces a software one.
	CFTypeRef hw = NULL;
	_hardware = -1;
	if (VTSessionCopyProperty(s, kVTCompressionPropertyKey_UsingHardwareAcceleratedVideoEncoder, NULL, &hw) == noErr &&
	    hw != NULL && CFGetTypeID(hw) == CFBooleanGetTypeID()) {
		_hardware = CFBooleanGetValue((CFBooleanRef)hw) ? 1 : 0;
	}
	if (hw != NULL) CFRelease(hw);
	return noErr;
}

- (OSStatus)startWidth:(int)w height:(int)h fps:(int)fps bitrate:(int)bitrate keySecs:(double)keySecs {
	[_lock lock];
	@try {
		[self teardownLocked];
		_w = w;
		_h = h;
		_fps = fps;
		_bitrate = bitrate;
		_keySecs = keySecs;
		_lastPts = kCMTimeInvalid;
		_forceNext = NO;
		return [self createSessionLocked];
	} @catch (NSException *e) {
		return kPCExceptionErr;
	} @finally {
		[_lock unlock];
	}
}

// Feeds one frame and keeps it as the cached picture. Lock held.
- (OSStatus)encodeLocked:(CVPixelBufferRef)pb pts:(CMTime)pts force:(BOOL)force {
	// The encoder wants strictly increasing timestamps.
	if (CMTIME_IS_VALID(_lastPts) && CMTimeCompare(pts, _lastPts) <= 0) {
		pts = CMTimeAdd(_lastPts, CMTimeMake(1, 1000));
	}
	_lastPts = pts;
	NSDictionary *props = force ? pcDict1((__bridge id)kVTEncodeFrameOptionKey_ForceKeyFrame, @YES) : nil;
	return VTCompressionSessionEncodeFrame(_session, pb, pts, kCMTimeInvalid, (__bridge CFDictionaryRef)props, NULL, NULL);
}

- (OSStatus)encode:(CVPixelBufferRef)pb pts:(CMTime)pts {
	[_lock lock];
	@try {
		if (_session == NULL) return kVTInvalidSessionErr;
		// A frame captured at the old size while a rebuild is in flight is dropped.
		if ((int)CVPixelBufferGetWidth(pb) != _w || (int)CVPixelBufferGetHeight(pb) != _h) return noErr;
		CVPixelBufferRetain(pb);
		if (_cached != NULL) CVPixelBufferRelease(_cached);
		_cached = pb;
		BOOL force = _forceNext;
		_forceNext = NO;
		return [self encodeLocked:pb pts:pts force:force];
	} @catch (NSException *e) {
		return kPCExceptionErr;
	} @finally {
		[_lock unlock];
	}
}

// Re-encodes the cached picture as an IDR. With nothing cached yet the next
// real frame becomes the keyframe instead.
- (OSStatus)forceKeyframe {
	[_lock lock];
	@try {
		if (_session == NULL) return kVTInvalidSessionErr;
		if (_cached == NULL) {
			_forceNext = YES;
			return noErr;
		}
		return [self encodeLocked:_cached pts:CMTimeAdd(_lastPts, CMTimeMake(1, 1000)) force:YES];
	} @catch (NSException *e) {
		return kPCExceptionErr;
	} @finally {
		[_lock unlock];
	}
}

- (OSStatus)rebuildWidth:(int)w height:(int)h {
	[_lock lock];
	@try {
		if (_session == NULL) return kVTInvalidSessionErr;
		[self teardownLocked]; // flushes the old size's units before the new session starts
		_w = w;
		_h = h;
		_forceNext = YES;
		return [self createSessionLocked];
	} @catch (NSException *e) {
		return kPCExceptionErr;
	} @finally {
		[_lock unlock];
	}
}

- (int)hardware {
	[_lock lock];
	int hw = _hardware;
	[_lock unlock];
	return hw;
}

- (void)stop {
	[_lock lock];
	@try {
		[self teardownLocked];
	} @catch (NSException *e) {
	} @finally {
		[_lock unlock];
	}
}
@end

// A moving gradient on an IOSurface-backed NV12 buffer, so frames differ.
static CVPixelBufferRef pcMakeNV12(int w, int h, int frame) {
	NSDictionary *attrs = pcDict1((__bridge id)kCVPixelBufferIOSurfacePropertiesKey, @{});
	if (attrs == nil) return NULL;
	CVPixelBufferRef pb = NULL;
	if (CVPixelBufferCreate(NULL, w, h, kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange, (__bridge CFDictionaryRef)attrs,
	                        &pb) != kCVReturnSuccess) {
		return NULL;
	}
	CVPixelBufferLockBaseAddress(pb, 0);
	uint8_t *y = CVPixelBufferGetBaseAddressOfPlane(pb, 0);
	size_t ys = CVPixelBufferGetBytesPerRowOfPlane(pb, 0);
	for (int r = 0; r < h; r++) {
		for (int c = 0; c < w; c++) {
			y[(size_t)r * ys + c] = (uint8_t)(16 + ((c * 219 / w + r / 4 + frame * 6) % 219));
		}
	}
	uint8_t *uv = CVPixelBufferGetBaseAddressOfPlane(pb, 1);
	size_t uvs = CVPixelBufferGetBytesPerRowOfPlane(pb, 1);
	for (size_t r = 0; r < CVPixelBufferGetHeightOfPlane(pb, 1); r++) memset(uv + r * uvs, 128, uvs);
	CVPixelBufferUnlockBaseAddress(pb, 0);
	return pb;
}

// Feeds `frames` synthetic NV12 frames through a real encoder. At frame index
// `force` it forces a keyframe, at `rebuildAt` it rebuilds at rw x rh (a
// negative index disables either). The encoder's hardware readback comes back
// in *hardware. Returns 0 or an OSStatus. Not capture: no TCC is involved.
int pc_encode_synthetic(uintptr_t handle, int frames, int w, int h, int force, int rebuildAt, int rw, int rh,
                        int *hardware) {
	@try {
		PCEncoder *enc = [[PCEncoder alloc] init];
		enc.handle = handle;
		OSStatus st = [enc startWidth:w height:h fps:kPCFPS bitrate:8000000 keySecs:4.0];
		if (st != noErr) return (int)st;
		*hardware = [enc hardware];
		int cw = w, ch = h;
		for (int i = 0; i < frames; i++) {
			if (i == force && (st = [enc forceKeyframe]) != noErr) break;
			if (i == rebuildAt) {
				if ((st = [enc rebuildWidth:rw height:rh]) != noErr) break;
				cw = rw;
				ch = rh;
				*hardware = [enc hardware];
			}
			CVPixelBufferRef pb = pcMakeNV12(cw, ch, i);
			if (pb == NULL) {
				st = kPCNilErr;
				break;
			}
			st = [enc encode:pb pts:CMTimeMake(i, kPCFPS)];
			CVPixelBufferRelease(pb);
			if (st != noErr) break;
			usleep(2000);
		}
		[enc stop];
		return (int)st;
	} @catch (NSException *e) {
		return kPCExceptionErr;
	}
}

// ---------------------------------------------------------------------------
// ScreenCaptureKit capture of the main display
// ---------------------------------------------------------------------------

static void pcSetErr(char *buf, int len, NSString *msg) {
	if (buf != NULL && len > 0) snprintf(buf, (size_t)len, "%s", msg.UTF8String ?: "");
}

// The same math as captureSize in frames.go: scale both sides down together
// when the long edge exceeds maxLong (never up), each side rounded down to even.
static void pcCaptureSize(double pw, double ph, int maxLong, int *w, int *h) {
	double longEdge = pw > ph ? pw : ph, f = 1.0;
	if (maxLong > 0 && longEdge > maxLong) f = maxLong / longEdge;
	*w = ((int)lround(pw * f)) & ~1;
	*h = ((int)lround(ph * f)) & ~1;
}

// Resolves the main display (CGMainDisplayID) to a content filter and its
// capture size: display points times the filter's pointPixelScale, capped.
// Returns 0 or an error code (SCK codes pass through; text in errbuf).
static int pcMainDisplay(int maxLong, SCContentFilter **filter, int *w, int *h, char *errbuf, int errlen) {
	CGDirectDisplayID main = CGMainDisplayID(); // also connects to the window server before SCK asks
	__block SCShareableContent *content = nil;
	__block NSError *err = nil;
	dispatch_semaphore_t sem = dispatch_semaphore_create(0);
	[SCShareableContent getShareableContentExcludingDesktopWindows:NO
	                                           onScreenWindowsOnly:YES
	                                             completionHandler:^(SCShareableContent *c, NSError *e) {
		                                             content = c;
		                                             err = e;
		                                             dispatch_semaphore_signal(sem);
	                                             }];
	if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 15 * NSEC_PER_SEC)) != 0) {
		pcSetErr(errbuf, errlen, @"SCShareableContent timed out");
		return kPCTimeoutErr;
	}
	if (content == nil) {
		pcSetErr(errbuf, errlen, err.localizedDescription ?: @"SCShareableContent returned nothing");
		return err != nil && err.code != 0 ? (int)err.code : kPCNilErr;
	}
	SCDisplay *display = nil;
	for (SCDisplay *d in content.displays) {
		if (d.displayID == main) display = d;
	}
	if (display == nil) {
		pcSetErr(errbuf, errlen, @"the main display is not in the shareable content");
		return kSCKNoDisplay;
	}
	SCContentFilter *f = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
	double scale = f.pointPixelScale;
	pcCaptureSize(display.width * scale, display.height * scale, maxLong, w, h);
	*filter = f;
	return 0;
}

@interface PCStreamHandler : NSObject <SCStreamOutput, SCStreamDelegate>
@property(nonatomic) uintptr_t handle;
@property(nonatomic, strong) PCEncoder *enc;
@end

@implementation PCStreamHandler
- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sb ofType:(SCStreamOutputType)type {
	@try {
		if (sb == NULL || type != SCStreamOutputTypeScreen) return;
		CFArrayRef atts = CMSampleBufferGetSampleAttachmentsArray(sb, false);
		NSDictionary *info = (atts != NULL && CFArrayGetCount(atts) > 0)
		                         ? (__bridge NSDictionary *)CFArrayGetValueAtIndex(atts, 0)
		                         : nil;
		NSNumber *status = info[SCStreamFrameInfoStatus];
		// Idle, blank and suspended frames carry no new picture.
		if (status == nil || status.integerValue != SCFrameStatusComplete) return;
		CVPixelBufferRef pb = CMSampleBufferGetImageBuffer(sb);
		if (pb == NULL) return;
		goScreencapFrameSeen(self.handle);
		OSStatus st = [self.enc encode:pb pts:CMSampleBufferGetPresentationTimeStamp(sb)];
		if (st != noErr) pcReport(self.handle, (int)st, "encoding a captured frame failed");
	} @catch (NSException *e) {
		pcReport(self.handle, kPCExceptionErr, "exception in the capture frame handler");
	}
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
	@try {
		pcReport(self.handle, (int)error.code, error.localizedDescription.UTF8String ?: "capture stream stopped");
	} @catch (NSException *e) {
		pcReport(self.handle, kPCExceptionErr, "exception in the capture stop handler");
	}
}
@end

static SCStream *gStream;
static SCStreamConfiguration *gConfig;
static PCStreamHandler *gHandler;
static PCEncoder *gEnc;
static dispatch_queue_t gQueue;

// Starts capturing the main display into a fresh encoder at the capture size.
// Returns 0 or an error code: SCK codes pass through (-3801 no Screen
// Recording grant, -3815 no display), OSStatus for the encoder.
int pc_stream_start(uintptr_t handle, int fps, int maxLong, int bitrate, double keyframeSecs, char *errbuf,
                    int errlen) {
	@try {
		if (gStream != nil) {
			pcSetErr(errbuf, errlen, @"a capture stream is already running");
			return kPCNilErr;
		}
		SCContentFilter *filter = nil;
		int w = 0, h = 0;
		int rc = pcMainDisplay(maxLong, &filter, &w, &h, errbuf, errlen);
		if (rc != 0) return rc;

		PCEncoder *enc = [[PCEncoder alloc] init];
		enc.handle = handle;
		OSStatus st = [enc startWidth:w height:h fps:fps bitrate:bitrate keySecs:keyframeSecs];
		if (st != noErr) {
			pcSetErr(errbuf, errlen, @"starting the encoder failed");
			return (int)st;
		}

		SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
		cfg.width = (size_t)w;
		cfg.height = (size_t)h;
		cfg.minimumFrameInterval = CMTimeMake(1, fps);
		cfg.queueDepth = kPCQueueDepth;
		cfg.showsCursor = YES;
		cfg.pixelFormat = kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange;
		cfg.colorMatrix = kCGDisplayStreamYCbCrMatrix_ITU_R_709_2;

		PCStreamHandler *hnd = [[PCStreamHandler alloc] init];
		hnd.handle = handle;
		hnd.enc = enc;
		dispatch_queue_t q = dispatch_queue_create("portty.screencap", DISPATCH_QUEUE_SERIAL);
		SCStream *s = [[SCStream alloc] initWithFilter:filter configuration:cfg delegate:hnd];
		NSError *err = nil;
		if (s == nil || ![s addStreamOutput:hnd type:SCStreamOutputTypeScreen sampleHandlerQueue:q error:&err]) {
			[enc stop];
			pcSetErr(errbuf, errlen, err.localizedDescription ?: @"adding the stream output failed");
			return err != nil && err.code != 0 ? (int)err.code : kPCNilErr;
		}
		__block NSError *startErr = nil;
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		[s startCaptureWithCompletionHandler:^(NSError *e) {
			startErr = e;
			dispatch_semaphore_signal(sem);
		}];
		if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 20 * NSEC_PER_SEC)) != 0) {
			[s stopCaptureWithCompletionHandler:^(NSError *e) {
			}];
			[enc stop];
			pcSetErr(errbuf, errlen, @"starting capture timed out");
			return kPCTimeoutErr;
		}
		if (startErr != nil) {
			[enc stop];
			pcSetErr(errbuf, errlen, startErr.localizedDescription ?: @"starting capture failed");
			return startErr.code != 0 ? (int)startErr.code : kPCNilErr;
		}
		gStream = s;
		gConfig = cfg;
		gHandler = hnd;
		gEnc = enc;
		gQueue = q;
		return 0;
	} @catch (NSException *e) {
		pcSetErr(errbuf, errlen, @"exception starting capture");
		return kPCExceptionErr;
	}
}

// Stops capture, lets a frame handler already in flight finish, then closes
// the encoder (which flushes its last units). Safe to call with nothing running.
void pc_stream_stop(void) {
	@try {
		SCStream *s = gStream;
		if (s != nil) {
			dispatch_semaphore_t sem = dispatch_semaphore_create(0);
			[s stopCaptureWithCompletionHandler:^(NSError *e) {
				dispatch_semaphore_signal(sem);
			}];
			dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
		}
		if (gQueue != nil) dispatch_sync(gQueue, ^{});
		[gEnc stop];
	} @catch (NSException *e) {
	}
	gStream = nil;
	gConfig = nil;
	gHandler = nil;
	gEnc = nil;
	gQueue = nil;
}

int pc_encoder_force_keyframe(void) {
	@try {
		return gEnc == nil ? (int)kVTInvalidSessionErr : (int)[gEnc forceKeyframe];
	} @catch (NSException *e) {
		return kPCExceptionErr;
	}
}

// Rebuilds the encoder at w x h, then resizes the capture to match. The encoder
// goes first so no frame of the new size meets the old session; frames of the
// old size in between are dropped by the encoder.
int pc_encoder_rebuild(int w, int h) {
	@try {
		if (gEnc == nil) return (int)kVTInvalidSessionErr;
		OSStatus st = [gEnc rebuildWidth:w height:h];
		if (st != noErr) return (int)st;
		if (gStream == nil || gConfig == nil) return 0;
		gConfig.width = (size_t)w;
		gConfig.height = (size_t)h;
		__block NSError *updErr = nil;
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		[gStream updateConfiguration:gConfig
		           completionHandler:^(NSError *e) {
			           updErr = e;
			           dispatch_semaphore_signal(sem);
		           }];
		if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC)) != 0) return kPCTimeoutErr;
		return updErr == nil ? 0 : (updErr.code != 0 ? (int)updErr.code : kPCNilErr);
	} @catch (NSException *e) {
		return kPCExceptionErr;
	}
}

// 1 hardware, 0 software, -1 unknown or no encoder.
int pc_encoder_hardware(void) {
	@try {
		return gEnc == nil ? -1 : [gEnc hardware];
	} @catch (NSException *e) {
		return -1;
	}
}

// One JPEG (quality 0.8) of the main display, at most maxLong px on the long
// edge, in a malloc'd buffer the caller frees. Returns 0 or an error code.
int pc_still_capture(int maxLong, uint8_t **out, int *n, char *errbuf, int errlen) {
	@try {
		SCContentFilter *filter = nil;
		int w = 0, h = 0;
		int rc = pcMainDisplay(maxLong, &filter, &w, &h, errbuf, errlen);
		if (rc != 0) return rc;
		SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
		cfg.width = (size_t)w;
		cfg.height = (size_t)h;
		__block CGImageRef image = NULL;
		__block NSError *err = nil;
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		[SCScreenshotManager captureImageWithFilter:filter
		                              configuration:cfg
		                          completionHandler:^(CGImageRef img, NSError *e) {
			                          if (img != NULL) image = CGImageRetain(img);
			                          err = e;
			                          dispatch_semaphore_signal(sem);
		                          }];
		if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 15 * NSEC_PER_SEC)) != 0) {
			pcSetErr(errbuf, errlen, @"the screenshot timed out");
			return kPCTimeoutErr;
		}
		if (image == NULL) {
			pcSetErr(errbuf, errlen, err.localizedDescription ?: @"the screenshot returned no image");
			return err != nil && err.code != 0 ? (int)err.code : kPCNilErr;
		}
		CFMutableDataRef data = CFDataCreateMutable(NULL, 0);
		CGImageDestinationRef dest = CGImageDestinationCreateWithData(data, CFSTR("public.jpeg"), 1, NULL);
		NSDictionary *props = pcDict1((__bridge id)kCGImageDestinationLossyCompressionQuality, @0.8);
		int result = kPCNilErr;
		if (dest != NULL && props != nil) {
			CGImageDestinationAddImage(dest, image, (__bridge CFDictionaryRef)props);
			if (CGImageDestinationFinalize(dest)) {
				*n = (int)CFDataGetLength(data);
				*out = malloc((size_t)*n);
				if (*out != NULL) {
					memcpy(*out, CFDataGetBytePtr(data), (size_t)*n);
					result = 0;
				}
			}
		}
		if (result != 0) pcSetErr(errbuf, errlen, @"encoding the screenshot as JPEG failed");
		if (dest != NULL) CFRelease(dest);
		CFRelease(data);
		CGImageRelease(image);
		return result;
	} @catch (NSException *e) {
		pcSetErr(errbuf, errlen, @"exception taking the screenshot");
		return kPCExceptionErr;
	}
}
