// audio_darwin.m: the Mac's system audio as Opus. ScreenCaptureKit hands the
// stream's audio buffers to pc_audio_handle (media_darwin.m adds the audio
// output to the one SCStream, so there is no microphone and no input device);
// they are interleaved, cut into 960-frame (20 ms) blocks and encoded with
// AudioToolbox's AudioConverter, one raw Opus packet per block. Each packet
// crosses into Go through goScreencapAudioPacket.
//
// Compiled by cgo with ARC on (see media_darwin.go). Like every ObjC entry
// point here, a failure is reported as an error and never as an exception that
// would end the daemon. Nothing here ever touches the picture.

#import <Foundation/Foundation.h>
#import <CoreMedia/CoreMedia.h>
#import <AudioToolbox/AudioToolbox.h>
#import <pthread.h>
#import <stdlib.h>
#import <string.h>

// Exported from audio_darwin.go. The handle is a runtime/cgo.Handle.
extern void goScreencapAudioPacket(uintptr_t handle, void *data, int n);
extern void goScreencapAudioError(uintptr_t handle, int code, char *msg);

// Keep equal to screencap.OpusFrames, OpusRate and OpusBitrate; the Opus tests
// read the bit rate back, so a drift between the copies fails there.
enum { kPCOpusFrames = 960, kPCOpusRate = 48000, kPCOpusBitrate = 64000, kPCOpusFallbackBitrate = 96000, kPCOpusMaxPacket = 4096 };
// AudioConverter's way of saying the input callback has no more data.
static const OSStatus kPCNoMoreInput = 'nmor';
// Codes of our own, outside the OSStatus range we pass through.
enum { kPCAudioFormatErr = -91001, kPCAudioBufferErr = -91002, kPCAudioExceptionErr = -91003 };

typedef struct pc_opus {
	AudioConverterRef conv;
	float *acc; // PCM waiting for a full block, interleaved stereo
	size_t accFrames, accCap;
	int bitrate; // read back from the converter
} pc_opus;

typedef struct {
	const float *pcm;
	UInt32 frames;
	BOOL used;
} pcFeed;

static OSStatus pcOpusInput(AudioConverterRef c, UInt32 *ioN, AudioBufferList *ioData, AudioStreamPacketDescription **desc,
                            void *ud) {
	pcFeed *f = ud;
	if (f->used) {
		*ioN = 0;
		return kPCNoMoreInput;
	}
	f->used = YES;
	ioData->mNumberBuffers = 1;
	ioData->mBuffers[0].mNumberChannels = 2;
	ioData->mBuffers[0].mDataByteSize = f->frames * 8;
	ioData->mBuffers[0].mData = (void *)f->pcm;
	*ioN = f->frames;
	return noErr;
}

void pc_opus_free(pc_opus *o) {
	if (o == NULL) return;
	if (o->conv != NULL) AudioConverterDispose(o->conv);
	free(o->acc);
	free(o);
}

// A converter from interleaved float32 stereo LPCM to Opus at `bitrate` bits per
// second, or the 96000 fallback when the converter refuses it. NULL on failure
// with the OSStatus in *status.
pc_opus *pc_opus_new(int bitrate, int *status) {
	pc_opus *o = calloc(1, sizeof *o);
	if (o == NULL) {
		*status = kAudioConverterErr_UnspecifiedError;
		return NULL;
	}
	AudioStreamBasicDescription in = {0}, out = {0};
	in.mSampleRate = kPCOpusRate;
	in.mFormatID = kAudioFormatLinearPCM;
	in.mFormatFlags = kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked;
	in.mBytesPerPacket = 8;
	in.mFramesPerPacket = 1;
	in.mBytesPerFrame = 8;
	in.mChannelsPerFrame = 2;
	in.mBitsPerChannel = 32;
	out.mSampleRate = kPCOpusRate;
	out.mFormatID = kAudioFormatOpus;
	out.mChannelsPerFrame = 2;
	out.mFramesPerPacket = kPCOpusFrames;
	OSStatus st = AudioConverterNew(&in, &out, &o->conv);
	if (st != noErr) {
		o->conv = NULL;
		*status = (int)st;
		pc_opus_free(o);
		return NULL;
	}
	UInt32 br = (UInt32)bitrate;
	st = AudioConverterSetProperty(o->conv, kAudioConverterEncodeBitRate, sizeof br, &br);
	if (st != noErr && bitrate != kPCOpusFallbackBitrate) {
		br = kPCOpusFallbackBitrate;
		st = AudioConverterSetProperty(o->conv, kAudioConverterEncodeBitRate, sizeof br, &br);
	}
	UInt32 got = 0, sz = sizeof got;
	if (st == noErr) st = AudioConverterGetProperty(o->conv, kAudioConverterEncodeBitRate, &sz, &got);
	if (st != noErr) {
		*status = (int)st;
		pc_opus_free(o);
		return NULL;
	}
	o->bitrate = (int)got;
	return o;
}

