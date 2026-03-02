package synth

// Reverb implements a Schroeder reverb model consisting of 4 parallel comb
// filters feeding into 2 series allpass filters.
//
// The Schroeder model (1962) is the foundation of most digital reverb
// algorithms. It works by simulating the dense echo pattern of a physical
// room through two filter types:
//
//	Comb filter:   recirculates a delayed copy of the signal with decay.
//	               Creates the sense of distance and room size.
//	               Four parallel combs produce a dense, diffuse reflection.
//
//	Allpass filter: passes all frequencies at equal amplitude but shifts
//	               phase. Diffuses the echo pattern without coloring the sound.
//	               Two series allpasses smooth the comb filter artifacts.
//
// Signal flow:
//
//	              ┌─[Comb 1 (delay=1557)]─┐
//	              ├─[Comb 2 (delay=1617)]─┤
//	input ────────┤                        ├──> sum ──> [Allpass 1] ──> [Allpass 2] ──> wet
//	              ├─[Comb 3 (delay=1491)]─┤
//	              └─[Comb 4 (delay=1422)]─┘
//
//	output = input * DryMix + wet * WetMix
//
// Delay line lengths are prime-number multiples chosen to avoid resonant
// beating between the comb filters. The values below are tuned for 44100 Hz
// and produce a natural-sounding room decay of roughly 1–2 seconds.
type Reverb struct {
	// combs are the 4 parallel feedback comb filters.
	// Each comb filter has its own delay line and feedback coefficient.
	combs [4]combFilter

	// allpasses are the 2 series allpass filters applied after comb mixing.
	allpasses [2]allpassFilter

	// DryMix is the gain applied to the unprocessed (direct) signal.
	// Range: 0.0–1.0. Default: 0.7
	DryMix float32

	// WetMix is the gain applied to the reverb (processed) signal.
	// Range: 0.0–1.0. Default: 0.3
	// Higher values = more reverb, more distant/washed sound.
	WetMix float32
}

// combFilter is a feedback comb filter with a fixed-length delay line.
//
// A comb filter delays the input signal by `delay` samples and adds a
// decayed copy of its own output back into the delay line. This creates
// an exponentially decaying series of echoes spaced `delay` samples apart.
//
// Transfer function (z-domain):
//
//	H(z) = z^(-delay) / (1 - feedback * z^(-delay))
//
// The name "comb" comes from the frequency response, which has peaks and
// nulls spaced evenly like the teeth of a comb.
//
// Memory layout:
//
//	buf      []float32   circular delay line, length = delay samples
//	         Each element holds one sample of audio history.
//	         Index wraps via writePos % len(buf).
//	writePos int         current write position in the circular buffer.
//	                     Advances by 1 each sample, wraps at len(buf).
//	feedback float32     recirculation gain [0.0, 1.0).
//	                     Values closer to 1.0 = longer decay tail.
//	                     Values >= 1.0 = unstable (infinite feedback).
type combFilter struct {
	buf      []float32
	writePos int
	feedback float32
}

// newCombFilter allocates a comb filter with the given delay length (samples)
// and feedback coefficient.
func newCombFilter(delaySamples int, feedback float32) combFilter {
	return combFilter{
		buf:      make([]float32, delaySamples),
		feedback: feedback,
	}
}

// process runs one sample through the comb filter and returns the output.
//
// Operation per sample:
//  1. Read the delayed sample from the circular buffer at writePos.
//     This is the echo from `delaySamples` ago.
//  2. Compute the new buffer value: input + delayed * feedback.
//     This mixes the current input with the decayed echo.
//  3. Write the new value back into the buffer at writePos.
//  4. Advance writePos, wrapping at buffer length.
//  5. Return the delayed sample (the echo, before feedback mixing).
//
// The output is the pure delayed signal rather than the feedback-mixed value.
// This preserves the original signal's character in the output while the
// feedback mixing happens internally for the next cycle.
func (c *combFilter) process(input float32) float32 {
	// Step 1: read the oldest sample from the delay line
	// writePos points to the position that is exactly `len(buf)` samples old
	delayed := c.buf[c.writePos]

	// Step 2: compute new buffer value — mix input with fed-back echo
	// feedback < 1.0 ensures the echo decays geometrically over time
	c.buf[c.writePos] = input + delayed*c.feedback

	// Step 3: advance write position with wrap
	c.writePos++
	if c.writePos >= len(c.buf) {
		c.writePos = 0
	}

	// Step 4: return the delayed output
	return delayed
}

// allpassFilter is a first-order allpass filter used to diffuse the output
// of the comb filter bank.
//
// An allpass filter passes all frequencies at the same amplitude but
// introduces frequency-dependent phase shifts. In reverb, allpass filters
// are used to smear echoes in time, creating a more diffuse, natural-sounding
// decay without coloring the frequency spectrum.
//
// Transfer function (z-domain):
//
//	H(z) = (-gain + z^(-delay)) / (1 - gain * z^(-delay))
//
// Memory layout:
//
//	buf      []float32   circular delay line, length = delay samples
//	writePos int         current write position, wraps at len(buf)
//	gain     float32     allpass coefficient, typically 0.5
//	                     Controls the balance between direct and delayed signal.
type allpassFilter struct {
	buf      []float32
	writePos int
	gain     float32
}

