// media_darwin.m: synthetic frame source and VideoToolbox H.264 encoder with
// Annex-B output for the desktop feasibility spike. Compiled by cgo with ARC on
// (see bridge_darwin.go). Each finished access unit is handed to Go once through
// spkOnAccessUnit; every OSStatus failure is returned to the caller and kept for
// spk_encoder_info so a probe never reports success it did not get.

#import <Foundation/Foundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <VideoToolbox/VideoToolbox.h>

// Exported from bridge_darwin.go.
extern void spkOnAccessUnit(void *data, int length, long long ptsNs, int keyframe);

static VTCompressionSessionRef gSession;
static CVPixelBufferPoolRef gPool;
static dispatch_queue_t gQueue;
static dispatch_source_t gTimer;
static volatile int gForceKey;
static int gWidth, gHeight, gFps;
static int gHardware;
static OSStatus gLastStatus;
static char gEncoderId[128];
static char gInfo[384];

// Converts one encoded sample (AVCC, 4-byte lengths) to Annex-B and hands it to Go.
static void outputCallback(void *refcon, void *sourceFrameRefCon, OSStatus status,
                           VTEncodeInfoFlags flags, CMSampleBufferRef sb) {
	if (status != noErr) {
		gLastStatus = status;
		return;
	}
	if (sb == NULL || !CMSampleBufferDataIsReady(sb) || (flags & kVTEncodeInfo_FrameDropped)) {
		return;
	}
	static const uint8_t startCode[4] = {0, 0, 0, 1};
	BOOL key = YES;
	CFArrayRef atts = CMSampleBufferGetSampleAttachmentsArray(sb, false);
	if (atts != NULL && CFArrayGetCount(atts) > 0) {
		CFDictionaryRef d = (CFDictionaryRef)CFArrayGetValueAtIndex(atts, 0);
		key = !CFDictionaryContainsKey(d, kCMSampleAttachmentKey_NotSync);
	}
	NSMutableData *out = [NSMutableData data];
	if (key) {
		CMFormatDescriptionRef fd = CMSampleBufferGetFormatDescription(sb);
		size_t count = 0;
		OSStatus st = CMVideoFormatDescriptionGetH264ParameterSetAtIndex(fd, 0, NULL, NULL, &count, NULL);
		if (st != noErr) {
			gLastStatus = st;
			return;
		}
		for (size_t i = 0; i < count; i++) {
			const uint8_t *ps = NULL;
			size_t psSize = 0;
			st = CMVideoFormatDescriptionGetH264ParameterSetAtIndex(fd, i, &ps, &psSize, NULL, NULL);
			if (st != noErr) {
				gLastStatus = st;
				return;
			}
			[out appendBytes:startCode length:4];
			[out appendBytes:ps length:psSize];
		}
	}
	CMBlockBufferRef bb = CMSampleBufferGetDataBuffer(sb);
	size_t total = 0, avail = 0;
	char *ptr = NULL;
	OSStatus st = CMBlockBufferGetDataPointer(bb, 0, &avail, &total, &ptr);
	if (st != noErr || avail != total) {
		gLastStatus = st != noErr ? st : kCMBlockBufferStructureAllocationFailedErr;
		return;
	}
	size_t off = 0;
	while (off + 4 <= total) {
		uint32_t n = ((uint32_t)(uint8_t)ptr[off] << 24) | ((uint32_t)(uint8_t)ptr[off + 1] << 16) |
		             ((uint32_t)(uint8_t)ptr[off + 2] << 8) | (uint32_t)(uint8_t)ptr[off + 3];
		if (off + 4 + n > total) {
			gLastStatus = kCMBlockBufferBadLengthParameterErr;
			return;
		}
		[out appendBytes:startCode length:4];
		[out appendBytes:ptr + off + 4 length:n];
		off += 4 + n;
	}
	CMTime pts = CMSampleBufferGetPresentationTimeStamp(sb);
	long long ptsNs = (long long)(CMTimeGetSeconds(pts) * 1e9);
	spkOnAccessUnit((void *)out.bytes, (int)out.length, ptsNs, key ? 1 : 0);
}

