package synth

import "math"

// SampleRate is the number of audio samples per second.
// 44100 Hz is the standard CD-quality sample rate and is universally
// supported by macOS CoreAudio, which is the backend oto uses on macOS.
//
// At 44100 Hz:
//   - One sample = ~22.7 microseconds
//   - One millisecond = 44.1 samples
//   - One second = 44100 samples
const SampleRate = 44100

// ADSRConfig defines the shape of the volume envelope applied to each note.
//
// ADSR stands for Attack, Decay, Sustain, Release — the four phases of how
// a note's amplitude evolves over time:
//
//	        peak (1.0)
//	         /\
//	        /  \_______ sustain level
//	       /    decay  \
//	      / attack      \ release
//	     /               \
//	────/─────────────────\────  0.0
//	   on                 off
//
// Without an envelope, notes start and stop instantaneously, causing
// audible clicks (Gibbs phenomenon — a square wave in amplitude).
// Even a 2ms attack/release ramp eliminates most clicking.
//
// Memory layout:
//
//	AttackMs   float64   milliseconds for amplitude to ramp 0.0 → 1.0
//	DecayMs    float64   milliseconds to fall from 1.0 → SustainLevel
//	SustainLevel float64  amplitude held during the body of the note (0.0–1.0)
//	ReleaseMs  float64   milliseconds to fall from SustainLevel → 0.0
type ADSRConfig struct {
	AttackMs     float64
	DecayMs      float64
	SustainLevel float64
	ReleaseMs    float64
}

// DefaultADSR returns a general-purpose ADSR config suitable for melodic
// playback. Attack and release are short enough to prevent clicks while
// still feeling immediate. Decay and sustain give notes a natural body.
func DefaultADSR() ADSRConfig {
	return ADSRConfig{
		AttackMs:     3.0,
		DecayMs:      15.0,
		SustainLevel: 0.7,
		ReleaseMs:    50.0,
	}
}

// ADSRForComplexity returns an ADSR config tuned to the generation complexity.
// More complex settings use shorter attack/decay for a punchier, more
// articulate sound that suits busier rhythms.
//
//	simple:  slower attack/release — smooth, legato
//	medium:  balanced — natural playing feel
//	complex: faster attack/release — punchy, staccato-friendly
func ADSRForComplexity(complexity string) ADSRConfig {
	switch complexity {
	case "simple":
		return ADSRConfig{AttackMs: 5.0, DecayMs: 20.0, SustainLevel: 0.8, ReleaseMs: 30.0}
	case "complex":
		return ADSRConfig{AttackMs: 2.0, DecayMs: 10.0, SustainLevel: 0.6, ReleaseMs: 80.0}
	default: // "medium"
		return ADSRConfig{AttackMs: 3.0, DecayMs: 15.0, SustainLevel: 0.7, ReleaseMs: 50.0}
	}
}

// MIDINoteToFreq converts a MIDI note number to its fundamental frequency in Hz.
//
// Formula: f = 440 * 2^((note - 69) / 12)
//
// This is derived from equal temperament tuning where:
//   - A4 (note 69) = 440 Hz (concert pitch)
//   - Each semitone is a frequency ratio of 2^(1/12) ≈ 1.05946
//   - Each octave doubles the frequency (12 semitones = 2^1 = 2x)
//
// Reference points:
//
//	note 21  = A0  =   27.50 Hz  (lowest piano key)
//	note 60  = C4  =  261.63 Hz  (middle C)
//	note 69  = A4  =  440.00 Hz  (concert A)
//	note 108 = C8  = 4186.01 Hz  (highest piano key)
func MIDINoteToFreq(note int) float64 {
	return 440.0 * math.Pow(2.0, float64(note-69)/12.0)
}