// newAllpassFilter allocates an allpass filter with the given delay and gain.
func newAllpassFilter(delaySamples int, gain float32) allpassFilter {
	return allpassFilter{
		buf:  make([]float32, delaySamples),
		gain: gain,
	}
}

// process runs one sample through the allpass filter and returns the output.
//
// Operation per sample (Schroeder allpass formulation):
//  1. Read delayed sample from the circular buffer.
//  2. Compute v = input - gain * delayed   (subtract scaled echo from input)
//  3. Write v into the buffer at writePos.
//  4. Advance writePos with wrap.
//  5. Return: delayed + gain * v           (add scaled v to the echo)
//
// This formulation is equivalent to the standard allpass transfer function
// but avoids multiplications by keeping intermediate values explicit.
func (a *allpassFilter) process(input float32) float32 {
	delayed := a.buf[a.writePos]

	// v is the value stored in the delay line — a blend of input and echo
	v := input - a.gain*delayed
	a.buf[a.writePos] = v

	a.writePos++
	if a.writePos >= len(a.buf) {
		a.writePos = 0
	}

	// Output blends the stored echo with scaled v
	return delayed + a.gain*v
}

// NewReverb creates a Reverb with delay line lengths and feedback tuned for
// 44100 Hz sample rate.
//
// Comb filter delay lengths (in samples at 44100 Hz):
//
//	1557, 1617, 1491, 1422
//
// These are mutually prime numbers chosen to avoid periodic coincidences
// between the four delay lines that would create audible resonant beating.
// They correspond to delays of roughly 32–37ms, producing a room decay
// characteristic of a medium-sized space.
//
// Comb feedback = 0.84: decay time ≈ 1.5 seconds at 44100 Hz.
// A higher value (e.g. 0.90) gives a longer tail; lower (e.g. 0.75) shorter.
//
// Allpass delay lengths: 225, 556 (also chosen to be mutually prime).
// Allpass gain = 0.5 (standard Schroeder value).
func NewReverb() *Reverb {
	return &Reverb{
		combs: [4]combFilter{
			newCombFilter(1557, 0.84),
			newCombFilter(1617, 0.84),
			newCombFilter(1491, 0.84),
			newCombFilter(1422, 0.84),
		},
		allpasses: [2]allpassFilter{
			newAllpassFilter(225, 0.5),
			newAllpassFilter(556, 0.5),
		},
		DryMix: 0.7,
		WetMix: 0.3,
	}
}

// NewReverbWithMix creates a Reverb with custom dry/wet mix ratios.
//
// Parameters:
//
//	dryMix — gain for the direct unprocessed signal (0.0–1.0)
//	wetMix — gain for the reverb signal (0.0–1.0)
//
// A dryMix of 1.0 and wetMix of 0.0 produces no reverb effect.
// A dryMix of 0.0 and wetMix of 1.0 produces full wet reverb with no dry signal.
// Typical values: dryMix=0.6–0.8, wetMix=0.2–0.4.
func NewReverbWithMix(dryMix, wetMix float32) *Reverb {
	r := NewReverb()
	r.DryMix = dryMix
	r.WetMix = wetMix
	return r
}

// Process applies reverb to an entire PCM buffer in-place.
//
// The buffer is modified directly — each sample is replaced with:
//
//	output[i] = input[i] * DryMix + wet[i] * WetMix
//
// where wet[i] is the output of the allpass chain for that sample.
//
// Processing is per-sample (not block-based) to keep the implementation
// simple and avoid additional buffer allocations. For a 44100 Hz buffer
// of 1 second, this is 44100 iterations of a small arithmetic loop —
// fast enough for real-time use on any modern CPU.
//
// The Reverb struct is stateful — delay lines persist between Process calls.
// This means the reverb tail from one buffer continues naturally into the
// next, which is correct behaviour for real-time streaming. For offline
// rendering, call Process once on the full buffer.
func (r *Reverb) Process(buf []float32) {
	for i, sample := range buf {
		// --- Comb filter bank (4 parallel) ---
		// Each comb filter processes the same input sample independently.
		// Their outputs are summed and scaled to prevent amplitude buildup.
		combSum := float32(0)
		for c := range r.combs {
			combSum += r.combs[c].process(sample)
		}
		// Scale by 1/4 to compensate for summing 4 parallel outputs.
		// Without scaling, 4 combs at equal amplitude would cause clipping.
		combSum *= 0.25

		// --- Allpass chain (2 series) ---
		// The combined comb output passes through both allpass filters in series.
		// Each allpass diffuses the echo pattern further in time.
		wet := r.allpasses[0].process(combSum)
		wet = r.allpasses[1].process(wet)

		// --- Mix dry and wet signals ---
		buf[i] = sample*r.DryMix + wet*r.WetMix
	}
}
