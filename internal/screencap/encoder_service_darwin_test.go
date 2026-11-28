package screencap

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// A start whose encoder VideoToolbox refused is ErrEncoderStart (the desktop
// recovery policy counts it), with the message the daemon has always logged;
// a capture failure never is.
func TestStartError_EncoderIsTyped(t *testing.T) {
	err := startError(-12902, true, "starting the encoder failed")
	if !errors.Is(err, ErrEncoderStart) {
		t.Fatalf("encoder failure = %v, want ErrEncoderStart", err)
	}
	if want := "screencap: starting the encoder failed (code -12902)"; err.Error() != want {
		t.Errorf("message %q, want %q (the U-load log line)", err.Error(), want)
	}
	for _, c := range []struct {
		code int
		msg  string
		want error
	}{
		{-3801, "denied", ErrPermissionDenied},
		{-3815, "no display", ErrNoDisplay},
		{-3805, "stream stopped", nil},
		{-90004, "starting capture timed out", nil},
	} {
		err := startError(c.code, false, c.msg)
		if err == nil || errors.Is(err, ErrEncoderStart) {
			t.Errorf("capture failure %d = %v, want a non-encoder error", c.code, err)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("capture failure %d = %v, want %v", c.code, err, c.want)
		}
	}
}

// Source check (no test may make VideoToolbox fail or signal a process): the
// encoder branch of pc_stream_start raises the flag the error is typed by, and
// the lookup refuses while this process has an encoder running and matches
// only a service whose responsible process is this one.
func TestEncoderService_SafetySource(t *testing.T) {
	start := objcFunc(t, "media_darwin.m", "pc_stream_start")
	enc := strings.Index(start, `@"starting the encoder failed"`)
	if enc < 0 || !strings.Contains(start[enc:min(enc+120, len(start))], "*encoderFailed = 1;") {
		t.Error("pc_stream_start's encoder branch must set *encoderFailed = 1")
	}
	look := objcFunc(t, "media_darwin.m", "pc_encoder_service_pids")
	for _, want := range []string{"if (gEnc != nil) return -2;", "pids[i] == self", "if (responsible(pids[i]) != self) continue;", "XPCServices/VTEncoderXPCService"} {
		if !strings.Contains(look, want) {
			t.Errorf("pc_encoder_service_pids lacks %q", want)
		}
	}
}

func TestResetEncoderService_RefusesUnderTest(t *testing.T) {
	if pids, err := ResetEncoderService(); !errors.Is(err, ErrRefusedInTest) || pids != nil {
		t.Fatalf("ResetEncoderService under test = %v, %v; want ErrRefusedInTest and no pid", pids, err)
	}
}

// The lookup behind ResetEncoderService only reads the process table (it
// signals nothing). It must resolve the responsibility call on this macOS (a
// missing one would leave the daemon without recovery) and never name this
// process.
func TestOwnEncoderServices_ReadOnly(t *testing.T) {
	pids, err := ownEncoderServices()
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	for _, p := range pids {
		if p == os.Getpid() {
			t.Fatalf("lookup named this process (%d)", p)
		}
	}
}
