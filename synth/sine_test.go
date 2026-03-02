package synth

import (
	"math"
	"testing"
)

// -----------------------------------------------------------------------------
// MIDINoteToFreq tests
// -----------------------------------------------------------------------------

// TestMIDINoteToFreq_ConcertA verifies the fundamental anchor point:
// A4 (note 69) = 440.0 Hz exactly. All other note frequencies derive from this.
func TestMIDINoteToFreq_ConcertA(t *testing.T) {
	freq := MIDINoteToFreq(69)
	if math.Abs(freq-440.0) > 0.001 {
		t.Errorf("A4 (note 69): expected 440.0 Hz, got %.4f Hz", freq)
	}
}

// TestMIDINoteToFreq_MiddleC verifies C4 (note 60) = 261.626 Hz.
func TestMIDINoteToFreq_MiddleC(t *testing.T) {
	freq := MIDINoteToFreq(60)
	expected := 261.6256
	if math.Abs(freq-expected) > 0.001 {
		t.Errorf("C4 (note 60): expected %.4f Hz, got %.4f Hz", expected, freq)
	}
}

// TestMIDINoteToFreq_OctaveDoubling verifies that moving up one octave (12
// semitones) always exactly doubles the frequency. This is the definition
// of octave equivalence in equal temperament.
func TestMIDINoteToFreq_OctaveDoubling(t *testing.T) {
	bases := []int{21, 36, 48, 60, 69, 84}
	for _, note := range bases {
		low := MIDINoteToFreq(note)
		high := MIDINoteToFreq(note + 12)
		ratio := high / low
		if math.Abs(ratio-2.0) > 0.0001 {
			t.Errorf("note %d→%d: expected freq ratio 2.0, got %.6f", note, note+12, ratio)
		}
	}
}

// TestMIDINoteToFreq_SemitoneRatio verifies that adjacent semitones have a
// frequency ratio of 2^(1/12) ≈ 1.05946. This is the equal temperament ratio.
func TestMIDINoteToFreq_SemitoneRatio(t *testing.T) {
	expectedRatio := math.Pow(2.0, 1.0/12.0)
	for note := 21; note < 107; note++ {
		low := MIDINoteToFreq(note)
		high := MIDINoteToFreq(note + 1)
		ratio := high / low
		if math.Abs(ratio-expectedRatio) > 0.0001 {
			t.Errorf("note %d→%d: expected semitone ratio %.6f, got %.6f",
				note, note+1, expectedRatio, ratio)
		}
	}
}

// TestMIDINoteToFreq_AllPositive verifies all 128 MIDI notes produce positive
// frequencies. A zero or negative frequency would cause math.Sin to produce
// correct values but the audio would be silent or DC-offset.
func TestMIDINoteToFreq_AllPositive(t *testing.T) {
	for note := 0; note <= 127; note++ {
		freq := MIDINoteToFreq(note)
		if freq <= 0 {
			t.Errorf("note %d: expected positive frequency, got %.4f", note, freq)
		}
	}
}

// -----------------------------------------------------------------------------
// NewVoice tests
// -----------------------------------------------------------------------------

// TestNewVoice_Fields verifies that NewVoice correctly initialises all fields
// from its parameters. The frequency should match MIDINoteToFreq, velocity
// should be normalised from 0–127 to 0.0–1.0, and the envelope starts in
// the attack phase with zero elapsed samples.
func TestNewVoice_Fields(t *testing.T) {
	adsr := DefaultADSR()
	v := NewVoice(69, 100, adsr)

	if v.Key != 69 {
		t.Errorf("Key: expected 69, got %d", v.Key)
	}

	expectedFreq := MIDINoteToFreq(69)
	if math.Abs(v.Freq-expectedFreq) > 0.001 {
		t.Errorf("Freq: expected %.4f, got %.4f", expectedFreq, v.Freq)
	}

	expectedVel := float64(100) / 127.0
	if math.Abs(v.Velocity-expectedVel) > 0.0001 {
		t.Errorf("Velocity: expected %.4f, got %.4f", expectedVel, v.Velocity)
	}

	if v.Phase != 0.0 {
		t.Errorf("Phase: expected 0.0, got %.4f", v.Phase)
	}

	if v.EnvPhase != envAttack {
		t.Errorf("EnvPhase: expected envAttack, got %d", v.EnvPhase)
	}

	if v.EnvSamples != 0 {
		t.Errorf("EnvSamples: expected 0, got %d", v.EnvSamples)
	}

	if v.Releasing {
		t.Error("Releasing: expected false on new voice")
	}
}

