package main

import (
	"os"
	"strings"
	"testing"
)

// MOB-04: docs/launcher-app.md is the static copy the website repo renders at portty.app/app.
// It must keep every item the phone user needs, and must stay a page that holds no token, input or socket.
func TestLauncherCopy(t *testing.T) {
	b, err := os.ReadFile("docs/launcher-app.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, want := range []string{
		"https://<your-mac>.<your-tailnet>.ts.net:7444/sessions/",
		"Add to Home Screen",
		"Open as Web App",
		"Portty",
		"iCloud",
		"Messages",
		"Universal Clipboard",
		"AirDrop",
		"This link only works on your own devices",
		"Link No Longer Works",
		"Ctrl",
		"Treat this link like a password. It only opens on devices signed in to your Tailscale account.",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/launcher-app.md is missing %q", want)
		}
	}
	for _, banned := range []string{"<input", "<form", "<script", "wss://", "cap=", "?session="} {
		if strings.Contains(doc, banned) {
			t.Errorf("docs/launcher-app.md must stay static but contains %q", banned)
		}
	}
}
