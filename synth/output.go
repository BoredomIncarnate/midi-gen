package synth

import "midi-gen/midi"

// Play renders the MIDI events to PCM and plays them through the system audio output.
// Requires github.com/ebitengine/oto/v3 — add with: go get github.com/ebitengine/oto/v3
func Play(events []midi.Event, bpm int, ticksPerQN int) error {
	// TODO Phase 4:
	// 1. Initialize oto context (once, 44100 Hz, stereo, float32)
	// 2. Create Scheduler and call Render()
	// 3. Write PCM buffer to oto player
	// 4. Wait for playback to finish
	return nil
}
