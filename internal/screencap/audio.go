package screencap

import "errors"

// audio.go: the system-audio half of a capture source. Audio rides the same
// ScreenCaptureKit stream as the picture (one SCStream, one Screen Recording
// grant, never a microphone or any input device) and leaves the source already
// encoded, one 20 ms Opus packet at a time.

// The Opus constants every packet is made with.
const (
	// OpusFrames is the PCM frames in one packet: 20 ms at 48 kHz.
	OpusFrames = 960
	// OpusRate is the sample rate, and the RTP clock rate of the audio track.
	OpusRate = 48000
	// OpusBitrate is the encoder's target in bits per second.
	OpusBitrate = 64000
)

// ErrAudioUnavailable is what a source reports when it cannot capture or
// encode system audio. The picture is unaffected.
var ErrAudioUnavailable = errors.New("screencap: system audio unavailable")

// AudioPacket is one encoded Opus packet of Samples PCM frames per channel.
type AudioPacket struct {
	Data    []byte
	Samples int
}

// AudioSource is the optional audio half of a Source, so every Source that has
// no audio keeps compiling.
//
//   - AudioPackets and AudioErrors are non-nil after a successful Start and
//     closed by Stop.
//   - A packet is dropped when nobody reads (audio is real time); a failure is
//     an error on AudioErrors and never stops Frames.
type AudioSource interface {
	AudioPackets() <-chan AudioPacket
	AudioErrors() <-chan error
}

// AudioRestarter is the optional audio restart of a running AudioSource: it
// re-creates the audio capture alone and never touches Frames.
type AudioRestarter interface {
	RestartAudio() error
}
