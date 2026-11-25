package main

// server.go: the spike's pion server (serve) and its Go receiver (selftest).
// The bind guard, token gate, single UDP mux, non-trickle /offer, /report
// writer and the "control" data channel all live here.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// h264Fmtp advertises 42e01f (what browsers offer) although the VideoToolbox
// bitstream says 42c0xx: pion matches only the first two profile bytes.
const h264Fmtp = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"

var (
	cgnat = netip.MustParsePrefix("100.64.0.0/10")

	errBindRefused = errors.New("bind address refused")
	errUsage       = errors.New("usage")

	safeName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)
	ipv4Text = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// checkBind refuses wildcard addresses always, accepts loopback (test runs) and
// the tailnet range, and refuses everything else unless allowLAN. It runs
// before any socket opens.
func checkBind(bind string, allowLAN bool) (netip.Addr, error) {
	addr, err := netip.ParseAddr(bind)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %q is not an IP address", errBindRefused, bind)
	}
	addr = addr.Unmap()
	switch {
	case addr.IsUnspecified():
		return netip.Addr{}, fmt.Errorf("%w: %s is a wildcard address", errBindRefused, addr)
	case !addr.Is4():
		return netip.Addr{}, fmt.Errorf("%w: %s is not IPv4 (the spike binds IPv4 only)", errBindRefused, addr)
	case addr.IsLoopback(), cgnat.Contains(addr):
		return addr, nil
	case allowLAN:
		return addr, nil
	}
	return netip.Addr{}, fmt.Errorf("%w: %s is outside 127.0.0.0/8 and 100.64.0.0/10 (--allow-lan overrides)", errBindRefused, addr)
}

type serveOpts struct {
	bind        string
	httpPort    int
	udpPort     int
	token       string
	allowLAN    bool
	source      string
	widthCap    int
	fps         int
	bitrate     int
	evidenceDir string
	duration    time.Duration
	tlsCert     string
	tlsKey      string
	spsPatch    bool
	quietToken  bool // selftest keeps its throwaway token off stdout
}

// listenInfo tells an in-process caller where the server ended up (ports may be 0 in opts).
type listenInfo struct {
	httpAddr string
	token    string
	tls      bool
}

type server struct {
	opts  serveOpts
	bind  netip.Addr
	mux   *ice.UDPMuxDefault
	track *webrtc.TrackLocalStaticSample

	mu  sync.Mutex
	pcs map[*webrtc.PeerConnection]struct{}
}

func validToken(t string) bool {
	if len(t) != 32 {
		return false
	}
	_, err := hex.DecodeString(t)
	return err == nil
}

func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// sourceSize gives a 16:10 frame at the width cap, both even.
func sourceSize(widthCap int) (int, int) {
	w := widthCap &^ 1
	h := (w * 10 / 16) &^ 1
	return w, h
}

func newMediaEngine() (*webrtc.MediaEngine, error) {
	me := &webrtc.MediaEngine{}
	err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: h264Fmtp},
		PayloadType:        102,
	}, webrtc.RTPCodecTypeVideo)
	return me, err
}

