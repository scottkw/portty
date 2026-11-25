// media_darwin.m: synthetic frame source and VideoToolbox H.264 encoder with
// Annex-B output for the desktop feasibility spike. Compiled by cgo with ARC on
// (see bridge_darwin.go). Each finished access unit is handed to Go once through
// spkOnAccessUnit; every OSStatus failure is returned to the caller and kept for
// spk_encoder_info so a probe never reports success it did not get.

#import <Foundation/Foundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <VideoToolbox/VideoToolbox.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ImageIO/ImageIO.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <AudioToolbox/AudioToolbox.h>
#import <mach/mach_time.h>
#import <pthread.h>

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
static int gBitrate;
static CMTime gLastPts;

// ScreenCaptureKit state. gStatLock guards gSt, gLat and gLatest; the frame
// handler, the VideoToolbox callback and the Go-facing readers all touch them.
// Statistics are kept after a stream stops so a probe can read them last.
enum { kLatRing = 4096 };
typedef struct {
	long complete, idle, blank, suspended, started, stopped, unknown;
	long ioSurfaceFrames;
	double firstFrameMs; // negative until the first frame of any status
	uint64_t startMach, stopMach, lastCompleteMach, lastRefreshMach, latestDisplayTime;
	int width, height, running, withAudio, mainMatched;
	int hasErr, errCode;
	char errDomain[96], errMsg[320];
	char pixelFormat[8], fallbackReason[320];
} sckStats;
static sckStats gSt;
static double gLat[kLatRing]; // encode latency in ms, a ring of the newest samples
static long gLatN;
static pthread_mutex_t gStatLock = PTHREAD_MUTEX_INITIALIZER;
static CVPixelBufferRef gLatest;
static SCStream *gStream;
static dispatch_queue_t gSCKQueue;
static dispatch_source_t gIdleTimer;
static char gPngErr[256];
// Returned to Go as C strings; each call replaces the previous result, which
// Go copies at once. One caller at a time, which the spike's probes are.
static NSData *gStatsData, *gListData, *gDispData;

static double machToMs(uint64_t t) {
	static mach_timebase_info_data_t tb;
	if (tb.denom == 0) mach_timebase_info(&tb);
	return (double)t * tb.numer / tb.denom / 1e6;
}

// Latency from SCStreamFrameInfoDisplayTime to the encoder's output callback.
static void recordLatency(uint64_t displayTime) {
	uint64_t now = mach_absolute_time();
	if (now < displayTime) return;
	pthread_mutex_lock(&gStatLock);
	gLat[gLatN++ % kLatRing] = machToMs(now - displayTime);
	pthread_mutex_unlock(&gStatLock);
}

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
	if (sourceFrameRefCon != NULL) {
		recordLatency((uint64_t)(uintptr_t)sourceFrameRefCon);
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
	gBitrate = bitrate;
	gLastPts = kCMTimeInvalid;
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

static void stopEncoder(void) {
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

static void stopSCK(void);

// Stops whichever source is running and the encoder. Safe to call twice.
void spk_source_stop(void) {
	stopSCK();
	if (gTimer != nil) {
		dispatch_source_cancel(gTimer);
		gTimer = nil;
	}
	if (gQueue != nil) {
		dispatch_sync(gQueue, ^{});
		gQueue = nil;
	}
	stopEncoder();
}

// ---------------------------------------------------------------------------
// ScreenCaptureKit source
// ---------------------------------------------------------------------------

// Returns a NUL-terminated copy of obj as JSON; the NSData is parked in *slot
// so the pointer stays valid until the next call for the same slot.
static const char *jsonC(id obj, NSData *__strong *slot) {
	NSError *e = nil;
	NSData *d = [NSJSONSerialization dataWithJSONObject:obj options:NSJSONWritingSortedKeys error:&e];
	if (d == nil) {
		NSDictionary *err = @{@"error" : @{@"domain" : e.domain ?: @"spike.json", @"code" : @(e.code),
		                                   @"message" : e.localizedDescription ?: @"json encode failed"}};
		d = [NSJSONSerialization dataWithJSONObject:err options:0 error:nil];
	}
	NSMutableData *m = [d mutableCopy];
	[m appendBytes:"" length:1];
	*slot = m;
	return (const char *)m.bytes;
}

static void copyStr(char *dst, size_t n, NSString *src) {
	snprintf(dst, n, "%s", src.UTF8String ?: "");
}

// Keeps the first error of a run; every SCError code is kept verbatim.
static void recordError(NSError *e) {
	pthread_mutex_lock(&gStatLock);
	gSt.hasErr = 1;
	gSt.errCode = (int)e.code;
	copyStr(gSt.errDomain, sizeof gSt.errDomain, e.domain);
	copyStr(gSt.errMsg, sizeof gSt.errMsg, e.localizedDescription);
	pthread_mutex_unlock(&gStatLock);
}

static void recordErrorText(NSString *domain, int code, NSString *msg) {
	pthread_mutex_lock(&gStatLock);
	gSt.hasErr = 1;
	gSt.errCode = code;
	copyStr(gSt.errDomain, sizeof gSt.errDomain, domain);
	copyStr(gSt.errMsg, sizeof gSt.errMsg, msg);
	pthread_mutex_unlock(&gStatLock);
}

// Synchronous SCShareableContent; the handler runs on a system queue, so this
// must not be called from a queue SCK needs. Go calls it from its own threads.
static SCShareableContent *fetchContent(NSError **err) {
	__block SCShareableContent *content = nil;
	__block NSError *e = nil;
	dispatch_semaphore_t sem = dispatch_semaphore_create(0);
	[SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *c, NSError *error) {
		content = c;
		e = error;
		dispatch_semaphore_signal(sem);
	}];
	if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 15 * NSEC_PER_SEC)) != 0) {
		*err = [NSError errorWithDomain:@"spike.sck" code:-1
		                       userInfo:@{NSLocalizedDescriptionKey : @"SCShareableContent timed out after 15 s"}];
		return nil;
	}
	if (content == nil && e == nil) {
		e = [NSError errorWithDomain:@"spike.sck" code:-2
		                    userInfo:@{NSLocalizedDescriptionKey : @"SCShareableContent returned neither content nor an error"}];
	}
	*err = e;
	return content;
}

