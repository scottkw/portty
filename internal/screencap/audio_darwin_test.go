package screencap

import (
	"math"
	"slices"
	"testing"
)

// sinePCM is seconds of a 440 Hz sine, interleaved float32 stereo at 48 kHz.
func sinePCM(seconds float64) []float32 {
	n := int(seconds * OpusRate)
	pcm := make([]float32, 2*n)
	for i := 0; i < n; i++ {
		v := float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/OpusRate))
		pcm[2*i], pcm[2*i+1] = v, v
	}
	return pcm
}

var _ AudioSource = (*DisplaySource)(nil)

// TestOpus_SineMakesFiftyPackets: one second of stereo sine becomes 50 non-empty
// packets, none over the 4096-byte buffer, at the 64000 bps target. A converter
// that refused 64000 would read back 96000 (RESEARCH A1); the SUMMARY records
// which one this Mac gave.
func TestOpus_SineMakesFiftyPackets(t *testing.T) {
	pkts, rate, err := encodeOpusForTest(sinePCM(1))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("AudioConverter Opus bit rate read back: %d", rate)
	if rate != OpusBitrate && rate != 96000 {
		t.Errorf("bit rate read back %d, want %d (or the 96000 fallback)", rate, OpusBitrate)
	}
	if len(pkts) != 50 {
		t.Fatalf("%d packets for 1 s of audio, want 50", len(pkts))
	}
	for i, p := range pkts {
		if len(p) == 0 || len(p) > 4096 {
			t.Errorf("packet %d is %d bytes, want 1..4096", i, len(p))
		}
	}
}

// TestOpus_SilenceIsSmall: silence costs a few bytes a packet (R21 control run:
// 3-byte packets), so an idle Mac sends almost nothing.
func TestOpus_SilenceIsSmall(t *testing.T) {
	pkts, _, err := encodeOpusForTest(make([]float32, 2*OpusRate))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkts) != 50 {
		t.Fatalf("%d packets for 1 s of silence, want 50", len(pkts))
	}
	sizes := make([]int, len(pkts))
	for i, p := range pkts {
		sizes[i] = len(p)
	}
	slices.Sort(sizes)
	if med := sizes[len(sizes)/2]; med >= 16 {
		t.Errorf("median silence packet is %d bytes, want under 16", med)
	}
}

// TestOpus_PartialBlockIsHeld: fewer than 960 frames are never encoded short;
// they wait for the next buffer, whatever size the buffers arrive in.
func TestOpus_PartialBlockIsHeld(t *testing.T) {
	pcm := sinePCM(1)
	if pkts, _, err := encodeOpusChunksForTest(pcm[:2*(OpusFrames-1)], OpusFrames-1); err != nil || len(pkts) != 0 {
		t.Fatalf("959 frames gave %d packets, err %v, want none", len(pkts), err)
	}
	if pkts, _, err := encodeOpusChunksForTest(pcm[:2*(2*OpusFrames-1)], 2*OpusFrames-1); err != nil || len(pkts) != 1 {
		t.Fatalf("1919 frames gave %d packets, err %v, want 1", len(pkts), err)
	}
	// Buffers of 1000 frames (not a multiple of 960) still make 50 packets a second.
	pkts, _, err := encodeOpusChunksForTest(pcm, 1000)
	if err != nil || len(pkts) != 50 {
		t.Fatalf("1 s in 1000-frame buffers gave %d packets, err %v, want 50", len(pkts), err)
	}
}
