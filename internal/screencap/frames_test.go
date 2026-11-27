package screencap

import (
	"bytes"
	"testing"
)

func TestCaptureSize(t *testing.T) {
	cases := []struct {
		name         string
		pw, ph       int
		scale        float64
		maxLong      int
		wantW, wantH int
	}{
		{"retina laptop display fits the cap", 1470, 956, 2, 2560, 2560, 1664},
		{"1x display below the cap stays", 1280, 800, 1, 2560, 1280, 800},
		{"large retina display is capped with even sides", 3024, 1964, 2, 2560, 2560, 1662},
		{"never upscales", 800, 600, 1, 2560, 800, 600},
		{"odd result rounds down to even", 1001, 601, 1, 2560, 1000, 600},
		{"no cap when maxLong is zero", 3000, 2000, 2, 0, 6000, 4000},
	}
	for _, c := range cases {
		w, h := captureSize(c.pw, c.ph, c.scale, c.maxLong)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: captureSize(%d,%d,%v,%d) = %dx%d, want %dx%d",
				c.name, c.pw, c.ph, c.scale, c.maxLong, w, h, c.wantW, c.wantH)
		}
		if w%2 != 0 || h%2 != 0 {
			t.Errorf("%s: odd size %dx%d", c.name, w, h)
		}
	}
}

func TestAvccToAnnexB(t *testing.T) {
	sps, pps := []byte{0x67, 1, 2}, []byte{0x68, 3}
	idr, sei := []byte{0x65, 9, 9, 9}, []byte{0x06, 7}
	avcc := []byte{0, 0, 0, 4}
	avcc = append(avcc, idr...)
	avcc = append(avcc, 0, 0, 0, 2)
	avcc = append(avcc, sei...)
	sc := []byte{0, 0, 0, 1}
	join := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

	got, err := avccToAnnexB(avcc, [][]byte{sps, pps}, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := join(sc, sps, sc, pps, sc, idr, sc, sei); !bytes.Equal(got, want) {
		t.Errorf("keyframe: got % x, want % x", got, want)
	}

	got, err = avccToAnnexB(avcc, [][]byte{sps, pps}, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := join(sc, idr, sc, sei); !bytes.Equal(got, want) {
		t.Errorf("delta must not carry parameter sets: got % x, want % x", got, want)
	}

	for name, bad := range map[string][]byte{
		"length past the end":   {0, 0, 0, 9, 1, 2},
		"truncated length":      {0, 0, 0, 1, 7, 0, 0},
		"zero length":           {0, 0, 0, 0, 1},
		"length prefix too big": {0xff, 0xff, 0xff, 0xff, 1},
	} {
		if _, err := avccToAnnexB(bad, nil, false); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