// Pixel size of a display: points times the filter's point-to-pixel scale.
static void displayPixels(SCDisplay *d, double *w, double *h) {
	SCContentFilter *f = [[SCContentFilter alloc] initWithDisplay:d excludingWindows:@[]];
	*w = d.width * f.pointPixelScale;
	*h = d.height * f.pointPixelScale;
}

const char *spk_displays(void) {
	NSError *err = nil;
	SCShareableContent *c = fetchContent(&err);
	if (c == nil) {
		return jsonC(@{@"error" : @{@"domain" : err.domain ?: @"spike.sck", @"code" : @(err.code),
		                            @"message" : err.localizedDescription ?: @"no content"}},
		             &gDispData);
	}
	NSMutableArray *out = [NSMutableArray array];
	for (SCDisplay *d in c.displays) {
		double w, h;
		displayPixels(d, &w, &h);
		[out addObject:@{@"id" : @(d.displayID), @"width" : @((int)lround(w)), @"height" : @((int)lround(h)),
		                 @"pointWidth" : @((int)d.width), @"pointHeight" : @((int)d.height),
		                 @"main" : @(d.displayID == CGMainDisplayID())}];
	}
	return jsonC(out, &gDispData);
}

const char *spk_encoder_list(void) {
	CFArrayRef list = NULL;
	OSStatus st = VTCopyVideoEncoderList(NULL, &list);
	if (st != noErr || list == NULL) {
		return jsonC(@{@"error" : @{@"domain" : @"OSStatus", @"code" : @(st), @"message" : @"VTCopyVideoEncoderList failed"}},
		             &gListData);
	}
	NSMutableArray *out = [NSMutableArray array];
	for (NSDictionary *d in (__bridge NSArray *)list) {
		NSString *eid = d[(__bridge NSString *)kVTVideoEncoderList_EncoderID];
		NSString *name = d[(__bridge NSString *)kVTVideoEncoderList_EncoderName];
		NSString *codec = d[(__bridge NSString *)kVTVideoEncoderList_CodecName];
		NSNumber *hw = d[(__bridge NSString *)kVTVideoEncoderList_IsHardwareAccelerated];
		[out addObject:@{@"id" : eid ?: @"", @"name" : name ?: @"", @"codec" : codec ?: @"",
		                 @"hardware" : @(hw.boolValue)}];
	}
	CFRelease(list);
	return jsonC(out, &gListData);
}

static void encodeBuffer(CVPixelBufferRef pb, CMTime pts, uint64_t displayTime, BOOL forceKey) {
	if (gSession == NULL) return;
	if (CMTIME_IS_VALID(gLastPts) && CMTimeCompare(pts, gLastPts) <= 0) {
		pts = CMTimeAdd(gLastPts, CMTimeMake(1, 1000));
	}
	gLastPts = pts;
	NSDictionary *props = nil;
	if (forceKey || gForceKey) {
		gForceKey = 0;
		props = @{(__bridge NSString *)kVTEncodeFrameOptionKey_ForceKeyFrame : @YES};
	}
	OSStatus st = VTCompressionSessionEncodeFrame(gSession, pb, pts, kCMTimeInvalid, (__bridge CFDictionaryRef)props,
	                                              (void *)(uintptr_t)displayTime, NULL);
	if (st != noErr) {
		gLastStatus = st;
	}
}

// Re-encodes the newest Complete frame (RESEARCH Pitfall 3: frames arrive only
// on change). Runs on gSCKQueue so it never races the frame handler.
static void encodeCached(BOOL forceKey) {
	pthread_mutex_lock(&gStatLock);
	CVPixelBufferRef pb = gLatest != NULL ? CVPixelBufferRetain(gLatest) : NULL;
	gSt.lastRefreshMach = mach_absolute_time();
	pthread_mutex_unlock(&gStatLock);
	if (pb == NULL) {
		if (forceKey) gForceKey = 1; // nothing cached yet: the next real frame becomes the IDR
		return;
	}
	encodeBuffer(pb, CMClockGetTime(CMClockGetHostTimeClock()), 0, forceKey);
	CVPixelBufferRelease(pb);
}