// Voice represents a single sounding note — one oscillator with its own
// phase accumulator and ADSR envelope state.
//
// Multiple Voices are mixed together by the Scheduler to produce polyphony.
// Each Voice is fully independent: different pitch, velocity, envelope state.
//
// Memory layout:
//
//	Key         byte     MIDI note number (0–127), used to match Note Off events
//	Freq        float64  fundamental frequency in Hz, derived from Key
//	Phase       float64  current sine phase in radians [0, 2π)
//	            Advances by (2π * Freq / SampleRate) each sample.
//	            Carried across RenderSamples calls to prevent phase discontinuity.
//	Velocity    float64  note velocity normalised to [0.0, 1.0]
//	            Scales the overall amplitude of the voice.
//	ADSR        ADSRConfig  envelope shape for this voice
//	EnvPhase    envPhase    current envelope stage (attack/decay/sustain/release/done)
//	EnvSamples  int         samples elapsed within the current envelope stage
//	Releasing   bool        true once a Note Off has been received for this voice
//	            When true, the voice transitions to the release stage regardless
//	            of where it is in attack/decay/sustain.
type Voice struct {
	Key        byte
	Freq       float64
	Phase      float64
	Velocity   float64
	ADSR       ADSRConfig
	EnvPhase   envPhase
	EnvSamples int
	Releasing  bool
}

// envPhase represents which stage of the ADSR envelope a Voice is currently in.
// Stages are traversed in order: attack → decay → sustain → release → done.
// "done" means the envelope has fully completed and the Voice can be discarded.
type envPhase int

const (
	envAttack  envPhase = iota // amplitude ramping 0.0 → 1.0
	envDecay                   // amplitude falling 1.0 → SustainLevel
	envSustain                 // amplitude held at SustainLevel
	envRelease                 // amplitude falling SustainLevel → 0.0
	envDone                    // amplitude = 0.0, voice can be freed
)

// NewVoice creates a Voice for the given MIDI note and velocity.
//
// Parameters:
//
//	key      — MIDI note number (0–127)
//	velocity — MIDI velocity (0–127), normalised to 0.0–1.0 internally
//	adsr     — envelope configuration
func NewVoice(key byte, velocity byte, adsr ADSRConfig) *Voice {
	return &Voice{
		Key:      key,
		Freq:     MIDINoteToFreq(int(key)),
		Phase:    0.0,
		Velocity: float64(velocity) / 127.0, // normalise to [0.0, 1.0]
		ADSR:     adsr,
		EnvPhase: envAttack,
	}
}

// Release signals that a Note Off has been received for this voice.
// The voice will transition to the release stage on the next RenderSamples call,
// regardless of which envelope stage it is currently in.
//
// This is safe to call multiple times — subsequent calls are no-ops.
func (v *Voice) Release() {
	if v.EnvPhase != envDone {
		v.Releasing = true
	}
}

// IsDone reports whether the voice has completed its release stage and
// its amplitude has reached zero. Done voices should be removed from the
// active voice list by the Scheduler to free memory.
func (v *Voice) IsDone() bool {
	return v.EnvPhase == envDone
}