int pc_opus_bitrate(const pc_opus *o) { return o->bitrate; }

// Encodes one full block and hands its packet to Go. Returns 0 or an OSStatus.
static int pcOpusBlock(pc_opus *o, const float *pcm, uintptr_t handle) {
	pcFeed f = {pcm, kPCOpusFrames, NO};
	uint8_t out[kPCOpusMaxPacket];
	AudioStreamPacketDescription pd = {0};
	AudioBufferList abl;
	abl.mNumberBuffers = 1;
	abl.mBuffers[0].mNumberChannels = 0;
	abl.mBuffers[0].mDataByteSize = sizeof out;
	abl.mBuffers[0].mData = out;
	UInt32 n = 1;
	OSStatus st = AudioConverterFillComplexBuffer(o->conv, pcOpusInput, &f, &n, &abl, &pd);
	if (st != noErr && st != kPCNoMoreInput) return (int)st;
	if (n == 0 || pd.mDataByteSize == 0) return 0; // the converter wants more input before its next packet
	goScreencapAudioPacket(handle, out + pd.mStartOffset, (int)pd.mDataByteSize);
	return 0;
}

// Appends interleaved stereo PCM and encodes every full 960-frame block; a
// partial block waits for the next call and is never encoded short. Returns 0 or
// the first OSStatus that failed.
int pc_opus_encode(pc_opus *o, const float *pcm, size_t frames, uintptr_t handle) {
	if (o->accFrames + frames > o->accCap) {
		size_t cap = o->accFrames + frames + kPCOpusFrames;
		float *grown = realloc(o->acc, cap * 8);
		if (grown == NULL) return kAudioConverterErr_UnspecifiedError;
		o->acc = grown;
		o->accCap = cap;
	}
	memcpy(o->acc + o->accFrames * 2, pcm, frames * 8);
	o->accFrames += frames;
	int rc = 0;
	size_t off = 0;
	while (o->accFrames - off >= kPCOpusFrames) {
		int st = pcOpusBlock(o, o->acc + off * 2, handle);
		if (st != 0 && rc == 0) rc = st;
		off += kPCOpusFrames;
	}
	memmove(o->acc, o->acc + off * 2, (o->accFrames - off) * 8);
	o->accFrames -= off;
	return rc;
}

// Test hook: encodes synthetic PCM in buffers of `chunk` frames with a fresh
// encoder and reads its bit rate back. Packets reach Go through the handle, the
// way live ones do. Host-safe: AudioConverter on data we made, no capture.
int pc_opus_test(uintptr_t handle, const float *pcm, int frames, int chunk, int bitrate, int *bitrateOut) {
	@autoreleasepool {
		int st = 0;
		pc_opus *o = pc_opus_new(bitrate, &st);
		if (o == NULL) return st != 0 ? st : kPCAudioBufferErr;
		*bitrateOut = pc_opus_bitrate(o);
		int rc = 0;
		for (int at = 0; at < frames; at += chunk) {
			int n = frames - at < chunk ? frames - at : chunk;
			int e = pc_opus_encode(o, pcm + (size_t)at * 2, (size_t)n, handle);
			if (e != 0 && rc == 0) rc = e;
		}
		pc_opus_free(o);
		return rc;
	}
}

// The live encoder, one per capture run. gLock serialises the handler on its
// queue against pc_audio_end on the stop path.
static pthread_mutex_t gLock = PTHREAD_MUTEX_INITIALIZER;
static pc_opus *gEnc;
static uintptr_t gHandle;
static int gBad; // an error was reported and no good buffer has followed it

// Starts the encoder for one capture run. Returns 0 or an OSStatus; the caller
// then runs without audio and the picture is unaffected.
int pc_audio_begin(uintptr_t handle) {
	@autoreleasepool {
		int st = 0;
		pc_opus *o = pc_opus_new(kPCOpusBitrate, &st);
		if (o == NULL) return st != 0 ? st : kPCAudioBufferErr;
		pthread_mutex_lock(&gLock);
		pc_opus_free(gEnc);
		gEnc = o;
		gHandle = handle;
		gBad = 0;
		pthread_mutex_unlock(&gLock);
		return 0;
	}
}