// Called on PLI/FIR and on viewer join. With a stream running it re-sends the
// cached picture as an IDR; otherwise the next synthetic frame becomes one.
void spk_request_refresh(void) {
	dispatch_queue_t q = gSCKQueue;
	if (q == nil) {
		gForceKey = 1;
		return;
	}
	dispatch_async(q, ^{
		encodeCached(YES);
	});
}

// Opus through AudioToolbox's AudioConverter (C7: no libopus unless this fails).
// 48 kHz float32 interleaved stereo in, one 960-frame (20 ms) raw Opus packet out
// per AudioConverterFillComplexBuffer call. Used by spk_opus_sine and by the
// ScreenCaptureKit audio output; each user owns one spkOpus.
enum { kOpusFrames = 960, kOpusRate = 48000, kOpusBitrate = 96000, kOpusMaxPacket = 4096 };
static const OSStatus kOpusNoMoreInput = 'nmor';

typedef struct {
	AudioConverterRef conv;
	float *acc; // PCM waiting for a full block, interleaved stereo
	size_t accFrames, accCap;
	long packets, bytes, frames, errors;
	int firstPacketBytes, firstToc, primeFrames, framesPerPacket;
	OSStatus lastStatus;
} spkOpus;

typedef struct {
	const float *pcm;
	UInt32 frames;
	BOOL used;
} opusFeedCtx;

static OSStatus opusInput(AudioConverterRef c, UInt32 *ioN, AudioBufferList *ioData,
                          AudioStreamPacketDescription **desc, void *ud) {
	opusFeedCtx *f = ud;
	if (f->used) {
		*ioN = 0;
		return kOpusNoMoreInput;
	}
	f->used = YES;
	ioData->mNumberBuffers = 1;
	ioData->mBuffers[0].mNumberChannels = 2;
	ioData->mBuffers[0].mDataByteSize = f->frames * 8;
	ioData->mBuffers[0].mData = (void *)f->pcm;
	*ioN = f->frames;
	return noErr;
}

static OSStatus opusCreate(spkOpus *o) {
	memset(o, 0, sizeof *o);
	AudioStreamBasicDescription in = {0}, out = {0};
	in.mSampleRate = kOpusRate;
	in.mFormatID = kAudioFormatLinearPCM;
	in.mFormatFlags = kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked;
	in.mBytesPerPacket = 8;
	in.mFramesPerPacket = 1;
	in.mBytesPerFrame = 8;
	in.mChannelsPerFrame = 2;
	in.mBitsPerChannel = 32;
	out.mSampleRate = kOpusRate;
	out.mFormatID = kAudioFormatOpus;
	out.mChannelsPerFrame = 2;
	out.mFramesPerPacket = kOpusFrames;
	OSStatus st = AudioConverterNew(&in, &out, &o->conv);
	if (st != noErr) {
		o->conv = NULL;
		return st;
	}
	UInt32 br = kOpusBitrate;
	if ((st = AudioConverterSetProperty(o->conv, kAudioConverterEncodeBitRate, sizeof br, &br)) != noErr) return st;
	// Read back what the converter really made; nothing is assumed.
	AudioStreamBasicDescription got = {0};
	UInt32 sz = sizeof got;
	if ((st = AudioConverterGetProperty(o->conv, kAudioConverterCurrentOutputStreamDescription, &sz, &got)) != noErr) return st;
	o->framesPerPacket = (int)got.mFramesPerPacket;
	AudioConverterPrimeInfo pi = {0};
	sz = sizeof pi;
	if (AudioConverterGetProperty(o->conv, kAudioConverterPrimeInfo, &sz, &pi) == noErr) o->primeFrames = (int)pi.leadingFrames;
	return noErr;
}

static void opusDispose(spkOpus *o) {
	if (o->conv != NULL) AudioConverterDispose(o->conv);
	o->conv = NULL;
	free(o->acc);
	o->acc = NULL;
	o->accFrames = o->accCap = 0;
}

static void opusEncodeBlock(spkOpus *o, const float *pcm) {
	opusFeedCtx f = {pcm, kOpusFrames, NO};
	uint8_t out[kOpusMaxPacket];
	AudioStreamPacketDescription pd = {0};
	AudioBufferList abl;
	abl.mNumberBuffers = 1;
	abl.mBuffers[0].mNumberChannels = 0;
	abl.mBuffers[0].mDataByteSize = sizeof out;
	abl.mBuffers[0].mData = out;
	UInt32 n = 1;
	OSStatus st = AudioConverterFillComplexBuffer(o->conv, opusInput, &f, &n, &abl, &pd);
	if (st != noErr && st != kOpusNoMoreInput) {
		o->lastStatus = st;
		o->errors++;
		return;
	}
	if (n == 0) return; // the converter wants more input before its next packet
	if (o->packets == 0) {
		o->firstPacketBytes = (int)pd.mDataByteSize;
		o->firstToc = pd.mDataByteSize > 0 ? out[pd.mStartOffset] : -1;
	}
	o->packets++;
	o->bytes += pd.mDataByteSize;
}

