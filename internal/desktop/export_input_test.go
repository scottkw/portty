package desktop

// export_input_test.go: test-only access to the input path for the external
// tests, kept apart from export_test.go (another plan edits that one).

// InputViewerForTest is a viewer that is not registered with the Manager: it
// has no socket and the periodic checks never see it.
type InputViewerForTest = viewer

// NewInputViewerForTest returns an unregistered viewer with the given rights.
func NewInputViewerForTest(id string, rights Rights) *InputViewerForTest {
	v := &viewer{Viewer: Viewer{ID: ViewerID(id), Rights: rights}, in: &viewerInput{}}
	v.setRightsBits(rights)
	return v
}

// HandleInputForTest parses raw and handles it synchronously on the caller's
// goroutine, as the input goroutine would.
func (m *Manager) HandleInputForTest(v *InputViewerForTest, raw string) error {
	msg, err := parseInput([]byte(raw))
	if err != nil {
		return err
	}
	m.handleInput(v, msg)
	return nil
}

// SetControlInputForTest sets the input control value.
func (m *Manager) SetControlInputForTest(s string) { m.setControlInput(s) }

// SetControlWindowForTest sets the window control value, as the surface plan does.
func (m *Manager) SetControlWindowForTest(s string) { m.setControlWindow(s) }

// InputChannelsForTest is how many viewers' input channels the Manager counts as open.
func (m *Manager) InputChannelsForTest() int { return len(m.inputChannels()) }

// SetRightsForTest sets the live rights of an unregistered test viewer.
func SetRightsForTest(v *InputViewerForTest, r Rights) { v.Rights = r; v.setRightsBits(r) }
