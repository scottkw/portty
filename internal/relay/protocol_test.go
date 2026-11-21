package relay

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestMakeOutputFrame(t *testing.T) {
	data := []byte("hello")
	frame := MakeOutputFrame(data)
	if len(frame) != len(data)+1 {
		t.Fatalf("expected length %d, got %d", len(data)+1, len(frame))
	}
	if frame[0] != MsgOutput {
		t.Errorf("expected type byte 0x%02x, got 0x%02x", MsgOutput, frame[0])
	}
	if !bytes.Equal(frame[1:], data) {
		t.Errorf("payload mismatch: got %v, want %v", frame[1:], data)
	}
}

func TestMakeInputFrame(t *testing.T) {
	data := []byte("world")
	frame := MakeInputFrame(data)
	if frame[0] != MsgInput {
		t.Errorf("expected type byte 0x%02x, got 0x%02x", MsgInput, frame[0])
	}
	if !bytes.Equal(frame[1:], data) {
		t.Errorf("payload mismatch: got %v, want %v", frame[1:], data)
	}
}

func TestMakeResizeFrame(t *testing.T) {
	frame := MakeResizeFrame(80, 24)
	// Expected: [MsgResize, 0x00, 0x50, 0x00, 0x18]
	expected := []byte{MsgResize, 0x00, 0x50, 0x00, 0x18}
	if len(frame) != 5 {
		t.Fatalf("expected 5 bytes, got %d", len(frame))
	}
	if !bytes.Equal(frame, expected) {
		t.Errorf("resize frame mismatch: got %v, want %v", frame, expected)
	}
}

func TestParseFrameOutput(t *testing.T) {
	data := []byte("hello")
	frame := MakeOutputFrame(data)
	msgType, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != MsgOutput {
		t.Errorf("expected type 0x%02x, got 0x%02x", MsgOutput, msgType)
	}
	if !bytes.Equal(payload, data) {
		t.Errorf("payload mismatch: got %v, want %v", payload, data)
	}
}

func TestParseFrameInput(t *testing.T) {
	data := []byte("world")
	frame := MakeInputFrame(data)
	msgType, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != MsgInput {
		t.Errorf("expected type 0x%02x, got 0x%02x", MsgInput, msgType)
	}
	if !bytes.Equal(payload, data) {
		t.Errorf("payload mismatch: got %v, want %v", payload, data)
	}
}

func TestParseFrameResize(t *testing.T) {
	frame := MakeResizeFrame(80, 24)
	msgType, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != MsgResize {
		t.Errorf("expected type 0x%02x, got 0x%02x", MsgResize, msgType)
	}
	// payload should be the 4 bytes of cols+rows
	expected := []byte{0x00, 0x50, 0x00, 0x18}
	if !bytes.Equal(payload, expected) {
		t.Errorf("resize payload mismatch: got %v, want %v", payload, expected)
	}
}

func TestParseFrameEmpty(t *testing.T) {
	_, _, err := ParseFrame([]byte{})
	if err == nil {
		t.Error("expected error on empty frame, got nil")
	}
}

func TestParseFrameSingleByte(t *testing.T) {
	// Single byte: type only, empty payload — should succeed
	msgType, payload, err := ParseFrame([]byte{MsgPing})
	if err != nil {
		t.Fatalf("unexpected error on single-byte frame: %v", err)
	}
	if msgType != MsgPing {
		t.Errorf("expected type 0x%02x, got 0x%02x", MsgPing, msgType)
	}
	if len(payload) != 0 {
		t.Errorf("expected empty payload, got %v", payload)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	types := []struct {
		name  string
		frame []byte
		typ   byte
	}{
		{"output", MakeOutputFrame([]byte("data")), MsgOutput},
		{"input", MakeInputFrame([]byte("data")), MsgInput},
	}
	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			msgType, payload, err := ParseFrame(tc.frame)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msgType != tc.typ {
				t.Errorf("expected type 0x%02x, got 0x%02x", tc.typ, msgType)
			}
			if !bytes.Equal(payload, []byte("data")) {
				t.Errorf("payload mismatch")
			}
		})
	}
}

func TestMakeMeta_RoundTrip(t *testing.T) {
	count := 3
	p := MetaPayload{ViewerCount: &count}
	frame := MakeMeta(p)

	if frame[0] != MsgMeta {
		t.Errorf("expected type byte 0x%02x, got 0x%02x", MsgMeta, frame[0])
	}

	msgType, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != MsgMeta {
		t.Errorf("expected type 0x%02x, got 0x%02x", MsgMeta, msgType)
	}

	var decoded MetaPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to unmarshal MetaPayload: %v", err)
	}
	if decoded.ViewerCount == nil || *decoded.ViewerCount != 3 {
		t.Errorf("expected viewerCount=3, got %v", decoded.ViewerCount)
	}
}

func TestMakeMeta_OmitsNilFields(t *testing.T) {
	frame := MakeMeta(MetaPayload{})
	_, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty MetaPayload should produce "{}" — no viewerCount key
	if bytes.Contains(payload, []byte("viewerCount")) {
		t.Errorf("expected omitted viewerCount for nil pointer, got: %s", payload)
	}
}

// TestMsgPongWireValue pins the pong wire byte; the frontend pins the same literal.
func TestMsgPongWireValue(t *testing.T) {
	if MsgPong != 0x04 {
		t.Errorf("MsgPong = 0x%02x, want 0x04", MsgPong)
	}
	if !bytes.Equal(PongFrame, []byte{0x04}) {
		t.Errorf("PongFrame = %v, want [4]", PongFrame)
	}
}

// TestParseResize pins the INPT-06 claim bit: MsgResize2 payload[4] bit 0.
// 4 bytes is a report (every older viewer), other bits are ignored.
func TestParseResize(t *testing.T) {
	for _, c := range []struct {
		name       string
		payload    []byte
		cols, rows int
		claim, ok  bool
	}{
		{"4 bytes is a report", []byte{0, 80, 0, 24}, 80, 24, false, true},
		{"claim bit", []byte{0, 80, 0, 24, 0x01}, 80, 24, true, true},
		{"zero byte 5 is a report", []byte{0, 80, 0, 24, 0x00}, 80, 24, false, true},
		{"other bits ignored", []byte{0, 80, 0, 24, 0x02}, 80, 24, false, true},
		{"big-endian", []byte{0x12, 0x34, 0x00, 0x30, 0x01}, 0x1234, 0x30, true, true},
		{"3 bytes is not ok", []byte{0, 80, 0}, 0, 0, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			cols, rows, claim, ok := ParseResize(c.payload)
			if cols != c.cols || rows != c.rows || claim != c.claim || ok != c.ok {
				t.Fatalf("ParseResize(%v) = %d, %d, %t, %t; want %d, %d, %t, %t",
					c.payload, cols, rows, claim, ok, c.cols, c.rows, c.claim, c.ok)
			}
		})
	}
}