// Appends PCM and encodes every full 960-frame block.
static void opusFeed(spkOpus *o, const float *pcm, size_t frames) {
	o->frames += (long)frames;
	if (o->accFrames + frames > o->accCap) {
		size_t cap = o->accFrames + frames + kOpusFrames;
		float *grown = realloc(o->acc, cap * 8);
		if (grown == NULL) {
			o->errors++;
			o->lastStatus = kAudioConverterErr_UnspecifiedError;
			return;
		}
		o->acc = grown;
		o->accCap = cap;
	}
	memcpy(o->acc + o->accFrames * 2, pcm, frames * 8);
	o->accFrames += frames;
	size_t off = 0;
	while (o->accFrames - off >= kOpusFrames) {
		opusEncodeBlock(o, o->acc + off * 2);
		off += kOpusFrames;
	}
	memmove(o->acc, o->acc + off * 2, (o->accFrames - off) * 8);
	o->accFrames -= off;
}

static NSMutableDictionary *opusDict(const spkOpus *o) {
	double secs = (double)o->frames / kOpusRate;
	NSMutableDictionary *d = [NSMutableDictionary dictionary];
	d[@"packets"] = @(o->packets);
	d[@"framesPerPacket"] = @(o->framesPerPacket);
	d[@"primeFrames"] = @(o->primeFrames);
	d[@"firstPacketBytes"] = @(o->firstPacketBytes);
	d[@"firstPacketToc"] = @(o->firstToc);
	d[@"totalBytes"] = @(o->bytes);
	d[@"inputFrames"] = @(o->frames);
	d[@"avgKbps"] = @(secs > 0 ? o->bytes * 8.0 / secs / 1000.0 : 0.0);
	d[@"osStatus"] = @((int)o->lastStatus);
	d[@"encodeErrors"] = @(o->errors);
	return d;
}

static NSData *gOpusData, *gAudData;

// Encodes a generated 440 Hz sine for `seconds` and returns the pinned JSON
// (`packets`, `framesPerPacket` always present). No TCC or SCK call.
const char *spk_opus_sine(int seconds) {
	spkOpus o;
	OSStatus st = opusCreate(&o);
	if (st != noErr) {
		NSMutableDictionary *d = opusDict(&o);
		d[@"osStatus"] = @((int)st);
		d[@"error"] = @"AudioConverter setup for Opus failed";
		opusDispose(&o);
		return jsonC(d, &gOpusData);
	}
	float block[kOpusFrames * 2];
	double phase = 0, step = 2.0 * M_PI * 440.0 / kOpusRate;
	long blocks = (long)seconds * kOpusRate / kOpusFrames;
	for (long b = 0; b < blocks; b++) {
		for (int i = 0; i < kOpusFrames; i++) {
			float v = (float)(0.5 * sin(phase));
			phase += step;
			block[2 * i] = v;
			block[2 * i + 1] = v;
		}
		opusFeed(&o, block, kOpusFrames);
	}
	NSMutableDictionary *d = opusDict(&o);
	opusDispose(&o);
	return jsonC(d, &gOpusData);
}

// SCK audio statistics. gAudLock guards gAud and gAudOpus; the audio handler and
// the Go-facing reader both touch them. Kept after a stream stops.
typedef struct {
	long buffers, frames, errors;
	double firstBufferMs; // negative until the first buffer
	double peak;          // largest absolute sample seen; 0 means only silence
	uint64_t startMach;
	int fmtChannels, fmtBits, fmtNonInterleaved;
	double fmtRate;
	char err[160];
} audioStats;
static audioStats gAud;
static spkOpus gAudOpus;
static pthread_mutex_t gAudLock = PTHREAD_MUTEX_INITIALIZER;
static dispatch_queue_t gAudQueue;

static void audioFail(const char *msg) {
	gAud.errors++;
	snprintf(gAud.err, sizeof gAud.err, "%s", msg);
}