// Releases the encoder. The caller has drained the audio queue, so no handler is
// running; the lock covers one that is anyway. Safe with nothing started.
void pc_audio_end(void) {
	pthread_mutex_lock(&gLock);
	pc_opus_free(gEnc);
	gEnc = NULL;
	pthread_mutex_unlock(&gLock);
}

// Interleaves one SCK buffer (float32 stereo at 48 kHz, planar or not) and feeds
// the encoder. Returns 0 or a code, with *why a fixed message.
static int pcAudioFeed(pc_opus *o, CMSampleBufferRef sb, uintptr_t h, const char **why) {
	CMFormatDescriptionRef fd = CMSampleBufferGetFormatDescription(sb);
	const AudioStreamBasicDescription *a = fd != NULL ? CMAudioFormatDescriptionGetStreamBasicDescription(fd) : NULL;
	if (a == NULL) {
		*why = "audio buffer has no stream description";
		return kPCAudioFormatErr;
	}
	BOOL isFloat = a->mFormatID == kAudioFormatLinearPCM && (a->mFormatFlags & kAudioFormatFlagIsFloat) && a->mBitsPerChannel == 32;
	if (!isFloat || a->mChannelsPerFrame != 2 || a->mSampleRate != kPCOpusRate) {
		*why = "audio buffer is not 48 kHz float32 stereo";
		return kPCAudioFormatErr;
	}
	BOOL planar = (a->mFormatFlags & kAudioFormatFlagIsNonInterleaved) != 0;
	size_t need = 0;
	CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sb, &need, NULL, 0, NULL, NULL, 0, NULL);
	AudioBufferList *abl = need > 0 ? malloc(need) : NULL;
	CMBlockBufferRef bb = NULL;
	OSStatus st = abl != NULL ? CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sb, NULL, abl, need, NULL, NULL, 0, &bb)
	                          : kCMSampleBufferError_ArrayTooSmall;
	size_t n = (size_t)CMSampleBufferGetNumSamples(sb);
	float *pcm = st == noErr && n > 0 ? malloc(n * 8) : NULL;
	int rc = 0;
	if (pcm == NULL) {
		*why = "could not read the audio buffer list";
		rc = kPCAudioBufferErr;
	} else if (planar ? abl->mNumberBuffers != 2 : abl->mNumberBuffers != 1) {
		*why = "unexpected audio buffer count for the stream format";
		rc = kPCAudioBufferErr;
	} else {
		const float *b0 = abl->mBuffers[0].mData;
		const float *b1 = planar ? abl->mBuffers[1].mData : NULL;
		for (size_t i = 0; i < n; i++) {
			pcm[2 * i] = planar ? b0[i] : b0[2 * i];
			pcm[2 * i + 1] = planar ? b1[i] : b0[2 * i + 1];
		}
		rc = pc_opus_encode(o, pcm, n, h);
		if (rc != 0) *why = "encoding captured audio failed";
	}
	free(pcm);
	free(abl);
	if (bb != NULL) CFRelease(bb);
	return rc;
}

// Called by the stream handler for every SCStreamOutputTypeAudio buffer, on the
// audio queue. Reports a failure once until a good buffer follows it.
void pc_audio_handle(CMSampleBufferRef sb) {
	@autoreleasepool {
		volatile BOOL locked = NO;
		@try {
			if (sb == NULL || !CMSampleBufferDataIsReady(sb)) return;
			pthread_mutex_lock(&gLock);
			locked = YES;
			if (gEnc != NULL) {
				const char *why = "audio failed";
				int rc = pcAudioFeed(gEnc, sb, gHandle, &why);
				if (rc == 0) {
					gBad = 0;
				} else if (!gBad) {
					gBad = 1;
					goScreencapAudioError(gHandle, rc, (char *)why);
				}
			}
			locked = NO;
			pthread_mutex_unlock(&gLock);
		} @catch (NSException *e) {
			if (!locked) pthread_mutex_lock(&gLock);
			if (!gBad && gEnc != NULL) {
				gBad = 1;
				goScreencapAudioError(gHandle, kPCAudioExceptionErr, "exception in the audio handler");
			}
			pthread_mutex_unlock(&gLock);
		}
	}
}
