package synth

import "midi-gen/midi"

// ActiveNote tracks a currently sounding oscillator.
type ActiveNote struct {
	Freq      float64
	Phase     float64 // current sine phase
	Remaining int     // samples remaining
	FadeIn    int     // samples left in fade-in ramp
	FadeOut   int     // samples to fade out before release
}

// Scheduler converts MIDI events into a stream of PCM samples.
type Scheduler struct {
	BPM        int
	TicksPerQN int
	Active     []*ActiveNote
}

// TicksToSamples converts a number of MIDI ticks to PCM sample count.
func (s *Scheduler) TicksToSamples(ticks uint32) int {
	// TODO: (ticks / TicksPerQN) * (60 / BPM) * SampleRate
	return 0
}

// Render walks the event list and returns a flat slice of interleaved stereo float32 PCM.
func (s *Scheduler) Render(events []midi.Event) []float32 {
	// TODO:
	// 1. Convert events to absolute tick times
	// 2. Walk sample-by-sample, activating/deactivating notes at the right sample offset
	// 3. Sum all active oscillators per sample
	// 4. Apply fade in/out to avoid clicks
	return nil
}
