// Command fakegh is a loopback-only fake of the GitHub Releases API for the M-92 update checks
// in the Tart VMs (see TESTING.md). A vmpreview build of Portty checks it instead of GitHub.
// It is a test tool: never imported by the app and never part of a release.
//
//	fakegh -scenario newer -version 9.9.9 -dmg ~/ptest/good.dmg
//
// Scenarios: newer, current, draft, prerelease, noasset, baddigest, 404, ratelimit.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address (loopback only)")
	scenario := flag.String("scenario", "newer", "newer|current|draft|prerelease|noasset|baddigest|404|ratelimit")
	version := flag.String("version", "9.9.9", "release version, without the v")
	dmg := flag.String("dmg", "", "DMG served as the release asset (required for newer and baddigest)")
	flag.Parse()

	host, _, err := net.SplitHostPort(*addr)
	if err != nil || (host != "127.0.0.1" && host != "::1" && host != "localhost") {
		log.Fatalf("fakegh: -addr %q is not a loopback address", *addr)
	}
	switch *scenario {
	case "newer", "current", "draft", "prerelease", "noasset", "baddigest", "404", "ratelimit":
	default:
		log.Fatalf("fakegh: unknown scenario %q", *scenario)
	}

	var data []byte
	if *dmg != "" {
		if data, err = os.ReadFile(*dmg); err != nil {
			log.Fatalf("fakegh: %v", err)
		}
	}
	if len(data) == 0 && (*scenario == "newer" || *scenario == "baddigest") {
		log.Fatalf("fakegh: a non-empty -dmg is required for %s", *scenario)
	}

	name := "portty-v" + *version + "-darwin-universal.dmg"
	sum := sha256.Sum256(data)
	if *scenario == "baddigest" {
		flipped := append([]byte(nil), data...)
		flipped[0] ^= 0xff
		sum = sha256.Sum256(flipped)
	}
	assets := []map[string]any{{
		"name":                 name,
		"size":                 len(data),
		"state":                "uploaded",
		"digest":               "sha256:" + hex.EncodeToString(sum[:]),
		"browser_download_url": "http://" + *addr + "/download/" + name,
	}}
	if *scenario == "noasset" {
		assets = []map[string]any{}
	}
	release, _ := json.Marshal(map[string]any{
		"tag_name":   "v" + *version,
		"draft":      *scenario == "draft",
		"prerelease": *scenario == "prerelease",
		"assets":     assets,
	})
	etag := fmt.Sprintf("%q", fmt.Sprintf("%s-%s-%d", *scenario, *version, len(data)))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/scottkw/portty/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case *scenario == "404":
			http.NotFound(w, r)
		case *scenario == "ratelimit":
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
		case r.Header.Get("If-None-Match") == etag:
			w.WriteHeader(http.StatusNotModified)
		default:
			w.Header().Set("ETag", etag)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(release)
		}
	})
	mux.HandleFunc("GET /download/{name}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("name") != name || data == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
	})

	log.Printf("fakegh: %s on %s (%s, %d bytes)", *scenario, *addr, name, len(data))
	if err := http.ListenAndServe(*addr, logRequests(mux)); err != nil {
		log.Fatalf("fakegh: %v", err)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }

// logRequests prints one stdout line per request: time, method, path, inm=yes|no, status.
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(sw, r)
		inm := "no"
		if r.Header.Get("If-None-Match") != "" {
			inm = "yes"
		}
		fmt.Printf("%s %s %s inm=%s %d\n", time.Now().Format(time.RFC3339), r.Method, r.URL.Path, inm, sw.status)
	})
}