// envelope computes the current envelope amplitude multiplier [0.0, 1.0]
// and advances the envelope state machine by one sample.
//
// The envelope is computed as a linear ramp within each stage:
//
//	Attack:  progress = EnvSamples / attackSamples     → amplitude = progress
//	Decay:   progress = EnvSamples / decaySamples      → amplitude = 1.0 - progress*(1.0-SustainLevel)
//	Sustain: amplitude = SustainLevel (held constant)
//	Release: progress = EnvSamples / releaseSamples    → amplitude = SustainLevel * (1.0 - progress)
//	Done:    amplitude = 0.0
//
// EnvSamples increments each call and resets to 0 on stage transitions.
// Stage transitions trigger immediately when EnvSamples reaches the stage
// duration (in samples), computed from milliseconds via SampleRate.
func (v *Voice) envelope() float64 {
	// ms → samples conversion: samples = ms * SampleRate / 1000
	attackSamples := int(v.ADSR.AttackMs * SampleRate / 1000.0)
	decaySamples := int(v.ADSR.DecayMs * SampleRate / 1000.0)
	releaseSamples := int(v.ADSR.ReleaseMs * SampleRate / 1000.0)

	// Guard against zero-length stages (e.g. AttackMs=0)
	if attackSamples < 1 {
		attackSamples = 1
	}
	if decaySamples < 1 {
		decaySamples = 1
	}
	if releaseSamples < 1 {
		releaseSamples = 1
	}

	// If a Note Off was received, jump to release from wherever we are
	if v.Releasing && v.EnvPhase != envRelease && v.EnvPhase != envDone {
		v.EnvPhase = envRelease
		v.EnvSamples = 0
	}

	var amp float64

	switch v.EnvPhase {

	case envAttack:
		// Linear ramp: 0.0 at sample 0, 1.0 at sample attackSamples
		amp = float64(v.EnvSamples) / float64(attackSamples)
		v.EnvSamples++
		if v.EnvSamples >= attackSamples {
			v.EnvPhase = envDecay
			v.EnvSamples = 0
		}

	case envDecay:
		// Linear fall: 1.0 at sample 0, SustainLevel at sample decaySamples
		progress := float64(v.EnvSamples) / float64(decaySamples)
		amp = 1.0 - progress*(1.0-v.ADSR.SustainLevel)
		v.EnvSamples++
		if v.EnvSamples >= decaySamples {
			v.EnvPhase = envSustain
			v.EnvSamples = 0
		}

	case envSustain:
		// Constant: held at SustainLevel until Release() is called
		amp = v.ADSR.SustainLevel
		v.EnvSamples++

	case envRelease:
		// Linear fall: SustainLevel at sample 0, 0.0 at sample releaseSamples
		progress := float64(v.EnvSamples) / float64(releaseSamples)
		amp = v.ADSR.SustainLevel * (1.0 - progress)
		v.EnvSamples++
		if v.EnvSamples >= releaseSamples {
			v.EnvPhase = envDone
			amp = 0.0
		}

	case envDone:
		amp = 0.0
	}

	return amp
}

// RenderSamples generates n samples from this voice into the provided buffer,
// adding to any existing content (mixing, not overwriting).
//
// Adding to the buffer (+=) rather than overwriting (=) allows multiple voices
// to be mixed together by calling RenderSamples on each voice with the same
// buffer. The Scheduler zeroes the buffer before mixing all active voices.
//
// Per-sample computation:
//
//  1. Compute envelope amplitude for this sample (ADSR state machine)
//  2. Compute sine value at current phase: sin(phase)
//  3. Scale by envelope * velocity * masterGain
//  4. Add to buffer[i]
//  5. Advance phase: phase += 2π * freq / SampleRate
//  6. Wrap phase to [0, 2π) to prevent float64 precision loss over time
//
// The phase wrap (step 6) is critical for long notes — without it, phase
// accumulates into large float64 values where sin() loses precision.
//
// Parameters:
//
//	buf        — float32 slice to mix into, length must be >= n
//	n          — number of samples to render
//	masterGain — global volume scale applied after velocity and envelope
//	             Typical value: 0.3 (prevents clipping when voices are summed)
func (v *Voice) RenderSamples(buf []float32, n int, masterGain float64) {
	// Phase increment per sample: how much the sine wave advances each step.
	// At 440 Hz and 44100 Hz sample rate: phaseInc = 2π * 440 / 44100 ≈ 0.0627
	phaseInc := 2.0 * math.Pi * v.Freq / SampleRate

	for i := 0; i < n; i++ {
		// Envelope amplitude for this sample (advances internal state)
		envAmp := v.envelope()

		// Sine oscillator: value in [-1.0, 1.0]
		sample := math.Sin(v.Phase)

		// Scale: velocity controls perceived loudness, envelope shapes it over time
		// masterGain prevents clipping when multiple voices are summed
		buf[i] += float32(sample * envAmp * v.Velocity * masterGain)

		// Advance and wrap phase
		// Wrapping to [0, 2π) keeps phase in a numerically stable range
		v.Phase += phaseInc
		if v.Phase >= 2.0*math.Pi {
			v.Phase -= 2.0 * math.Pi
		}
	}
}