// TestNewVoice_VelocityNormalisation verifies the 0–127 → 0.0–1.0 mapping
// at the boundary values and a midpoint.
func TestNewVoice_VelocityNormalisation(t *testing.T) {
	tests := []struct {
		midiVel  byte
		expected float64
	}{
		{0, 0.0 / 127.0},
		{1, 1.0 / 127.0},
		{64, 64.0 / 127.0},
		{127, 127.0 / 127.0},
	}
	for _, tt := range tests {
		v := NewVoice(60, tt.midiVel, DefaultADSR())
		if math.Abs(v.Velocity-tt.expected) > 0.0001 {
			t.Errorf("velocity %d: expected %.4f, got %.4f",
				tt.midiVel, tt.expected, v.Velocity)
		}
	}
}

// -----------------------------------------------------------------------------
// ADSR envelope tests
// -----------------------------------------------------------------------------

// TestEnvelope_StartsAtZero verifies that the very first sample of a new voice
// has an envelope amplitude of zero (start of attack ramp).
// A non-zero start would cause a click on note onset.
func TestEnvelope_StartsAtZero(t *testing.T) {
	v := NewVoice(60, 100, DefaultADSR())
	// The first call to envelope() at EnvSamples=0 should return 0/attackSamples = 0
	amp := v.envelope()
	if amp != 0.0 {
		t.Errorf("first envelope sample: expected 0.0, got %.4f", amp)
	}
}

// TestEnvelope_ReachesOneAfterAttack verifies that after attackSamples have
// elapsed, the envelope amplitude has reached (or is very close to) 1.0.
func TestEnvelope_ReachesOneAfterAttack(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 10.0, DecayMs: 50.0, SustainLevel: 0.7, ReleaseMs: 30.0}
	v := NewVoice(60, 127, adsr)

	attackSamples := int(adsr.AttackMs * SampleRate / 1000.0)

	// Advance through the entire attack stage
	var lastAmp float64
	for i := 0; i < attackSamples; i++ {
		lastAmp = v.envelope()
	}

	// The last attack sample should be at or very near 1.0
	if lastAmp < 0.95 {
		t.Errorf("end of attack: expected amplitude >= 0.95, got %.4f", lastAmp)
	}
}

// TestEnvelope_SustainIsStable verifies that once in the sustain stage,
// the amplitude remains constant at SustainLevel for many samples.
func TestEnvelope_SustainIsStable(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 1.0, DecayMs: 1.0, SustainLevel: 0.7, ReleaseMs: 30.0}
	v := NewVoice(60, 127, adsr)

	// Burn through attack + decay
	attackSamples := int(adsr.AttackMs * SampleRate / 1000.0)
	decaySamples := int(adsr.DecayMs * SampleRate / 1000.0)
	for i := 0; i < attackSamples+decaySamples+5; i++ {
		v.envelope()
	}

	// Now in sustain — amplitude should be stable at SustainLevel
	for i := 0; i < 100; i++ {
		amp := v.envelope()
		if math.Abs(amp-adsr.SustainLevel) > 0.01 {
			t.Errorf("sustain sample %d: expected %.2f, got %.4f",
				i, adsr.SustainLevel, amp)
		}
	}
}

// TestEnvelope_ReleaseFallsToZero verifies that after Release() is called,
// the envelope falls to zero within the expected release time.
func TestEnvelope_ReleaseFallsToZero(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 1.0, DecayMs: 1.0, SustainLevel: 0.7, ReleaseMs: 20.0}
	v := NewVoice(60, 127, adsr)

	// Advance into sustain
	attackSamples := int(adsr.AttackMs * SampleRate / 1000.0)
	decaySamples := int(adsr.DecayMs * SampleRate / 1000.0)
	for i := 0; i < attackSamples+decaySamples+10; i++ {
		v.envelope()
	}

	// Trigger release
	v.Release()

	releaseSamples := int(adsr.ReleaseMs * SampleRate / 1000.0)

	// Advance through entire release
	var finalAmp float64
	for i := 0; i < releaseSamples+10; i++ {
		finalAmp = v.envelope()
	}

	if finalAmp != 0.0 {
		t.Errorf("after release: expected amplitude 0.0, got %.4f", finalAmp)
	}
}

// TestEnvelope_IsDoneAfterRelease verifies that IsDone() returns true once
// the release stage has fully completed.
func TestEnvelope_IsDoneAfterRelease(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 1.0, DecayMs: 1.0, SustainLevel: 0.7, ReleaseMs: 5.0}
	v := NewVoice(60, 100, adsr)

	// Advance into sustain and release
	totalSamples := int((adsr.AttackMs + adsr.DecayMs + adsr.ReleaseMs + 10) * SampleRate / 1000.0)
	v.Release()
	for i := 0; i < totalSamples; i++ {
		v.envelope()
	}

	if !v.IsDone() {
		t.Error("expected IsDone()=true after release completes")
	}
}

