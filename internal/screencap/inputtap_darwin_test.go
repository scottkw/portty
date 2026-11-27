package screencap

import (
	"errors"
	"testing"
)

func TestInputTap_RefusesInTests(t *testing.T) {
	if err := StartInputTap(); !errors.Is(err, ErrInputTapInTests) {
		t.Fatalf("StartInputTap = %v, want ErrInputTapInTests", err)
	}
	if running, _ := InputTapState(); running {
		t.Fatal("a tap is running after a refused start")
	}
	if r := PhysicalState(); r.Healthy || r.Seen {
		t.Fatalf("PhysicalState = %+v with no tap, want unhealthy and unseen", r)
	}
	_ = SecureInputOn() // a plain read; starts nothing
}
