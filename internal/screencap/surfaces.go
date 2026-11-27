package screencap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// minWindowPoints is the smallest side of a window worth offering: anything
// smaller is a helper or a shadow, not something a person would pick.
const minWindowPoints = 64

// RawDisplay is one active display as the native list reports it, in points.
type RawDisplay struct {
	ID uint32  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
}

// RawWindow is one window as the native list reports it, in points.
type RawWindow struct {
	ID       uint32  `json:"id"`
	PID      int32   `json:"pid"`
	App      string  `json:"app"`
	Title    string  `json:"title"`
	Layer    int     `json:"layer"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
	OnScreen bool    `json:"onScreen"`
}

// decodeSurfaces turns the native list's JSON into the surfaces on offer.
func decodeSurfaces(b []byte) ([]SurfaceInfo, error) {
	var raw struct {
		Main     uint32       `json:"main"`
		Displays []RawDisplay `json:"displays"`
		Windows  []RawWindow  `json:"windows"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("screencap: surface list: %w", err)
	}
	return BuildSurfaces(raw.Displays, raw.Windows, raw.Main), nil
}

// BuildSurfaces names, filters and sorts the raw lists into what a viewer is
// offered: the main display first as "Main Display", the others as "Display 2",
// "Display 3" in ascending ID order, then the normal windows (layer 0, an
// owning process, at least 64 points each way, a title or an app name) named
// "App — Title" or just "App", sorted by app, title and ID. Windows that are
// minimized or off screen are kept with OnScreen false so a caller can tell
// them from a closed one.
func BuildSurfaces(displays []RawDisplay, windows []RawWindow, mainID uint32) []SurfaceInfo {
	out := make([]SurfaceInfo, 0, len(displays)+len(windows))
	rest := make([]RawDisplay, 0, len(displays))
	for _, d := range displays {
		if d.ID == mainID {
			out = append(out, SurfaceInfo{Kind: SurfaceDisplay, ID: d.ID, Name: "Main Display", X: d.X, Y: d.Y, W: d.W, H: d.H, Main: true, OnScreen: true})
		} else {
			rest = append(rest, d)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].ID < rest[j].ID })
	for i, d := range rest {
		out = append(out, SurfaceInfo{Kind: SurfaceDisplay, ID: d.ID, Name: "Display " + strconv.Itoa(i+2), X: d.X, Y: d.Y, W: d.W, H: d.H, OnScreen: true})
	}

	var wins []SurfaceInfo
	for _, w := range windows {
		if w.Layer != 0 || w.PID == 0 || w.W < minWindowPoints || w.H < minWindowPoints || (w.App == "" && w.Title == "") {
			continue
		}
		name := w.App
		switch {
		case w.App != "" && w.Title != "":
			name = w.App + " — " + w.Title
		case w.App == "":
			name = w.Title
		}
		wins = append(wins, SurfaceInfo{Kind: SurfaceWindow, ID: w.ID, PID: w.PID, Name: name, App: w.App, Title: w.Title, X: w.X, Y: w.Y, W: w.W, H: w.H, OnScreen: w.OnScreen})
	}
	sort.Slice(wins, func(i, j int) bool {
		a, b := wins[i], wins[j]
		if x, y := strings.ToLower(a.App), strings.ToLower(b.App); x != y {
			return x < y
		}
		if x, y := strings.ToLower(a.Title), strings.ToLower(b.Title); x != y {
			return x < y
		}
		return a.ID < b.ID
	})
	return append(out, wins...)
}