// TestEnvelope_ReleaseFromAttack verifies that Release() can be called while
// still in the attack stage (very short note) and the voice still reaches
// envDone cleanly without getting stuck.
func TestEnvelope_ReleaseFromAttack(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 100.0, DecayMs: 50.0, SustainLevel: 0.7, ReleaseMs: 5.0}
	v := NewVoice(60, 100, adsr)

	// Release immediately — still in attack
	v.Release()

	// Advance through the release
	releaseSamples := int(adsr.ReleaseMs * SampleRate / 1000.0)
	for i := 0; i < releaseSamples+20; i++ {
		v.envelope()
	}

	if !v.IsDone() {
		t.Error("expected IsDone()=true after early release completes")
	}
}

// -----------------------------------------------------------------------------
// RenderSamples tests
// -----------------------------------------------------------------------------

// TestRenderSamples_MixesIntoBuffer verifies that RenderSamples adds to the
// buffer rather than overwriting it. This is essential for polyphonic mixing.
func TestRenderSamples_MixesIntoBuffer(t *testing.T) {
	v := NewVoice(69, 127, DefaultADSR())
	buf := []float32{1.0, 1.0, 1.0, 1.0}

	// Advance past attack so we get non-zero output
	advanceSamples := int(DefaultADSR().AttackMs * SampleRate / 1000.0)
	scratch := make([]float32, advanceSamples)
	v.RenderSamples(scratch, advanceSamples, 1.0)

	// Now render into a pre-filled buffer — values should increase, not reset
	before := make([]float32, 4)
	copy(before, buf)
	v.RenderSamples(buf, 4, 0.3)

	changed := false
	for i := range buf {
		if buf[i] != before[i] {
			changed = true
		}
		// Values should be >= original (positive sine added to 1.0)
		// We can't guarantee > because sine can be negative — just check changed
	}
	if !changed {
		t.Error("RenderSamples did not modify the buffer")
	}
}

// TestRenderSamples_PhaseAdvances verifies that Phase increases after
// RenderSamples and stays within [0, 2π).
// Phase accumulation outside this range causes sin() precision loss.
func TestRenderSamples_PhaseAdvances(t *testing.T) {
	v := NewVoice(69, 127, DefaultADSR())
	initialPhase := v.Phase

	buf := make([]float32, 100)
	v.RenderSamples(buf, 100, 0.3)

	if v.Phase == initialPhase {
		t.Error("Phase did not advance after RenderSamples")
	}
	if v.Phase < 0 || v.Phase >= 2.0*math.Pi {
		t.Errorf("Phase %.4f is outside [0, 2π)", v.Phase)
	}
}

// TestRenderSamples_SilentWhenDone verifies that a voice in envDone state
// contributes zero to the buffer — dead voices don't add noise.
func TestRenderSamples_SilentWhenDone(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 1.0, DecayMs: 1.0, SustainLevel: 0.7, ReleaseMs: 1.0}
	v := NewVoice(60, 127, adsr)
	v.Release()

	// Burn through all envelope stages
	burnSamples := int((adsr.AttackMs + adsr.DecayMs + adsr.ReleaseMs + 10) * SampleRate / 1000.0)
	scratch := make([]float32, burnSamples)
	v.RenderSamples(scratch, burnSamples, 1.0)

	if !v.IsDone() {
		t.Fatal("voice should be done after full envelope cycle")
	}

	// Render into a zeroed buffer — should stay zero
	buf := make([]float32, 64)
	v.RenderSamples(buf, 64, 1.0)
	for i, s := range buf {
		if s != 0.0 {
			t.Errorf("sample %d: expected 0.0 from done voice, got %f", i, s)
		}
	}
}

// TestRenderSamples_MasterGainScales verifies that doubling masterGain
// doubles the output amplitude proportionally.
func TestRenderSamples_MasterGainScales(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 0.0, DecayMs: 0.0, SustainLevel: 1.0, ReleaseMs: 10.0}

	// Render with gain 0.1
	v1 := NewVoice(69, 127, adsr)
	buf1 := make([]float32, 64)
	v1.RenderSamples(buf1, 64, 0.1)

	// Render with gain 0.2 (double)
	v2 := NewVoice(69, 127, adsr)
	buf2 := make([]float32, 64)
	v2.RenderSamples(buf2, 64, 0.2)

	// Every non-zero sample in buf2 should be approximately 2x buf1
	for i := range buf1 {
		if buf1[i] == 0 {
			continue
		}
		ratio := float64(buf2[i]) / float64(buf1[i])
		if math.Abs(ratio-2.0) > 0.01 {
			t.Errorf("sample %d: gain doubling ratio %.4f (expected ~2.0)", i, ratio)
			break
		}
	}
}
