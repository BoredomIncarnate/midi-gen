package theory

import "midi-gen/midi"

// GeneratorConfig holds all parameters for music generation.
type GeneratorConfig struct {
	Scale      string // e.g. "major", "minor"
	RootNote   int    // MIDI note number, e.g. 60 for C4
	Octaves    int    // how many octaves to span
	Length     int    // number of notes or chords
	MinVel     int    // minimum MIDI velocity (0–127)
	MaxVel     int    // maximum MIDI velocity (0–127)
	Complexity string // "simple" | "medium" | "complex"
	Mode       string // "melody" | "chords" | "progression"
	BPM        int
	Quantize   string // "quarter" | "eighth" | "sixteenth"
	Seed       int64  // 0 = use random seed
}

// Generate produces a MIDI track from the given config.
func Generate(cfg GeneratorConfig) (midi.Track, error) {
	// TODO:
	// 1. Seed RNG (use cfg.Seed if non-zero)
	// 2. Build note pool from scale + octaves
	// 3. Depending on cfg.Mode, generate note-on/off events
	// 4. Apply complexity (velocity range, rhythmic variation, chord extensions)
	return midi.Track{}, nil
}