// Turns one SCK audio buffer into interleaved float32 stereo and feeds the encoder.
static void handleAudio(CMSampleBufferRef sb) {
	uint64_t now = mach_absolute_time();
	CMFormatDescriptionRef fd = CMSampleBufferGetFormatDescription(sb);
	const AudioStreamBasicDescription *a = fd != NULL ? CMAudioFormatDescriptionGetStreamBasicDescription(fd) : NULL;
	pthread_mutex_lock(&gAudLock);
	gAud.buffers++;
	if (gAud.firstBufferMs < 0) gAud.firstBufferMs = machToMs(now - gAud.startMach);
	if (a == NULL) {
		audioFail("audio sample buffer has no stream description");
		pthread_mutex_unlock(&gAudLock);
		return;
	}
	gAud.fmtRate = a->mSampleRate;
	gAud.fmtChannels = (int)a->mChannelsPerFrame;
	gAud.fmtBits = (int)a->mBitsPerChannel;
	gAud.fmtNonInterleaved = (a->mFormatFlags & kAudioFormatFlagIsNonInterleaved) != 0;
	BOOL isFloat = a->mFormatID == kAudioFormatLinearPCM && (a->mFormatFlags & kAudioFormatFlagIsFloat) && a->mBitsPerChannel == 32;
	if (!isFloat || a->mChannelsPerFrame != 2 || a->mSampleRate != kOpusRate) {
		audioFail("audio buffer is not 48 kHz float32 stereo");
		pthread_mutex_unlock(&gAudLock);
		return;
	}
	size_t need = 0;
	CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sb, &need, NULL, 0, NULL, NULL, 0, NULL);
	AudioBufferList *abl = need > 0 ? malloc(need) : NULL;
	CMBlockBufferRef bb = NULL;
	OSStatus st = abl != NULL ? CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sb, NULL, abl, need, NULL, NULL, 0, &bb)
	                          : kCMSampleBufferError_ArrayTooSmall;
	size_t n = (size_t)CMSampleBufferGetNumSamples(sb);
	float *pcm = st == noErr && n > 0 ? malloc(n * 8) : NULL;
	if (pcm == NULL) {
		audioFail("could not read the audio buffer list");
		pthread_mutex_unlock(&gAudLock);
		free(abl);
		if (bb != NULL) CFRelease(bb);
		return;
	}
	BOOL planar = gAud.fmtNonInterleaved;
	BOOL shapeOk = planar ? abl->mNumberBuffers == 2 : abl->mNumberBuffers == 1;
	if (!shapeOk) {
		audioFail("unexpected audio buffer count for the stream format");
	} else {
		for (size_t i = 0; i < n; i++) {
			float l = planar ? ((const float *)abl->mBuffers[0].mData)[i] : ((const float *)abl->mBuffers[0].mData)[2 * i];
			float r = planar ? ((const float *)abl->mBuffers[1].mData)[i] : ((const float *)abl->mBuffers[0].mData)[2 * i + 1];
			pcm[2 * i] = l;
			pcm[2 * i + 1] = r;
			if (fabsf(l) > gAud.peak) gAud.peak = fabsf(l);
			if (fabsf(r) > gAud.peak) gAud.peak = fabsf(r);
		}
		gAud.frames += (long)n;
		if (gAudOpus.conv != NULL) opusFeed(&gAudOpus, pcm, n);
	}
	pthread_mutex_unlock(&gAudLock);
	free(pcm);
	free(abl);
	if (bb != NULL) CFRelease(bb);
}

// Fresh audio state for one capture; creates the encoder when audio is wanted.
static OSStatus audioReset(int withAudio) {
	pthread_mutex_lock(&gAudLock);
	opusDispose(&gAudOpus);
	memset(&gAud, 0, sizeof gAud);
	memset(&gAudOpus, 0, sizeof gAudOpus);
	gAud.firstBufferMs = -1;
	OSStatus st = noErr;
	if (withAudio) {
		gAud.startMach = mach_absolute_time();
		st = opusCreate(&gAudOpus);
		if (st != noErr) audioFail("AudioConverter setup for Opus failed");
	}
	pthread_mutex_unlock(&gAudLock);
	return st;
}

// Pinned `audio` object: buffers received, frames, packets out, bytes, kbps,
// first-buffer ms, errors, plus the peak sample and the buffer format seen.
const char *spk_audio_stats(void) {
	pthread_mutex_lock(&gAudLock);
	NSMutableDictionary *o = opusDict(&gAudOpus);
	o[@"audioBuffers"] = @(gAud.buffers);
	o[@"frames"] = @(gAud.frames);
	o[@"kbps"] = o[@"avgKbps"];
	[o removeObjectForKey:@"avgKbps"];
	if (gAud.firstBufferMs >= 0) o[@"firstBufferMs"] = @(gAud.firstBufferMs);
	o[@"peak"] = @(gAud.peak);
	o[@"errors"] = @(gAud.errors);
	if (gAud.err[0] != 0) o[@"lastError"] = @(gAud.err);
	if (gAud.buffers > 0) {
		o[@"bufferFormat"] = @{@"sampleRate" : @(gAud.fmtRate), @"channels" : @(gAud.fmtChannels), @"bits" : @(gAud.fmtBits),
		                       @"nonInterleaved" : @(gAud.fmtNonInterleaved != 0)};
	}
	pthread_mutex_unlock(&gAudLock);
	return jsonC(o, &gAudData);
}

@interface SpkSCKHandler : NSObject <SCStreamOutput, SCStreamDelegate>
@end