static OSStatus setProp(VTCompressionSessionRef s, CFStringRef key, CFTypeRef value) {
	OSStatus st = VTSessionSetProperty(s, key, value);
	if (st != noErr) {
		gLastStatus = st;
	}
	return st;
}

int spk_encoder_start(int width, int height, int fps, int bitrate) {
	gLastStatus = noErr;
	gWidth = width;
	gHeight = height;
	gFps = fps;
	NSDictionary *spec = @{(__bridge NSString *)kVTVideoEncoderSpecification_EnableHardwareAcceleratedVideoEncoder : @YES};
	OSStatus st = VTCompressionSessionCreate(NULL, width, height, kCMVideoCodecType_H264,
	                                         (__bridge CFDictionaryRef)spec, NULL, NULL, outputCallback, NULL, &gSession);
	if (st != noErr) {
		gSession = NULL;
		gLastStatus = st;
		return st;
	}
	VTCompressionSessionRef s = gSession;
	if ((st = setProp(s, kVTCompressionPropertyKey_RealTime, kCFBooleanTrue)) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_AllowFrameReordering, kCFBooleanFalse)) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_ProfileLevel,
	                  kVTProfileLevel_H264_ConstrainedBaseline_AutoLevel)) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_AverageBitRate, (__bridge CFTypeRef) @(bitrate))) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_ExpectedFrameRate, (__bridge CFTypeRef) @(fps))) != noErr) return st;
	// A long interval: keyframes come from viewer joins and PLI, not a timer.
	if ((st = setProp(s, kVTCompressionPropertyKey_MaxKeyFrameInterval, (__bridge CFTypeRef) @(fps * 3600))) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_ColorPrimaries, kCVImageBufferColorPrimaries_ITU_R_709_2)) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_TransferFunction, kCVImageBufferTransferFunction_ITU_R_709_2)) != noErr) return st;
	if ((st = setProp(s, kVTCompressionPropertyKey_YCbCrMatrix, kCVImageBufferYCbCrMatrix_ITU_R_709_2)) != noErr) return st;
	if ((st = VTCompressionSessionPrepareToEncodeFrames(s)) != noErr) {
		gLastStatus = st;
		return st;
	}

	// Readbacks decide what the evidence says; nothing is assumed.
	CFTypeRef hw = NULL;
	if ((st = VTSessionCopyProperty(s, kVTCompressionPropertyKey_UsingHardwareAcceleratedVideoEncoder, NULL, &hw)) != noErr) {
		gLastStatus = st;
		return st;
	}
	gHardware = hw != NULL && CFGetTypeID(hw) == CFBooleanGetTypeID() && CFBooleanGetValue((CFBooleanRef)hw);
	if (hw != NULL) CFRelease(hw);
	CFTypeRef eid = NULL;
	if ((st = VTSessionCopyProperty(s, kVTCompressionPropertyKey_EncoderID, NULL, &eid)) != noErr) {
		gLastStatus = st;
		return st;
	}
	gEncoderId[0] = 0;
	if (eid != NULL && CFGetTypeID(eid) == CFStringGetTypeID()) {
		CFStringGetCString((CFStringRef)eid, gEncoderId, sizeof gEncoderId, kCFStringEncodingUTF8);
	}
	if (eid != NULL) CFRelease(eid);
	return 0;
}

void spk_encoder_force_keyframe(void) {
	gForceKey = 1;
}

const char *spk_encoder_info(void) {
	snprintf(gInfo, sizeof gInfo,
	         "{\"encoderId\":\"%s\",\"hardware\":%s,\"width\":%d,\"height\":%d,\"lastStatus\":%d}",
	         gEncoderId, gHardware ? "true" : "false", gWidth, gHeight, (int)gLastStatus);
	return gInfo;
}

