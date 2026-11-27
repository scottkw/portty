package screencap

import (
	"errors"
	"math"
)

// captureSize is the encoded size of a display: points times the display's
// point-to-pixel scale, both sides scaled down together when the long edge
// exceeds maxLong (never up), each side rounded down to an even number as
// H.264 4:2:0 needs. maxLong <= 0 means no cap.
func captureSize(pointsW, pointsH int, scale float64, maxLong int) (w, h int) {
	pw, ph := float64(pointsW)*scale, float64(pointsH)*scale
	f := 1.0
	if long := math.Max(pw, ph); maxLong > 0 && long > float64(maxLong) {
		f = float64(maxLong) / long
	}
	return int(math.Round(pw*f)) &^ 1, int(math.Round(ph*f)) &^ 1
}

// errBadAVCC is returned for an AVCC buffer whose length prefixes do not tile it.
var errBadAVCC = errors.New("screencap: malformed AVCC access unit")

// avccToAnnexB converts one VideoToolbox sample (4-byte big-endian length
// prefixes) to Annex-B (0x00000001 start codes). On a keyframe the parameter
// sets (SPS then PPS, as the caller orders them) are emitted first.
func avccToAnnexB(avcc []byte, paramSets [][]byte, keyframe bool) ([]byte, error) {
	start := []byte{0, 0, 0, 1}
	out := make([]byte, 0, len(avcc)+64)
	if keyframe {
		for _, ps := range paramSets {
			out = append(append(out, start...), ps...)
		}
	}
	for off := 0; off < len(avcc); {
		if off+4 > len(avcc) {
			return nil, errBadAVCC
		}
		n := int(avcc[off])<<24 | int(avcc[off+1])<<16 | int(avcc[off+2])<<8 | int(avcc[off+3])
		off += 4
		if n == 0 || n > len(avcc)-off {
			return nil, errBadAVCC
		}
		out = append(append(out, start...), avcc[off:off+n]...)
		off += n
	}
	return out, nil
}