@implementation SpkSCKHandler
- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sb ofType:(SCStreamOutputType)type {
	if (sb == NULL) return;
	if (type == SCStreamOutputTypeAudio) {
		handleAudio(sb);
		return;
	}
	if (type != SCStreamOutputTypeScreen) return;
	CFArrayRef atts = CMSampleBufferGetSampleAttachmentsArray(sb, false);
	NSDictionary *d = (atts != NULL && CFArrayGetCount(atts) > 0) ? (__bridge NSDictionary *)CFArrayGetValueAtIndex(atts, 0) : nil;
	NSNumber *statusNum = d[SCStreamFrameInfoStatus];
	uint64_t now = mach_absolute_time();
	pthread_mutex_lock(&gStatLock);
	if (gSt.firstFrameMs < 0) gSt.firstFrameMs = machToMs(now - gSt.startMach);
	if (statusNum == nil) {
		gSt.unknown++;
		pthread_mutex_unlock(&gStatLock);
		return;
	}
	switch ((SCFrameStatus)statusNum.integerValue) {
	case SCFrameStatusComplete: gSt.complete++; break;
	case SCFrameStatusIdle: gSt.idle++; break;
	case SCFrameStatusBlank: gSt.blank++; break;
	case SCFrameStatusSuspended: gSt.suspended++; break;
	case SCFrameStatusStarted: gSt.started++; break;
	case SCFrameStatusStopped: gSt.stopped++; break;
	default: gSt.unknown++; break;
	}
	BOOL complete = statusNum.integerValue == SCFrameStatusComplete;
	pthread_mutex_unlock(&gStatLock);
	if (!complete) return;

	CVPixelBufferRef pb = CMSampleBufferGetImageBuffer(sb);
	if (pb == NULL) return;
	uint64_t displayTime = [d[SCStreamFrameInfoDisplayTime] unsignedLongLongValue];
	pthread_mutex_lock(&gStatLock);
	if (CVPixelBufferGetIOSurface(pb) != NULL) gSt.ioSurfaceFrames++;
	gSt.lastCompleteMach = now;
	gSt.latestDisplayTime = displayTime;
	if (gLatest != NULL) CVPixelBufferRelease(gLatest);
	gLatest = CVPixelBufferRetain(pb);
	pthread_mutex_unlock(&gStatLock);
	encodeBuffer(pb, CMSampleBufferGetPresentationTimeStamp(sb), displayTime, NO);
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
	recordError(error);
	pthread_mutex_lock(&gStatLock);
	gSt.running = 0;
	gSt.stopMach = mach_absolute_time();
	pthread_mutex_unlock(&gStatLock);
}
@end

static SpkSCKHandler *gHandler;

// Builds and starts one stream; returns the NSError on failure.
static NSError *startStream(SCContentFilter *filter, SCStreamConfiguration *cfg) {
	NSError *err = nil;
	gStream = [[SCStream alloc] initWithFilter:filter configuration:cfg delegate:gHandler];
	if (![gStream addStreamOutput:gHandler type:SCStreamOutputTypeScreen sampleHandlerQueue:gSCKQueue error:&err]) {
		gStream = nil;
		return err;
	}
	if (gAudQueue != nil && ![gStream addStreamOutput:gHandler type:SCStreamOutputTypeAudio sampleHandlerQueue:gAudQueue error:&err]) {
		gStream = nil;
		return err;
	}
	__block NSError *startErr = nil;
	dispatch_semaphore_t sem = dispatch_semaphore_create(0);
	[gStream startCaptureWithCompletionHandler:^(NSError *e) {
		startErr = e;
		dispatch_semaphore_signal(sem);
	}];
	if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 20 * NSEC_PER_SEC)) != 0) {
		gStream = nil;
		return [NSError errorWithDomain:@"spike.sck" code:-3
		                       userInfo:@{NSLocalizedDescriptionKey : @"startCapture timed out after 20 s"}];
	}
	if (startErr != nil) gStream = nil;
	return startErr;
}

static int failStart(NSError *e) {
	recordError(e);
	return e.code != 0 ? (int)e.code : -1;
}

