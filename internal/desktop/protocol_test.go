package desktop

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"slices"
	"testing"
)

// goldenLine is one line of a testdata/*-v1.jsonl file: the frozen v1 wire
// contract. The frontend (and later phases) read the same files, keyed by name.
type goldenLine struct {
	Name  string          `json:"name"`
	Dir   string          `json:"dir"`
	Valid bool            `json:"valid"`
	Wire  json.RawMessage `json:"wire"`
}

func readGolden(t *testing.T, path string) []goldenLine {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []goldenLine
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var l goldenLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil || l.Name == "" || len(l.Wire) == 0 {
			t.Fatalf("%s: bad line %q: %v", path, sc.Text(), err)
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		t.Fatalf("%s is empty", path)
	}
	return out
}

// TestProtocolGolden_InputDecodes: every valid viewer line decodes with its
// fields intact (x and y after clamping) and every invalid one is rejected.
func TestProtocolGolden_InputDecodes(t *testing.T) {
	var valid, invalid int
	for _, l := range readGolden(t, "testdata/input-v1.jsonl") {
		if l.Dir != "viewer" {
			continue // 20-02 tests the daemon's own messages
		}
		msg, err := parseInput(l.Wire)
		if !l.Valid {
			invalid++
			if err == nil {
				t.Errorf("%s: an invalid line was accepted as %+v", l.Name, msg)
			} else if !errors.Is(err, errBadInput) && !errors.Is(err, errUnknownKind) {
				t.Errorf("%s: rejected with %v, want a validation or unknown-kind error", l.Name, err)
			}
			continue
		}
		valid++
		if err != nil {
			t.Errorf("%s: %v", l.Name, err)
			continue
		}
		var wire map[string]any
		if err := json.Unmarshal(l.Wire, &wire); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"t": msg.T}
		set := func(k string, v any) {
			if _, ok := wire[k]; ok {
				want[k] = v
			}
		}
		set("x", msg.X)
		set("y", msg.Y)
		set("dx", msg.DX)
		set("dy", msg.DY)
		set("e", float64(msg.E))
		set("b", float64(msg.B))
		set("d", msg.D)
		set("ts", msg.TS)
		set("code", msg.Code)
		set("r", msg.R)
		set("m", float64(msg.M))
		set("caps", msg.Caps)
		set("k", msg.K)
		set("s", msg.S)
		for _, k := range []string{"x", "y"} {
			if v, ok := wire[k].(float64); ok {
				wire[k] = math.Max(0, math.Min(1, v)) // the wire value after clamping
			}
		}
		for k, v := range wire {
			if k == "future" {
				continue // unknown fields are ignored, not decoded
			}
			if want[k] != v {
				t.Errorf("%s: field %s decoded as %v, wire says %v", l.Name, k, want[k], v)
			}
		}
	}
	if valid < 16 || invalid < 8 {
		t.Errorf("golden file has %d valid and %d invalid viewer lines, want at least 16 and 8", valid, invalid)
	}
}

// signalShape is the required fields of a signalling message of one type and
// direction; anyOf lists alternatives (a reply with an error or with a list).
type signalShape struct {
	dir, typ string
	anyOf    [][]string
}

var signalShapes = []signalShape{
	{"viewer", "hello", [][]string{{"v", "caps"}}},
	{"viewer", "surfaces", [][]string{{}}},
	{"viewer", "select", [][]string{{"kind", "id"}}},
	{"viewer", "clip_get", [][]string{{"xid"}}},
	{"viewer", "clip_put", [][]string{{"xid", "seq", "total", "len", "text"}}},
	{"server", "hello", [][]string{{"v", "viewer", "caps", "rights"}}},
	{"server", "surfaces", [][]string{{"displays", "windows"}, {"error"}}},
	{"server", "surface", [][]string{{"e", "kind", "id", "name", "w", "h", "reason"}}},
	{"server", "select_failed", [][]string{{"code"}}},
	{"server", "clip_data", [][]string{{"xid", "seq", "total", "len", "text"}}},
	{"server", "clip_result", [][]string{{"xid", "outcome"}}},
}

// TestProtocolGolden_SignalShapes: every signalling line is a JSON object of a
// v1 type with that type's required fields, so a later edit that drops a field
// fails here before the handlers decode it into real types.
func TestProtocolGolden_SignalShapes(t *testing.T) {
	for _, l := range readGolden(t, "testdata/signal-v1.jsonl") {
		var wire map[string]json.RawMessage
		if err := json.Unmarshal(l.Wire, &wire); err != nil {
			t.Errorf("%s: not a JSON object: %v", l.Name, err)
			continue
		}
		var typ string
		if err := json.Unmarshal(wire["type"], &typ); err != nil {
			t.Errorf("%s: no string type", l.Name)
			continue
		}
		i := slices.IndexFunc(signalShapes, func(s signalShape) bool { return s.dir == l.Dir && s.typ == typ })
		if i < 0 {
			t.Errorf("%s: %s message of type %q is not in the v1 set", l.Name, l.Dir, typ)
			continue
		}
		ok := slices.ContainsFunc(signalShapes[i].anyOf, func(fields []string) bool {
			return !slices.ContainsFunc(fields, func(f string) bool { _, has := wire[f]; return !has })
		})
		if !ok {
			t.Errorf("%s: %s %s lacks the required fields %v", l.Name, l.Dir, typ, signalShapes[i].anyOf)
		}
	}
}