func runServe(ctx context.Context, o serveOpts, ready chan<- listenInfo) error {
	addr, err := checkBind(o.bind, o.allowLAN)
	if err != nil {
		return err
	}
	if (o.tlsCert == "") != (o.tlsKey == "") {
		return fmt.Errorf("%w: --tls-cert and --tls-key go together", errUsage)
	}
	if o.token != "" && !validToken(o.token) {
		return fmt.Errorf("%w: --token must be 32 hex characters", errUsage)
	}
	if o.source != "synthetic" && o.source != "sck" {
		return fmt.Errorf("%w: --source must be synthetic or sck", errUsage)
	}
	if o.token == "" {
		if o.token, err = newToken(); err != nil {
			return fmt.Errorf("token: %w", err)
		}
		if !o.quietToken {
			fmt.Printf("token: %s\n", o.token) // printed once, never logged
		}
	}
	if o.duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.duration)
		defer cancel()
	}

	width, height := sourceSize(o.widthCap)
	if err := startEncoder(width, height, o.fps, o.bitrate); err != nil {
		return err
	}
	defer stopSource()

	udpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: addr.AsSlice(), Port: o.udpPort})
	if err != nil {
		return fmt.Errorf("udp listen: %w", err)
	}
	s := &server{opts: o, bind: addr, mux: ice.NewUDPMuxDefault(ice.UDPMuxParams{UDPConn: udpConn}), pcs: map[*webrtc.PeerConnection]struct{}{}}
	defer func() { _ = s.mux.Close() }()

	s.track, err = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: h264Fmtp}, "video", "spike")
	if err != nil {
		return fmt.Errorf("track: %w", err)
	}
	if o.evidenceDir != "" {
		if err := os.MkdirAll(o.evidenceDir, 0o755); err != nil {
			return fmt.Errorf("evidence dir: %w", err)
		}
	}

	ln, err := net.Listen("tcp4", net.JoinHostPort(addr.String(), strconv.Itoa(o.httpPort)))
	if err != nil {
		return fmt.Errorf("http listen: %w", err)
	}
	httpSrv := &http.Server{Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() {
		if o.tlsCert != "" {
			serveErr <- httpSrv.ServeTLS(ln, o.tlsCert, o.tlsKey)
		} else {
			serveErr <- httpSrv.Serve(ln)
		}
	}()

	if err := startSource(o.source, width, height, o.fps); err != nil {
		_ = httpSrv.Close()
		return err
	}
	go s.pump(ctx)

	slog.Info("serve listening", "http", ln.Addr().String(), "udp", udpConn.LocalAddr().String(),
		"source", o.source, "width", width, "height", height, "fps", o.fps, "tls", o.tlsCert != "")
	if ready != nil {
		ready <- listenInfo{httpAddr: ln.Addr().String(), token: o.token, tls: o.tlsCert != ""}
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serveErr:
	}
	shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
	s.closePeers()
	if n := auDropped.Load(); n > 0 {
		slog.Warn("access units dropped (pump behind)", "count", n)
	}
	if runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
		return runErr
	}
	return nil
}

func (s *server) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /offer", s.gate(s.offer))
	m.HandleFunc("POST /report", s.gate(s.report))
	return m
}

// gate answers 403 unless query t equals the token. The token is never logged.
func (s *server) gate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.opts.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// pump feeds access units to the shared track with wall-clock durations.
func (s *server) pump(ctx context.Context) {
	last := time.Now().Add(-time.Second / time.Duration(s.opts.fps))
	for {
		select {
		case <-ctx.Done():
			return
		case au := <-auCh:
			now := time.Now()
			if s.opts.spsPatch {
				patchSPS(au.data)
			}
			if err := s.track.WriteSample(media.Sample{Data: au.data, Duration: now.Sub(last)}); err != nil {
				slog.Error("write sample", "err", err)
			}
			last = now
		}
	}
}

// patchSPS rewrites the SPS constraint byte 0xc0 to 0xe0 in place (RESEARCH
// Pitfall 1 fallback): start code, NAL type 7, profile 0x42, constraint byte.
func patchSPS(b []byte) {
	for i := 0; i+6 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 && b[i+3]&0x1f == 7 && b[i+4] == 0x42 && b[i+5] == 0xc0 {
			b[i+5] = 0xe0
		}
	}
}

func (s *server) newAPI(lite bool) (*webrtc.API, error) {
	se := webrtc.SettingEngine{}
	se.SetICEUDPMux(s.mux)
	se.SetLite(lite)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetIPFilter(func(ip net.IP) bool { return ip.Equal(s.bind.AsSlice()) })
	if s.bind.IsLoopback() {
		se.SetIncludeLoopbackCandidate(true)
	}
	me, err := newMediaEngine()
	if err != nil {
		return nil, err
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return nil, err
	}
	return webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir)), nil
}

func (s *server) addPeer(pc *webrtc.PeerConnection) {
	s.mu.Lock()
	s.pcs[pc] = struct{}{}
	s.mu.Unlock()
}

func (s *server) dropPeer(pc *webrtc.PeerConnection) {
	s.mu.Lock()
	delete(s.pcs, pc)
	s.mu.Unlock()
}

func (s *server) closePeers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pc := range s.pcs {
		_ = pc.Close()
	}
}