// Captures the main display (RESEARCH "SCK stream"): 420v first, BGRA only as
// the recorded fallback, long edge capped at widthCap, fps frames per second.
// The encoder is (re)created at the real capture size here, so callers need
// not know the display size before starting.
int spk_sck_start(int widthCap, int fps, int withAudio) {
	stopSCK();
	pthread_mutex_lock(&gStatLock);
	memset(&gSt, 0, sizeof gSt);
	gSt.firstFrameMs = -1;
	gSt.withAudio = withAudio;
	gLatN = 0;
	if (gLatest != NULL) {
		CVPixelBufferRelease(gLatest);
		gLatest = NULL;
	}
	pthread_mutex_unlock(&gStatLock);
	OSStatus audioSt = audioReset(withAudio);
	if (audioSt != noErr) {
		recordErrorText(@"OSStatus", (int)audioSt, @"AudioConverter setup for Opus failed");
		return (int)audioSt;
	}

	CGMainDisplayID(); // connect to the window server before SCK asks
	NSError *err = nil;
	SCShareableContent *content = fetchContent(&err);
	if (content == nil) return failStart(err);
	SCDisplay *display = nil;
	for (SCDisplay *d in content.displays) {
		if (d.displayID == CGMainDisplayID()) display = d;
	}
	gSt.mainMatched = display != nil;
	if (display == nil) display = content.displays.firstObject;
	if (display == nil) {
		recordErrorText(@"spike.sck", -4, @"SCShareableContent lists no display");
		return -4;
	}
	double pw, ph;
	displayPixels(display, &pw, &ph);
	double longEdge = pw > ph ? pw : ph;
	double f = longEdge > widthCap ? widthCap / longEdge : 1.0;
	int w = ((int)lround(pw * f)) & ~1, h = ((int)lround(ph * f)) & ~1;
	gSt.width = w;
	gSt.height = h;

	// A fresh encoder at the capture size when the current one does not match.
	if (gSession == NULL || gWidth != w || gHeight != h) {
		stopEncoder();
		int rc = spk_encoder_start(w, h, fps, gBitrate > 0 ? gBitrate : 8000000);
		if (rc != 0) {
			recordErrorText(@"OSStatus", rc, @"spk_encoder_start failed at the capture size");
			return rc;
		}
	}

	gHandler = [[SpkSCKHandler alloc] init];
	gSCKQueue = dispatch_queue_create("spike.sck", DISPATCH_QUEUE_SERIAL);
	gAudQueue = withAudio ? dispatch_queue_create("spike.sck.audio", DISPATCH_QUEUE_SERIAL) : nil;
	SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
	SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
	cfg.width = (size_t)w;
	cfg.height = (size_t)h;
	cfg.minimumFrameInterval = CMTimeMake(1, fps);
	cfg.pixelFormat = kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange;
	cfg.queueDepth = 5;
	cfg.showsCursor = YES;
	cfg.colorMatrix = kCGDisplayStreamYCbCrMatrix_ITU_R_709_2;
	if (withAudio) {
		cfg.capturesAudio = YES;
		cfg.sampleRate = kOpusRate;
		cfg.channelCount = 2;
		cfg.excludesCurrentProcessAudio = YES;
	}
	pthread_mutex_lock(&gStatLock);
	snprintf(gSt.pixelFormat, sizeof gSt.pixelFormat, "420v");
	gSt.startMach = mach_absolute_time();
	pthread_mutex_unlock(&gStatLock);
	err = startStream(filter, cfg);
	if (err != nil) {
		// Let VideoToolbox convert if the 420v request itself is refused.
		pthread_mutex_lock(&gStatLock);
		copyStr(gSt.fallbackReason, sizeof gSt.fallbackReason,
		        [NSString stringWithFormat:@"420v start failed: %@ %ld %@", err.domain, (long)err.code, err.localizedDescription]);
		snprintf(gSt.pixelFormat, sizeof gSt.pixelFormat, "BGRA");
		pthread_mutex_unlock(&gStatLock);
		cfg.pixelFormat = kCVPixelFormatType_32BGRA;
		err = startStream(filter, cfg);
		if (err != nil) {
			gSCKQueue = nil;
			gAudQueue = nil;
			return failStart(err);
		}
	}
	pthread_mutex_lock(&gStatLock);
	gSt.startMach = mach_absolute_time();
	gSt.running = 1;
	pthread_mutex_unlock(&gStatLock);

	// Idle keepalive: a still screen yields no Complete frames, so after 2 s
	// without one the cached picture is re-sent as an IDR.
	gIdleTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, gSCKQueue);
	dispatch_source_set_timer(gIdleTimer, dispatch_time(DISPATCH_TIME_NOW, 500 * NSEC_PER_MSEC), 500 * NSEC_PER_MSEC,
	                          50 * NSEC_PER_MSEC);
	dispatch_source_set_event_handler(gIdleTimer, ^{
		pthread_mutex_lock(&gStatLock);
		uint64_t last = gSt.lastCompleteMach > gSt.lastRefreshMach ? gSt.lastCompleteMach : gSt.lastRefreshMach;
		BOOL stale = gLatest != NULL && last != 0 && machToMs(mach_absolute_time() - last) > 2000;
		pthread_mutex_unlock(&gStatLock);
		if (stale) encodeCached(YES);
	});
	dispatch_resume(gIdleTimer);
	return 0;
}

static void stopSCK(void) {
	if (gIdleTimer != nil) {
		dispatch_source_cancel(gIdleTimer);
		gIdleTimer = nil;
	}
	SCStream *s = gStream;
	if (s != nil) {
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		[s stopCaptureWithCompletionHandler:^(NSError *e) {
			if (e != nil) recordError(e);
			dispatch_semaphore_signal(sem);
		}];
		dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
		gStream = nil;
		pthread_mutex_lock(&gStatLock);
		gSt.running = 0;
		gSt.stopMach = mach_absolute_time();
		pthread_mutex_unlock(&gStatLock);
	}
	dispatch_queue_t q = gSCKQueue;
	if (q != nil) {
		dispatch_sync(q, ^{}); // let a handler already in flight finish before the encoder goes
		gSCKQueue = nil;
	}
	dispatch_queue_t aq = gAudQueue;
	if (aq != nil) {
		dispatch_sync(aq, ^{}); // same for an audio handler in flight
		gAudQueue = nil;
		pthread_mutex_lock(&gAudLock);
		opusDispose(&gAudOpus); // the counters stay for spk_audio_stats
		pthread_mutex_unlock(&gAudLock);
	}
	gHandler = nil;
}

static int cmpDouble(const void *a, const void *b) {
	double x = *(const double *)a, y = *(const double *)b;
	return x < y ? -1 : x > y;
}

