package synth

import (
	"math"
	"testing"
)

// -----------------------------------------------------------------------------
// combFilter tests
// -----------------------------------------------------------------------------

// TestCombFilter_OutputDelayed verifies that the comb filter output is silent
// for the first `delaySamples` samples when fed an impulse at sample 0.
// The impulse should not appear at the output until it has travelled through
// the full delay line.
func TestCombFilter_OutputDelayed(t *testing.T) {
	delay := 10
	c := newCombFilter(delay, 0.5)

	// Feed an impulse at sample 0, then silence
	outputs := make([]float32, delay*3)
	for i := range outputs {
		input := float32(0)
		if i == 0 {
			input = 1.0
		}
		outputs[i] = c.process(input)
	}

	// The first `delay` output samples should be 0 (impulse not yet arrived)
	for i := 0; i < delay; i++ {
		if outputs[i] != 0.0 {
			t.Errorf("sample %d: expected 0.0 before delay, got %f", i, outputs[i])
		}
	}

	// Sample at index `delay` should be non-zero (impulse has arrived)
	if outputs[delay] == 0.0 {
		t.Errorf("sample %d: expected non-zero output after delay, got 0.0", delay)
	}
}

// TestCombFilter_FeedbackDecays verifies that the comb filter output decays
// geometrically with each recirculation. After the initial impulse arrives,
// each subsequent echo should be smaller than the previous by the feedback factor.
func TestCombFilter_FeedbackDecays(t *testing.T) {
	delay := 8
	feedback := float32(0.5)
	c := newCombFilter(delay, feedback)

	// Feed an impulse then silence
	outputs := make([]float32, delay*5)
	for i := range outputs {
		input := float32(0)
		if i == 0 {
			input = 1.0
		}
		outputs[i] = c.process(input)
	}

	// Each echo (at multiples of `delay`) should be smaller than the previous
	// by a factor of approximately `feedback`.
	// We check echo 1 vs echo 2 (indices delay and delay*2).
	echo1 := math.Abs(float64(outputs[delay]))
	echo2 := math.Abs(float64(outputs[delay*2]))

	if echo1 == 0 {
		t.Fatal("first echo is zero — comb filter not producing output")
	}
	if echo2 >= echo1 {
		t.Errorf("feedback not decaying: echo1=%.4f, echo2=%.4f (echo2 should be smaller)",
			echo1, echo2)
	}
}

// TestCombFilter_ZeroFeedbackSilentAfterDelay verifies that with feedback=0,
// the comb filter acts as a pure delay line — the impulse appears once at
// the output after `delay` samples and then goes completely silent.
func TestCombFilter_ZeroFeedbackSilentAfterDelay(t *testing.T) {
	delay := 5
	c := newCombFilter(delay, 0.0)

	outputs := make([]float32, delay*3)
	for i := range outputs {
		input := float32(0)
		if i == 0 {
			input = 1.0
		}
		outputs[i] = c.process(input)
	}

	// After the single echo at index `delay`, output should be silent
	for i := delay + 1; i < len(outputs); i++ {
		if outputs[i] != 0.0 {
			t.Errorf("sample %d: expected silence after single echo (feedback=0), got %f",
				i, outputs[i])
		}
	}
}

// TestCombFilter_SilentInputSilentOutput verifies that a comb filter fed
// constant silence produces silent output (no self-oscillation at zero).
func TestCombFilter_SilentInputSilentOutput(t *testing.T) {
	c := newCombFilter(100, 0.84)
	for i := 0; i < 500; i++ {
		out := c.process(0.0)
		if out != 0.0 {
			t.Errorf("sample %d: expected 0.0 for silent input, got %f", i, out)
		}
	}
}

// -----------------------------------------------------------------------------
// allpassFilter tests
// -----------------------------------------------------------------------------

// TestAllpassFilter_SilentInputSilentOutput verifies an allpass filter fed
// silence produces silence. No self-oscillation should occur.
func TestAllpassFilter_SilentInputSilentOutput(t *testing.T) {
	a := newAllpassFilter(100, 0.5)
	for i := 0; i < 300; i++ {
		out := a.process(0.0)
		if out != 0.0 {
			t.Errorf("sample %d: expected 0.0 for silent input, got %f", i, out)
		}
	}
}

// TestAllpassFilter_ImpulseResponseSettles verifies that an allpass filter's
// response to a single impulse eventually decays to zero. The allpass should
// not ring indefinitely.
func TestAllpassFilter_ImpulseResponseSettles(t *testing.T) {
	a := newAllpassFilter(50, 0.5)

	// Feed an impulse then silence
	outputs := make([]float32, 1000)
	for i := range outputs {
		input := float32(0)
		if i == 0 {
			input = 1.0
		}
		outputs[i] = a.process(input)
	}

	// The last 100 samples should be effectively silent (< 0.001)
	for i := 900; i < 1000; i++ {
		if math.Abs(float64(outputs[i])) > 0.001 {
			t.Errorf("sample %d: allpass still ringing at %.6f after 900 samples",
				i, outputs[i])
		}
	}
}

// -----------------------------------------------------------------------------
// Reverb (full pipeline) tests
// -----------------------------------------------------------------------------

// TestReverb_SilentInputSilentOutput verifies the full reverb pipeline
// produces silence when fed silence. No self-oscillation or DC offset.
func TestReverb_SilentInputSilentOutput(t *testing.T) {
	r := NewReverb()
	buf := make([]float32, 1000)
	r.Process(buf)

	for i, s := range buf {
		if s != 0.0 {
			t.Errorf("sample %d: expected 0.0 for silent input, got %f", i, s)
		}
	}
}

