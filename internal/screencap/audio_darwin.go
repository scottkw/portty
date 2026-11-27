package screencap

// audio_darwin.go: the cgo side of audio_darwin.m. Encoded Opus packets and
// audio errors come in through two exports; the capture itself is the one
// SCStream in media_darwin.m. Live capture never runs under testing.Testing()
// (DisplaySource.Start refuses); the test hook below only runs AudioConverter on
// PCM the test made.

/*
#cgo darwin LDFLAGS: -framework AudioToolbox

#include <stdint.h>

int pc_opus_test(uintptr_t handle, const float *pcm, int frames, int chunk, int bitrate, int *bitrateOut);
*/
import "C"

import (
	"errors"
	"fmt"
	"log"
	"runtime/cgo"
	"testing"
	"unsafe"
)

//export goScreencapAudioPacket
func goScreencapAudioPacket(h C.uintptr_t, data unsafe.Pointer, n C.int) {
	// A panic on a framework thread would end the daemon.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screencap: audio packet callback: %v", r)
		}
	}()
	if s := lookup(uintptr(h)); s != nil && s.audio != nil {
		s.audio(AudioPacket{Data: C.GoBytes(data, n), Samples: OpusFrames})
	}
}

//export goScreencapAudioError
func goScreencapAudioError(h C.uintptr_t, code C.int, msg *C.char) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screencap: audio error callback: %v", r)
		}
	}()
	if s := lookup(uintptr(h)); s != nil && s.audioErr != nil {
		s.audioErr(fmt.Errorf("%w: %s (code %d)", ErrAudioUnavailable, C.GoString(msg), int(code)))
	}
}

// encodeOpusForTest encodes interleaved float32 stereo PCM (48 kHz) with a fresh
// AudioConverter in one buffer and returns the packets and the bit rate the
// converter reports.
func encodeOpusForTest(pcm []float32) ([][]byte, int, error) {
	return encodeOpusChunksForTest(pcm, len(pcm)/2)
}

// encodeOpusChunksForTest is encodeOpusForTest with the PCM delivered in buffers
// of chunk frames, the way ScreenCaptureKit delivers it.
func encodeOpusChunksForTest(pcm []float32, chunk int) ([][]byte, int, error) {
	if !testing.Testing() {
		return nil, 0, errors.New("screencap: encodeOpusChunksForTest outside a test binary")
	}
	if len(pcm) == 0 || chunk <= 0 {
		return nil, 0, errors.New("screencap: no PCM to encode")
	}
	var pkts [][]byte
	var errs []error
	h := cgo.NewHandle(&sink{
		audio:    func(p AudioPacket) { pkts = append(pkts, p.Data) },
		audioErr: func(err error) { errs = append(errs, err) },
	})
	defer h.Delete()
	var rate C.int
	rc := C.pc_opus_test(C.uintptr_t(h), (*C.float)(unsafe.Pointer(&pcm[0])), C.int(len(pcm)/2), C.int(chunk), C.int(OpusBitrate), &rate)
	if rc != 0 {
		return nil, 0, fmt.Errorf("%w: AudioConverter status %d", ErrAudioUnavailable, int(rc))
	}
	// pc_opus_test finishes before it returns, so every callback has run.
	return pkts, int(rate), errors.Join(errs...)
}
