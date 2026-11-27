package desktop

// stats.go: the counts-and-states snapshot behind GET /desktop/stats on the daemon
// socket (23-09). It feeds the VM rows and the soak sampler and carries no identity,
// name, address, token, SDP or password: a test pins the closed key set.

import "runtime"

// Stats is the snapshot. Audio is "ok" or "failed"; Quality and Unlock are the
// controllers' own count-and-state records.
type Stats struct {
	Goroutines int          `json:"goroutines"`
	Viewers    int          `json:"viewers"`
	OwnViewers int          `json:"own_viewers"`
	State      string       `json:"state"`
	Audio      string       `json:"audio"`
	Quality    QualityStats `json:"quality"`
	Still      bool         `json:"still"`
	Unlock     UnlockStats  `json:"unlock"`
}

// Stats gathers the snapshot.
func (m *Manager) Stats() Stats {
	st := m.Status()
	q := m.QualityStats()
	audio := "ok"
	if m.audioFail.Load() {
		audio = "failed"
	}
	return Stats{
		Goroutines: runtime.NumGoroutine(),
		Viewers:    st.Viewers,
		OwnViewers: m.ownViewers(),
		State:      string(st.State),
		Audio:      audio,
		Quality:    q,
		Still:      q.Still,
		Unlock:     m.UnlockStats(),
	}
}