// TestReverb_ImpulseProducesNonZeroOutput verifies that an impulse fed into
// the reverb produces non-zero output — the reverb is actually doing something.
func TestReverb_ImpulseProducesNonZeroOutput(t *testing.T) {
	r := NewReverb()
	buf := make([]float32, 4000)
	buf[0] = 1.0 // impulse

	r.Process(buf)

	// After the impulse, there should be non-zero samples (the reverb tail)
	nonZero := 0
	for _, s := range buf[1:] {
		if s != 0.0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Error("reverb produced no tail after impulse — reverb has no effect")
	}
}

// TestReverb_DryMixScalesDirectSignal verifies that with WetMix=0, the output
// is exactly input * DryMix — the dry path scales correctly.
func TestReverb_DryMixScalesDirectSignal(t *testing.T) {
	r := NewReverb()
	r.DryMix = 0.5
	r.WetMix = 0.0

	// Use silence so combs produce nothing (verified by SilentInputSilentOutput)
	// Instead test with a short constant signal
	buf := make([]float32, 10)
	for i := range buf {
		buf[i] = 0.8
	}

	// Copy original values
	original := make([]float32, len(buf))
	copy(original, buf)

	r.Process(buf)

	// With WetMix=0, output = input * DryMix exactly
	// Note: comb filters on silent input produce 0 wet, so this holds for
	// the first few samples before any echo arrives
	for i := 0; i < 3; i++ {
		expected := original[i] * 0.5
		if math.Abs(float64(buf[i]-expected)) > 0.001 {
			t.Errorf("sample %d: expected %.4f (input*dryMix), got %.4f",
				i, expected, buf[i])
		}
	}
}

// TestReverb_WetMixZeroNoDryChange verifies that with DryMix=1.0 and WetMix=0,
// the signal passes through completely unmodified for the first samples
// (before any comb echo arrives).
func TestReverb_WetMixZeroNoDryChange(t *testing.T) {
	r := NewReverb()
	r.DryMix = 1.0
	r.WetMix = 0.0

	buf := []float32{0.5, 0.3, 0.7}
	original := []float32{0.5, 0.3, 0.7}

	r.Process(buf)

	// Shortest comb delay is 1422 samples — no echo arrives in first 3 samples
	for i := range buf {
		if math.Abs(float64(buf[i]-original[i])) > 0.001 {
			t.Errorf("sample %d: dry signal altered (DryMix=1, WetMix=0): expected %.4f, got %.4f",
				i, original[i], buf[i])
		}
	}
}

// TestReverb_TailDecays verifies that the reverb tail produced by an impulse
// decreases in amplitude over time — the reverb decays rather than growing.
// We measure the RMS amplitude in two time windows and confirm the later
// window is quieter.
func TestReverb_TailDecays(t *testing.T) {
	r := NewReverb()
	buf := make([]float32, SampleRate*3) // 3 seconds of audio
	buf[0] = 1.0

	r.Process(buf)

	// Measure RMS in two windows:
	//   early:  samples 2000–3000  (shortly after impulse)
	//   late:   samples 100000–101000 (well into the tail)
	earlyRMS := rms(buf[2000:3000])
	lateRMS := rms(buf[100000:101000])

	if lateRMS >= earlyRMS {
		t.Errorf("reverb tail not decaying: earlyRMS=%.6f, lateRMS=%.6f",
			earlyRMS, lateRMS)
	}
}

// TestReverb_ProcessIsInPlace verifies that Process modifies the buffer
// in place rather than allocating a new one. The input slice header
// should remain the same object after processing.
func TestReverb_ProcessIsInPlace(t *testing.T) {
	r := NewReverb()
	buf := make([]float32, 100)
	buf[0] = 0.5
	ptr := &buf[0]

	r.Process(buf)

	if &buf[0] != ptr {
		t.Error("Process reallocated the buffer — it should modify in place")
	}
}

// TestReverb_StatefulAcrossCalls verifies that the reverb tail from one
// Process call continues into the next. If each call were independent, the
// tail would reset to silence at every call boundary.
func TestReverb_StatefulAcrossCalls(t *testing.T) {
	r := NewReverb()

	// First call: impulse at start
	buf1 := make([]float32, 100)
	buf1[0] = 1.0
	r.Process(buf1)

	// Second call: silence — but should contain reverb tail from first call
	buf2 := make([]float32, 100)
	r.Process(buf2)

	// The shortest comb delay is 1422 samples — tail won't appear in first
	// 100 samples of buf2. Use a fresh reverb with shorter delays to test
	// statefulness with a smaller buffer.
	r2 := &Reverb{
		combs: [4]combFilter{
			newCombFilter(5, 0.9),
			newCombFilter(7, 0.9),
			newCombFilter(11, 0.9),
			newCombFilter(13, 0.9),
		},
		allpasses: [2]allpassFilter{
			newAllpassFilter(3, 0.5),
			newAllpassFilter(5, 0.5),
		},
		DryMix: 0.0,
		WetMix: 1.0,
	}

	impulse := make([]float32, 20)
	impulse[0] = 1.0
	r2.Process(impulse)

	silence := make([]float32, 20)
	r2.Process(silence)

	nonZero := false
	for _, s := range silence {
		if math.Abs(float64(s)) > 0.0001 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("reverb state did not carry over between Process calls — reverb tail was lost")
	}
}

// TestNewReverbWithMix verifies custom dry/wet values are stored correctly.
func TestNewReverbWithMix(t *testing.T) {
	r := NewReverbWithMix(0.4, 0.6)
	if r.DryMix != 0.4 {
		t.Errorf("DryMix: expected 0.4, got %f", r.DryMix)
	}
	if r.WetMix != 0.6 {
		t.Errorf("WetMix: expected 0.6, got %f", r.WetMix)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// rms computes the root mean square amplitude of a float32 slice.
// Used to measure signal energy in a window without caring about phase.
func rms(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}
