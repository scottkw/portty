// Command wailsshell is a throwaway Wails v2 shell for the desktop feasibility
// spike (R13). It loads the spike's viewer page from the embedded assets (the
// wails:// origin a Desktop tab in the Portty app would use) and signals
// through a Go-side proxy of POST /offer and POST /report, so the page never
// holds the token and never talks to the spike directly. With --navigate the
// window instead loads the spike's http viewer URL, as the fallback attempt.
package main

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var embedded embed.FS

const bodyCap = 4 << 20

func main() {
	server := flag.String("server", "", "base URL of the spike, e.g. http://<tailnet address>:47800")
	token := flag.String("token", "", "spike token, appended by the proxy")
	ice := flag.String("ice", "lite", "ICE mode of the viewer session: lite or full")
	row := flag.String("row", "R13", "report row name")
	tag := flag.String("tag", "wails", "report tag")
	navigate := flag.Bool("navigate", false, "load the http viewer URL instead of the embedded page")
	flag.Parse()

	if *server == "" || *token == "" || (*ice != "lite" && *ice != "full") {
		fmt.Fprintln(os.Stderr, "wailsshell: --server, --token and --ice lite|full are required")
		os.Exit(2)
	}
	base := strings.TrimRight(*server, "/")

	assets, err := fs.Sub(embedded, "frontend/dist")
	if err != nil {
		log.Fatalf("wailsshell: assets: %v", err)
	}

	var once sync.Once
	err = wails.Run(&options.App{
		Title:  "Portty Spike Shell",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: proxy(base, *token, *ice, *row, *tag),
		},
		OnDomReady: func(ctx context.Context) {
			if !*navigate {
				return
			}
			// Once: the http page is a new load and must not be navigated again.
			once.Do(func() {
				target := fmt.Sprintf("%s/viewer.html?t=%s&row=%s&tag=%s-http&ice=%s",
					base, url.QueryEscape(*token), url.QueryEscape(*row), url.QueryEscape(*tag), url.QueryEscape(*ice))
				runtime.WindowExecJS(ctx, "window.location.href = "+jsString(target))
			})
		},
	})
	if err != nil {
		log.Fatalf("wailsshell: %v", err)
	}
}

// jsString quotes s as a JavaScript string literal.
func jsString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// proxy serves only POST /offer and POST /report, forwards the body to the one
// configured spike with the token and session parameters appended, and answers
// 404 for every other request. The body is capped and the upstream call has a
// 10 s timeout.
func proxy(base, token, ice, row, tag string) http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}
	q := url.Values{"t": {token}, "ice": {ice}, "row": {row}, "tag": {tag}}.Encode()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || (r.URL.Path != "/offer" && r.URL.Path != "/report") {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyCap))
		if err != nil {
			log.Printf("wailsshell: %s: body: %v", r.URL.Path, err)
			http.Error(w, "body too large or unreadable", http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("wailsshell: %s from origin %q, body %d bytes", r.URL.Path, r.Header.Get("Origin"), len(body))
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, base+r.URL.Path+"?"+q, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "bad upstream request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("wailsshell: %s: upstream: %v", r.URL.Path, err)
			http.Error(w, "upstream unreachable", http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, io.LimitReader(resp.Body, bodyCap)); err != nil {
			log.Printf("wailsshell: %s: relay: %v", r.URL.Path, err)
		}
	})
}
