package screencap

import (
	"slices"
	"testing"
)

func names(list []SurfaceInfo) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.Name)
	}
	return out
}

func TestBuildSurfaces_Names(t *testing.T) {
	got := BuildSurfaces([]RawDisplay{{ID: 9, W: 1920, H: 1080}, {ID: 7, W: 1470, H: 956}, {ID: 3, X: -1280, W: 1280, H: 720}}, nil, 7)
	if want := []string{"Main Display", "Display 2", "Display 3"}; !slices.Equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	for i, id := range []uint32{7, 3, 9} {
		if got[i].ID != id || got[i].Kind != SurfaceDisplay || got[i].Main != (id == 7) {
			t.Errorf("entry %d = %+v, want display %d", i, got[i], id)
		}
	}
	if got[0].W != 1470 || got[0].H != 956 || got[1].X != -1280 {
		t.Errorf("sizes and origins must pass through in points: %+v", got)
	}
}

func TestBuildSurfaces_WindowFilter(t *testing.T) {
	ok := func(id uint32, app, title string) RawWindow {
		return RawWindow{ID: id, PID: 10, App: app, Title: title, W: 800, H: 600, OnScreen: true}
	}
	layer := ok(1, "Dock", "x")
	layer.Layer = 25
	noPID := ok(2, "Ghost", "x")
	noPID.PID = 0
	thin := ok(3, "Strip", "x")
	thin.W, thin.H = 40, 900
	blank := ok(4, "", "")
	untitled := ok(5, "Finder", "")
	own := ok(6, "Portty", "Terminal")
	hidden := ok(7, "Notes", "Todo")
	hidden.OnScreen = false

	got := BuildSurfaces(nil, []RawWindow{layer, noPID, thin, blank, untitled, own, hidden}, 0)
	if want := []string{"Finder", "Notes — Todo", "Portty — Terminal"}; !slices.Equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	if got[1].OnScreen {
		t.Error("an off-screen window must keep OnScreen false")
	}
	if !got[0].OnScreen || got[0].PID != 10 || got[0].App != "Finder" || got[0].Title != "" {
		t.Errorf("entry = %+v", got[0])
	}
}

func TestBuildSurfaces_SortAndName(t *testing.T) {
	w := func(id uint32, app, title string) RawWindow {
		return RawWindow{ID: id, PID: 1, App: app, Title: title, W: 300, H: 300, OnScreen: true}
	}
	got := BuildSurfaces(nil, []RawWindow{w(5, "terminal", "b"), w(4, "Safari", "z"), w(3, "Terminal", "a"), w(2, "Terminal", "b"), w(1, "", "Untitled app")}, 0)
	want := []string{"Untitled app", "Safari — z", "Terminal — a", "Terminal — b", "terminal — b"}
	if !slices.Equal(names(got), want) {
		t.Fatalf("names = %v, want %v", names(got), want)
	}
	if got[3].ID != 2 || got[4].ID != 5 {
		t.Errorf("equal app and title (case aside) must sort by ID, got %d then %d", got[3].ID, got[4].ID)
	}
	if got := BuildSurfaces(nil, []RawWindow{w(1, "Terminal", "zsh")}, 0); got[0].Name != "Terminal — zsh" {
		t.Errorf("name = %q, want the app, U+2014 and the title with one space each side", got[0].Name)
	}
}

func TestDecodeSurfaces(t *testing.T) {
	got, err := decodeSurfaces([]byte(`{"main":2,"displays":[{"id":2,"x":0,"y":0,"w":1470,"h":956}],"windows":[{"id":9,"pid":5,"app":"Terminal","title":"zsh","layer":0,"x":1,"y":2,"w":800,"h":600,"onScreen":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Main Display" || got[1].Name != "Terminal \u2014 zsh" || got[1].PID != 5 || got[1].X != 1 || !got[1].OnScreen {
		t.Errorf("decoded %+v", got)
	}
	if _, err := decodeSurfaces([]byte(`{`)); err == nil {
		t.Error("a truncated list must be an error, not an empty list")
	}
}
