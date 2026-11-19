package relay

// IN-08 (DEBT-03): charset and hyperlink state for rendered replays.
//
// A viewer joining through a rendered replay starts from a RIS. If the host is
// mid-way through SO/G1 line drawing, has a G0 line-drawing set designated or
// has an OSC 8 link open, the viewer would print the host's later bytes in
// ASCII or unlinked. vt exposes none of that state, so the hub tracks it from
// the bytes it feeds the live emulator and ends every replay body with it.
//
// Designations and the link come from handlers registered on the emulator
// (newLiveEmulator), like the DEC-mode tracking, so they run in byte order
// inside the write. GL comes from a byte scan: SO and SI are C0 controls, and
// vt's control handlers cannot be registered (registerCcHandler is unexported).
// The tracker mirrors vt, not xterm: vt has no DECSTR handler and keeps no
// charset in its saved cursor (cursor.go), so neither is tracked.
//
// shortcut: a link that DECRC or an alt-screen switch restores in vt (which
// keeps the link in each screen's cursor) is not seen until the next alt-exit
// rebuild resyncs the tracker from its probes; G2/G3, LS2/LS3, single shifts
// and GR are not tracked. Track them if a program needs them.

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// charState is the hub's view of the host terminal's charset and link state.
// Hub-owned (it survives rebuilds) and only touched under Hub.emuMu.
type charState struct {
	g    [2]byte   // G0 and G1 designation final byte ('0', 'A'); 0 is ASCII ('B')
	gl1  bool      // GL is G1 (SO)
	link [2]string // the open OSC 8 link: params, URI; an empty URI is none
	vt   vtScan    // vt's parser state across reads, for the GL scan
	esc  bool      // the last byte scanned was an ESC
}

// designate records ESC ( f (set 0) or ESC ) f (set 1).
func (c *charState) designate(set int, f byte) {
	if f == 'B' {
		f = 0
	}
	c.g[set] = f
}

// osc8 records an OSC 8 payload ("8;params;uri") the way vt accepts it: exactly
// three parts, otherwise ignored.
func (c *charState) osc8(data []byte) {
	parts := strings.Split(string(data), ";")
	if len(parts) != 3 {
		return
	}
	c.link = [2]string{parts[1], parts[2]}
	if parts[2] == "" {
		c.link = [2]string{}
	}
}

// reset is a RIS: designations and link to their defaults. GL is reset by the
// scan, in byte order.
func (c *charState) reset() {
	c.g, c.link = [2]byte{}, [2]string{}
}

// scan updates GL from b, the bytes about to be fed to the emulator: SO and SI
// where vt executes C0 controls (ground, ESC, CSI), never inside a string, and
// ESC c resets it.
func (c *charState) scan(b []byte) {
	for _, x := range b {
		switch {
		case (x == 0x0e || x == 0x0f) && c.vt.need == 0 &&
			(c.vt.state == scanGround || c.vt.state == scanEsc || c.vt.state == scanCsi):
			c.gl1 = x == 0x0e
		case x == 'c' && c.esc && c.vt.state == scanEsc:
			c.gl1 = false
		}
		c.esc = x == 0x1b
		c.vt.step(x)
	}
}

// suffix re-establishes the tracked state in a viewer that starts from a RIS.
func (c *charState) suffix() string {
	var b strings.Builder
	for i, intro := range []string{"\x1b(", "\x1b)"} {
		if c.g[i] != 0 {
			b.WriteString(intro + string(c.g[i]))
		}
	}
	if c.gl1 {
		b.WriteByte(0x0e)
	}
	if c.link[1] != "" {
		b.WriteString(ansi.SetHyperlink(c.link[1], c.link[0]))
	}
	return b.String()
}

// renderedLinkRE matches the OSC 8 that uv's renderLine writes: BEL-terminated,
// and neither field can hold a ';' (vt only stores a three-part payload).
var renderedLinkRE = regexp.MustCompile("\x1b\\]8;([^;\x07\x1b]*);([^;\x07\x1b]*)\x07")

// fixRenderedLinks swaps the two fields of every OSC 8 in a vt-rendered string.
// vt's handleHyperlink (osc.go) stores the payload's params in Link.URL and its
// URI in Link.Params, and renderLine emits SetHyperlink(Link.URL, Link.Params),
// which writes ESC]8;Params;URL. So ESC]8;id=7;https://example.invalid/x ST
// renders as ESC]8;https://example.invalid/x;id=7 BEL, and a link without
// params reaches xterm.js with an empty URI, which closes it.
//
// renderLine closes a link at the row end only when Link.URL (the params) is
// set, so a link without params that runs to the row end is closed here: a
// rendered row stays self-contained.
func fixRenderedLinks(s string) string {
	if !strings.Contains(s, "\x1b]8;") {
		return s
	}
	open := false
	s = renderedLinkRE.ReplaceAllStringFunc(s, func(m string) string {
		f := renderedLinkRE.FindStringSubmatch(m)
		open = f[1] != ""
		return "\x1b]8;" + f[2] + ";" + f[1] + "\x07"
	})
	if open {
		s += ansi.ResetHyperlink()
	}
	return s
}