// Fills a 420v frame with a checkerboard that scrolls with the frame index.
static void fillPattern(CVPixelBufferRef pb, int frame) {
	CVPixelBufferLockBaseAddress(pb, 0);
	uint8_t *y = CVPixelBufferGetBaseAddressOfPlane(pb, 0);
	size_t ys = CVPixelBufferGetBytesPerRowOfPlane(pb, 0);
	size_t w = CVPixelBufferGetWidthOfPlane(pb, 0), h = CVPixelBufferGetHeightOfPlane(pb, 0);
	for (size_t r = 0; r < h; r++) {
		size_t rowBlock = (r + (size_t)frame * 2) / 32;
		for (size_t c = 0; c < w; c += 32) {
			size_t colBlock = (c + (size_t)frame * 4) / 32;
			size_t n = w - c < 32 ? w - c : 32;
			memset(y + r * ys + c, ((rowBlock + colBlock) & 1) ? 200 : 60, n);
		}
	}
	uint8_t *uv = CVPixelBufferGetBaseAddressOfPlane(pb, 1);
	size_t uvs = CVPixelBufferGetBytesPerRowOfPlane(pb, 1);
	size_t uvh = CVPixelBufferGetHeightOfPlane(pb, 1);
	for (size_t r = 0; r < uvh; r++) {
		memset(uv + r * uvs, 128, uvs);
	}
	CVPixelBufferUnlockBaseAddress(pb, 0);
}

int spk_synthetic_start(int width, int height, int fps) {
	if (gSession == NULL) {
		gLastStatus = kVTInvalidSessionErr;
		return kVTInvalidSessionErr;
	}
	NSDictionary *attrs = @{
		(__bridge NSString *)kCVPixelBufferPixelFormatTypeKey : @(kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange),
		(__bridge NSString *)kCVPixelBufferWidthKey : @(width),
		(__bridge NSString *)kCVPixelBufferHeightKey : @(height),
		(__bridge NSString *)kCVPixelBufferIOSurfacePropertiesKey : @{},
	};
	CVReturn cr = CVPixelBufferPoolCreate(NULL, NULL, (__bridge CFDictionaryRef)attrs, &gPool);
	if (cr != kCVReturnSuccess) {
		gLastStatus = cr;
		return cr;
	}
	gQueue = dispatch_queue_create("spike.synthetic", DISPATCH_QUEUE_SERIAL);
	gTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, gQueue);
	dispatch_source_set_timer(gTimer, dispatch_time(DISPATCH_TIME_NOW, 0), NSEC_PER_SEC / (uint64_t)fps, NSEC_PER_MSEC);
	__block int frame = 0;
	dispatch_source_set_event_handler(gTimer, ^{
		CVPixelBufferRef pb = NULL;
		CVReturn r = CVPixelBufferPoolCreatePixelBuffer(NULL, gPool, &pb);
		if (r != kCVReturnSuccess) {
			gLastStatus = r;
			return;
		}
		fillPattern(pb, frame);
		NSDictionary *props = nil;
		if (gForceKey) {
			gForceKey = 0;
			props = @{(__bridge NSString *)kVTEncodeFrameOptionKey_ForceKeyFrame : @YES};
		}
		OSStatus st = VTCompressionSessionEncodeFrame(gSession, pb, CMTimeMake(frame, gFps), kCMTimeInvalid,
		                                              (__bridge CFDictionaryRef)props, NULL, NULL);
		if (st != noErr) {
			gLastStatus = st;
		}
		CVPixelBufferRelease(pb);
		frame++;
	});
	dispatch_resume(gTimer);
	return 0;
}

void spk_source_stop(void) {
	if (gTimer != nil) {
		dispatch_source_cancel(gTimer);
		gTimer = nil;
	}
	if (gQueue != nil) {
		dispatch_sync(gQueue, ^{});
		gQueue = nil;
	}
	if (gSession != NULL) {
		VTCompressionSessionCompleteFrames(gSession, kCMTimeInvalid);
		VTCompressionSessionInvalidate(gSession);
		CFRelease(gSession);
		gSession = NULL;
	}
	if (gPool != NULL) {
		CVPixelBufferPoolRelease(gPool);
		gPool = NULL;
	}
}