func (s *server) offer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil || offer.Type != webrtc.SDPTypeOffer {
		http.Error(w, "bad offer", http.StatusBadRequest)
		return
	}
	iceMode := r.URL.Query().Get("ice")
	if iceMode == "" {
		iceMode = "lite"
	}
	if iceMode != "lite" && iceMode != "full" {
		http.Error(w, "ice must be lite or full", http.StatusBadRequest)
		return
	}
	api, err := s.newAPI(iceMode == "lite")
	if err != nil {
		slog.Error("api", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	pc, err := api.NewPeerConnection(webrtc.Configuration{}) // ICEServers empty: no STUN, no TURN
	if err != nil {
		slog.Error("peer connection", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	fail := func(code int, msg string, err error) {
		slog.Warn("offer failed", "msg", msg, "err", err)
		_ = pc.Close()
		http.Error(w, msg, code)
	}

	sender, err := pc.AddTrack(s.track)
	if err != nil {
		fail(http.StatusInternalServerError, "internal error", err)
		return
	}
	go rtcpLoop(sender)
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() != "control" {
			return
		}
		dc.OnMessage(func(m webrtc.DataChannelMessage) { handleControl(dc, m) })
	})
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		slog.Info("peer state", "state", st.String(), "ice", iceMode)
		switch st {
		case webrtc.PeerConnectionStateConnected:
			forceKeyframe() // a joining viewer needs an IDR now
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			s.dropPeer(pc)
			_ = pc.Close()
		}
	})
	s.addPeer(pc)

	if err := pc.SetRemoteDescription(offer); err != nil {
		s.dropPeer(pc)
		fail(http.StatusBadRequest, "bad offer", err)
		return
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		s.dropPeer(pc)
		fail(http.StatusInternalServerError, "internal error", err)
		return
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		s.dropPeer(pc)
		fail(http.StatusInternalServerError, "internal error", err)
		return
	}
	select {
	case <-gathered:
	case <-time.After(5 * time.Second):
		s.dropPeer(pc)
		fail(http.StatusGatewayTimeout, "ice gathering timed out", errors.New("gather timeout"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(pc.LocalDescription()); err != nil {
		slog.Warn("write answer", "err", err)
	}
}

// rtcpLoop turns a viewer's PLI or FIR into a keyframe request.
func rtcpLoop(sender *webrtc.RTPSender) {
	for {
		pkts, _, err := sender.ReadRTCP()
		if err != nil {
			return
		}
		for _, p := range pkts {
			switch p.(type) {
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				forceKeyframe()
			}
		}
	}
}

// handleControl answers ping:<n> with pong:<n> and ignores everything else.
// Input (key, wake) arrives behind --allow-input in a later plan.
func handleControl(dc *webrtc.DataChannel, m webrtc.DataChannelMessage) {
	if !m.IsString {
		return
	}
	n, ok := strings.CutPrefix(string(m.Data), "ping:")
	if !ok {
		return
	}
	if _, err := strconv.ParseUint(n, 10, 63); err != nil {
		return
	}
	if err := dc.SendText("pong:" + n); err != nil {
		slog.Warn("pong", "err", err)
	}
}

// report stores a viewer report as <row>-report-<tag>-<ice>-<unix>.json after
// redaction; a frameJpeg data URL becomes a sibling .jpg.
func (s *server) report(w http.ResponseWriter, r *http.Request) {
	if s.opts.evidenceDir == "" {
		http.Error(w, "no --evidence-dir", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	row, tag, iceMode := q.Get("row"), q.Get("tag"), q.Get("ice")
	if !safeName.MatchString(row) || !safeName.MatchString(tag) || (iceMode != "lite" && iceMode != "full") {
		http.Error(w, "row, tag (A-Za-z0-9._-) and ice (lite|full) are required", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "body too large or unreadable", http.StatusRequestEntityTooLarge)
		return
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		http.Error(w, "body is not a JSON object", http.StatusBadRequest)
		return
	}
	base := fmt.Sprintf("%s-report-%s-%s-%d", row, tag, iceMode, time.Now().Unix())
	if raw, ok := doc["frameJpeg"]; ok {
		delete(doc, "frameJpeg")
		if err := s.writeFrame(filepath.Join(s.opts.evidenceDir, base+".jpg"), raw); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	clean, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	clean = []byte(strings.ReplaceAll(string(redact(clean)), s.opts.token, "REDACTED"))
	if err := writeNew(filepath.Join(s.opts.evidenceDir, base+".json"), append(clean, '\n')); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, os.ErrExist) {
			code = http.StatusConflict
		}
		http.Error(w, err.Error(), code)
		return
	}
	slog.Info("report stored", "file", base+".json")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) writeFrame(path string, raw json.RawMessage) error {
	var dataURL string
	if err := json.Unmarshal(raw, &dataURL); err != nil {
		return errors.New("frameJpeg is not a string")
	}
	b64, ok := strings.CutPrefix(dataURL, "data:image/jpeg;base64,")
	if !ok {
		return errors.New("frameJpeg is not a JPEG data URL")
	}
	img, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(img) < 3 || img[0] != 0xff || img[1] != 0xd8 {
		return errors.New("frameJpeg does not decode to a JPEG")
	}
	return writeNew(path, img)
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// redact turns each distinct 100.64/10 address into 100.x.x.<n> and each
// RFC1918 address into lan.<n>, numbered in order of first appearance.
func redact(b []byte) []byte {
	seen := map[string]string{}
	counts := map[string]int{}
	return ipv4Text.ReplaceAllFunc(b, func(m []byte) []byte {
		addr, err := netip.ParseAddr(string(m))
		if err != nil {
			return m
		}
		var kind string
		switch {
		case cgnat.Contains(addr):
			kind = "100.x.x."
		case addr.IsPrivate():
			kind = "lan."
		default:
			return m
		}
		key := string(m)
		if _, ok := seen[key]; !ok {
			counts[kind]++
			seen[key] = kind + strconv.Itoa(counts[kind])
		}
		return []byte(seen[key])
	})
}

// selftest runs a Go pion receiver against a spike server and writes the
// tracer evidence. With no --server it starts an in-process serve on loopback.
func selftest(ctx context.Context, args []string) error {
	fs := newFlagSet("selftest")
	out := fs.String("out", "build/t0", "output directory")
	seconds := fs.Int("seconds", 5, "seconds to receive after ICE connects")
	serverURL := fs.String("server", "", "target a remote serve instead of an in-process one")
	token := fs.String("token", "", "token for --server")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}

	base, tok, encoder := *serverURL, *token, json.RawMessage(nil)
	if base == "" {
		ready := make(chan listenInfo, 1)
		srvCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- runServe(srvCtx, serveOpts{bind: "127.0.0.1", source: "synthetic", widthCap: 1280, fps: 30, bitrate: 4_000_000,
				evidenceDir: *out, quietToken: true}, ready)
		}()
		defer func() { stop(); <-done }()
		select {
		case info := <-ready:
			base, tok = "http://"+info.httpAddr, info.token
		case err := <-done:
			return fmt.Errorf("in-process serve: %w", err)
		case <-time.After(15 * time.Second):
			return errors.New("in-process serve did not start")
		}
	} else if tok == "" {
		return fmt.Errorf("%w: --server needs --token", errUsage)
	}

	rep, err := receive(ctx, base, tok, *out, time.Duration(*seconds)*time.Second)
	if *serverURL == "" {
		encoder = json.RawMessage(encoderInfo())
	}
	rep.Encoder = encoder
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "t0-loopback-report.json"), append(body, '\n'), 0o644); err != nil {
		return err
	}
	slog.Info("selftest result", "connectedMs", rep.ConnectedMs, "packets", rep.Packets, "bytes", rep.Bytes,
		"accessUnits", rep.AccessUnits, "rttMedianMs", rep.RTTMedianMs)
	if err := postReport(base, tok, body); err != nil {
		return err
	}
	if rep.AccessUnits < 30 || rep.pongs < 1 {
		return fmt.Errorf("selftest failed: %d access units (need 30), %d pongs (need 1)", rep.AccessUnits, rep.pongs)
	}
	return nil
}