// TestProtocolGolden_SurfaceDecodes: the surfaces, select, surface and
// select_failed lines of the golden file unmarshal into the Go message types
// and marshal back to the same JSON, so the frozen wire shape cannot drift from
// the handlers' types.
func TestProtocolGolden_SurfaceDecodes(t *testing.T) {
	semantically := func(b []byte) any {
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	seen := map[string]int{}
	for _, l := range readGolden(t, "testdata/signal-v1.jsonl") {
		var msg any
		switch {
		case l.Dir == "viewer" && (l.Name == "surfaces-request" || l.Name == "select-main" || l.Name == "select-window"):
			var in inbound
			if err := json.Unmarshal(l.Wire, &in); err != nil {
				t.Errorf("%s: %v", l.Name, err)
			}
			if l.Name == "select-window" && (in.Type != typeSelect || in.Kind != "window" || in.ID != 4242) {
				t.Errorf("%s decoded as %+v", l.Name, in)
			}
			if l.Name == "select-main" && (in.Type != typeSelect || in.Kind != "display" || in.ID != 0) {
				t.Errorf("%s decoded as %+v", l.Name, in)
			}
			seen["viewer"]++
			continue
		case l.Dir == "server" && (l.Name == "surfaces" || l.Name == "surfaces-error"):
			msg = &msgSurfaces{}
		case l.Dir == "server" && (l.Name == "surface-window" || l.Name == "surface-fallback"):
			msg = &msgSurface{}
		case l.Dir == "server" && l.Name == "select-failed":
			msg = &msgSelectFailed{}
		default:
			continue
		}
		if err := json.Unmarshal(l.Wire, msg); err != nil {
			t.Errorf("%s: %v", l.Name, err)
			continue
		}
		back, err := json.Marshal(msg)
		if err != nil {
			t.Errorf("%s: %v", l.Name, err)
			continue
		}
		if !reflect.DeepEqual(semantically(back), semantically(l.Wire)) {
			t.Errorf("%s: marshals back as %s, want %s", l.Name, back, l.Wire)
		}
		seen["server"]++
	}
	if seen["viewer"] != 3 || seen["server"] != 5 {
		t.Errorf("checked %d viewer and %d server lines, want 3 and 5", seen["viewer"], seen["server"])
	}
}

// TestProtocolGolden_ClipDecodes: the viewer's clip_get and clip_put lines
// decode into inbound with their fields, and the daemon's clip_data and
// clip_result lines unmarshal into the Go message types and marshal back equal.
func TestProtocolGolden_ClipDecodes(t *testing.T) {
	semantically := func(b []byte) any {
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	var viewer, server int
	for _, l := range readGolden(t, "testdata/signal-v1.jsonl") {
		switch {
		case l.Dir == "viewer" && (l.Name == "clip-get" || l.Name == "clip-put-0" || l.Name == "clip-put-1"):
			var in inbound
			if err := json.Unmarshal(l.Wire, &in); err != nil {
				t.Errorf("%s: %v", l.Name, err)
			}
			want := map[string]inbound{
				"clip-get":   {Type: typeClipGet, Xid: "c1"},
				"clip-put-0": {Type: typeClipPut, Xid: "p1", Seq: 0, Total: 2, Len: 7, Text: "hel"},
				"clip-put-1": {Type: typeClipPut, Xid: "p1", Seq: 1, Total: 2, Len: 7, Text: "lo, "},
			}[l.Name]
			if !reflect.DeepEqual(in, want) {
				t.Errorf("%s decoded as %+v, want %+v", l.Name, in, want)
			}
			viewer++
		case l.Dir == "server" && (l.Name == "clip-data" || l.Name == "clip-result-ok" || l.Name == "clip-result-concealed" || l.Name == "clip-result-waiting"):
			var msg any = &msgClipResult{}
			if l.Name == "clip-data" {
				msg = &msgClipData{}
			}
			if err := json.Unmarshal(l.Wire, msg); err != nil {
				t.Errorf("%s: %v", l.Name, err)
				continue
			}
			back, err := json.Marshal(msg)
			if err != nil {
				t.Errorf("%s: %v", l.Name, err)
				continue
			}
			if !reflect.DeepEqual(semantically(back), semantically(l.Wire)) {
				t.Errorf("%s: marshals back as %s, want %s", l.Name, back, l.Wire)
			}
			server++
		}
	}
	if viewer != 3 || server != 4 {
		t.Errorf("checked %d viewer and %d server lines, want 3 and 4", viewer, server)
	}
}