// Pinned `capture` object: see the plan's "Pinned JSON keys". Keys of steps
// that did not happen are omitted, never null.
const char *spk_sck_stats(void) {
	pthread_mutex_lock(&gStatLock);
	sckStats s = gSt;
	long n = gLatN < kLatRing ? gLatN : kLatRing;
	double *lat = n > 0 ? malloc(n * sizeof(double)) : NULL;
	if (lat != NULL) memcpy(lat, gLat, n * sizeof(double));
	pthread_mutex_unlock(&gStatLock);

	NSMutableDictionary *o = [NSMutableDictionary dictionary];
	o[@"status"] = @{@"complete" : @(s.complete), @"idle" : @(s.idle), @"blank" : @(s.blank),
	                 @"suspended" : @(s.suspended), @"started" : @(s.started), @"stopped" : @(s.stopped),
	                 @"unknown" : @(s.unknown)};
	o[@"running"] = @(s.running != 0);
	if (s.firstFrameMs >= 0) o[@"firstFrameMs"] = @(s.firstFrameMs);
	if (s.startMach != 0) {
		uint64_t end = s.running ? mach_absolute_time() : (s.stopMach != 0 ? s.stopMach : mach_absolute_time());
		double secs = machToMs(end - s.startMach) / 1000.0;
		o[@"completeFps"] = @(secs > 0 ? s.complete / secs : 0.0);
	}
	if (lat != NULL) {
		double sum = 0;
		for (long i = 0; i < n; i++) sum += lat[i];
		qsort(lat, (size_t)n, sizeof(double), cmpDouble);
		long p95 = (long)ceil(0.95 * n) - 1;
		o[@"encodeLatencyMs"] = @{@"mean" : @(sum / n), @"p95" : @(lat[p95 < 0 ? 0 : p95]), @"max" : @(lat[n - 1])};
		free(lat);
	}
	if (s.pixelFormat[0] != 0) o[@"pixelFormat"] = @(s.pixelFormat);
	if (s.complete > 0) {
		o[@"ioSurface"] = @(s.ioSurfaceFrames == s.complete);
		o[@"ioSurfaceFrames"] = @(s.ioSurfaceFrames);
	}
	if (s.width > 0) {
		o[@"width"] = @(s.width);
		o[@"height"] = @(s.height);
		o[@"mainDisplayMatched"] = @(s.mainMatched != 0);
	}
	if (s.fallbackReason[0] != 0) o[@"fallbackReason"] = @(s.fallbackReason);
	if (s.latestDisplayTime != 0) o[@"latestDisplayTime"] = @(s.latestDisplayTime);
	o[@"withAudio"] = @(s.withAudio != 0);
	if (s.hasErr) {
		o[@"error"] = @{@"domain" : @(s.errDomain), @"code" : @(s.errCode), @"message" : @(s.errMsg)};
	}
	return jsonC(o, &gStatsData);
}

const char *spk_png_error(void) {
	return gPngErr;
}

// Writes the cached frame as PNG and returns its mean luma (0-255), or a
// negative number with the reason in spk_png_error.
double spk_dump_png(const char *path) {
	gPngErr[0] = 0;
	pthread_mutex_lock(&gStatLock);
	CVPixelBufferRef pb = gLatest != NULL ? CVPixelBufferRetain(gLatest) : NULL;
	pthread_mutex_unlock(&gStatLock);
	if (pb == NULL) {
		snprintf(gPngErr, sizeof gPngErr, "no Complete frame is cached");
		return -1;
	}
	CGImageRef img = NULL;
	OSStatus st = VTCreateCGImageFromCVPixelBuffer(pb, NULL, &img);
	CVPixelBufferRelease(pb);
	if (st != noErr || img == NULL) {
		snprintf(gPngErr, sizeof gPngErr, "VTCreateCGImageFromCVPixelBuffer: OSStatus %d", (int)st);
		return -2;
	}
	CFURLRef url = (__bridge CFURLRef)[NSURL fileURLWithPath:@(path)];
	CGImageDestinationRef dst = CGImageDestinationCreateWithURL(url, CFSTR("public.png"), 1, NULL);
	if (dst == NULL) {
		snprintf(gPngErr, sizeof gPngErr, "cannot create a PNG destination at %s", path);
		CGImageRelease(img);
		return -3;
	}
	CGImageDestinationAddImage(dst, img, NULL);
	bool ok = CGImageDestinationFinalize(dst);
	CFRelease(dst);
	if (!ok) {
		snprintf(gPngErr, sizeof gPngErr, "writing the PNG to %s failed", path);
		CGImageRelease(img);
		return -4;
	}
	// Mean luma over a 64x64 downscale, BT.709 weights on the rendered RGB.
	uint8_t px[64 * 64 * 4];
	CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
	CGContextRef ctx = CGBitmapContextCreate(px, 64, 64, 8, 64 * 4, cs, (CGBitmapInfo)kCGImageAlphaPremultipliedLast);
	CGColorSpaceRelease(cs);
	if (ctx == NULL) {
		snprintf(gPngErr, sizeof gPngErr, "cannot create the luma bitmap context");
		CGImageRelease(img);
		return -5;
	}
	CGContextDrawImage(ctx, CGRectMake(0, 0, 64, 64), img);
	CGContextRelease(ctx);
	CGImageRelease(img);
	double sum = 0;
	for (int i = 0; i < 64 * 64; i++) {
		sum += 0.2126 * px[i * 4] + 0.7152 * px[i * 4 + 1] + 0.0722 * px[i * 4 + 2];
	}
	return sum / (64 * 64);
}