type selftestReport struct {
	ConnectedMs float64         `json:"connectedMs"`
	Packets     int64           `json:"packets"`
	Bytes       int64           `json:"bytes"`
	AccessUnits int64           `json:"accessUnits"`
	RTTMedianMs float64         `json:"rttMedianMs"`
	Encoder     json.RawMessage `json:"encoder"`
	pongs       int
}

func postReport(base, tok string, body []byte) error {
	u := fmt.Sprintf("%s/report?t=%s&row=t0&tag=selftest&ice=lite", base, url.QueryEscape(tok))
	resp, err := http.Post(u, "application/json", bytes.NewReader(body)) //nolint:gosec // spike: URL is the operator's own --server
	if err != nil {
		return fmt.Errorf("post report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("post report: status %d", resp.StatusCode)
	}
	return nil
}

// receive plays the browser: offer recvonly video plus the control channel,
// write the depacketized stream to <out>/t0-loopback.h264 and time the pongs.
func receive(ctx context.Context, base, tok, out string, dur time.Duration) (*selftestReport, error) {
	rep := &selftestReport{}
	u, err := url.Parse(base)
	if err != nil {
		return rep, err
	}
	se := webrtc.SettingEngine{}
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	if host, err := netip.ParseAddr(u.Hostname()); err == nil && host.IsLoopback() {
		se.SetIncludeLoopbackCandidate(true)
		se.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() }) // nothing but 127.0.0.1 leaves this process
	}
	me, err := newMediaEngine()
	if err != nil {
		return rep, err
	}
	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return rep, err
	}
	defer func() { _ = pc.Close() }()

	h264File, err := os.Create(filepath.Join(out, "t0-loopback.h264"))
	if err != nil {
		return rep, err
	}
	defer func() { _ = h264File.Close() }()

	var packets, bytesIn, units atomic.Int64
	trackDone := make(chan struct{})
	pc.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		defer close(trackDone)
		if err := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(tr.SSRC())}}); err != nil {
			slog.Warn("send PLI", "err", err)
		}
		var depack codecs.H264Packet
		for {
			pkt, _, err := tr.ReadRTP()
			if err != nil {
				return
			}
			packets.Add(1)
			bytesIn.Add(int64(len(pkt.Payload)))
			nal, err := depack.Unmarshal(pkt.Payload)
			if err != nil {
				slog.Warn("depacketize", "err", err)
				continue
			}
			if _, err := h264File.Write(nal); err != nil {
				slog.Error("write h264", "err", err)
				return
			}
			if pkt.Marker {
				units.Add(1)
			}
		}
	})
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		return rep, err
	}
	dc, err := pc.CreateDataChannel("control", nil)
	if err != nil {
		return rep, err
	}

	var rttMu sync.Mutex
	sent := map[string]time.Time{}
	var rtts []float64
	dc.OnMessage(func(m webrtc.DataChannelMessage) {
		n, ok := strings.CutPrefix(string(m.Data), "pong:")
		if !ok {
			return
		}
		rttMu.Lock()
		defer rttMu.Unlock()
		if t0, ok := sent[n]; ok {
			rtts = append(rtts, float64(time.Since(t0).Microseconds())/1000)
			delete(sent, n)
		}
	})
	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	dc.OnOpen(func() {
		go func() {
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			for i := 1; ; i++ {
				n := strconv.Itoa(i)
				rttMu.Lock()
				sent[n] = time.Now()
				rttMu.Unlock()
				if err := dc.SendText("ping:" + n); err != nil {
					return
				}
				select {
				case <-pingCtx.Done():
					return
				case <-tick.C:
				}
			}
		}()
	})

	start := time.Now()
	connected := make(chan struct{})
	var once sync.Once
	pc.OnICEConnectionStateChange(func(st webrtc.ICEConnectionState) {
		if st == webrtc.ICEConnectionStateConnected {
			once.Do(func() {
				rep.ConnectedMs = float64(time.Since(start).Microseconds()) / 1000
				close(connected)
			})
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return rep, err
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		return rep, err
	}
	<-gathered
	answer, err := exchange(base, tok, pc.LocalDescription())
	if err != nil {
		return rep, err
	}
	if err := pc.SetRemoteDescription(*answer); err != nil {
		return rep, err
	}

	select {
	case <-connected:
	case <-time.After(10 * time.Second):
		return rep, errors.New("ICE did not connect within 10 s")
	case <-ctx.Done():
		return rep, ctx.Err()
	}
	select {
	case <-time.After(dur):
	case <-ctx.Done():
		return rep, ctx.Err()
	}
	stopPing()
	_ = pc.Close()
	select {
	case <-trackDone:
	case <-time.After(3 * time.Second):
		return rep, errors.New("no video track arrived")
	}

	rep.Packets, rep.Bytes, rep.AccessUnits = packets.Load(), bytesIn.Load(), units.Load()
	rttMu.Lock()
	defer rttMu.Unlock()
	rep.pongs = len(rtts)
	if len(rtts) > 0 {
		sort.Float64s(rtts)
		rep.RTTMedianMs = rtts[len(rtts)/2]
	}
	return rep, nil
}

func exchange(base, tok string, offer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	body, err := json.Marshal(offer)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/offer?t=%s", base, url.QueryEscape(tok))
	resp, err := http.Post(u, "application/json", bytes.NewReader(body)) //nolint:gosec // spike: URL is the operator's own --server
	if err != nil {
		return nil, fmt.Errorf("post offer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("post offer: status %d", resp.StatusCode)
	}
	var answer webrtc.SessionDescription
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&answer); err != nil {
		return nil, fmt.Errorf("decode answer: %w", err)
	}
	return &answer, nil
}
