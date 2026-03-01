package synth

import "math"

const SampleRate = 44100

// MIDINoteToFreq converts a MIDI note number to its frequency in Hz.
// A4 (note 69) = 440 Hz.
func MIDINoteToFreq(note int) float64 {
	return 440.0 * math.Pow(2, float64(note-69)/12.0)
}

// GenerateSine produces numSamples of a sine wave at the given frequency and amplitude.
// Phase is passed in and returned so callers can chain calls without discontinuities.
func GenerateSine(freq, amplitude float64, phase float64, numSamples int) (samples []float32, nextPhase float64) {
	// TODO: fill samples with sine wave, advance phase each sample by (2π * freq / SampleRate)
	samples = make([]float32, numSamples)
	_ = math.Sin
	return samples, phase
}
