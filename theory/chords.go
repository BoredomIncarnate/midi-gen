package theory

import "fmt"

// ChordIntervals maps chord quality names to their semitone intervals from the root.
//
// Each slice represents the notes of the chord as offsets from the root in
// semitones. The root (0) is always the first element.
//
// Interval reference:
//
//	0  = root (unison)
//	2  = major 2nd
//	3  = minor 3rd
//	4  = major 3rd
//	5  = perfect 4th
//	6  = tritone (diminished 5th / augmented 4th)
//	7  = perfect 5th
//	8  = augmented 5th / minor 6th
//	9  = major 6th
//	10 = minor 7th (dominant 7th)
//	11 = major 7th
//	14 = major 9th  (2 + octave)
//	17 = perfect 11th (5 + octave)
//	21 = major 13th (9 + octave)
//
// Memory layout: map[string][]int
//
//	key:   chord quality name string
//	value: slice of semitone offsets from root, always beginning with 0
var ChordIntervals = map[string][]int{
	// --- Triads (3 notes) ---

	// Major triad: root + major 3rd + perfect 5th — bright, stable
	"major": {0, 4, 7},

	// Minor triad: root + minor 3rd + perfect 5th — dark, stable
	"minor": {0, 3, 7},

	// Diminished triad: root + minor 3rd + tritone — tense, unstable
	"dim": {0, 3, 6},

	// Augmented triad: root + major 3rd + augmented 5th — dreamlike, ambiguous
	"aug": {0, 4, 8},

	// Suspended 2nd: replaces the 3rd with a major 2nd — open, unresolved
	"sus2": {0, 2, 7},

	// Suspended 4th: replaces the 3rd with a perfect 4th — tense, wants to resolve
	"sus4": {0, 5, 7},

	// --- Seventh chords (4 notes) ---

	// Dominant 7th: major triad + minor 7th — bluesy, strong pull to resolve
	"dom7": {0, 4, 7, 10},

	// Major 7th: major triad + major 7th — lush, jazzy
	"maj7": {0, 4, 7, 11},

	// Minor 7th: minor triad + minor 7th — smooth, jazzy
	"min7": {0, 3, 7, 10},

	// Minor-major 7th: minor triad + major 7th — dark, cinematic tension
	"minmaj7": {0, 3, 7, 11},

	// Diminished 7th: fully diminished — symmetric, all minor 3rds, extremely tense
	"dim7": {0, 3, 6, 9},

	// Half-diminished (minor 7 flat 5): dim triad + minor 7th
	"halfdim7": {0, 3, 6, 10},

	// Augmented major 7th: aug triad + major 7th — very tense, chromatic
	"augmaj7": {0, 4, 8, 11},

	// Dominant 7th suspended 4th: sus4 + minor 7th — funky, unresolved
	"dom7sus4": {0, 5, 7, 10},

	// --- Extended chords (5+ notes) ---

	// Major 9th: maj7 + major 9th — full, orchestral
	"maj9": {0, 4, 7, 11, 14},

	// Dominant 9th: dom7 + major 9th — rich dominant, common in jazz/funk
	"dom9": {0, 4, 7, 10, 14},

	// Minor 9th: min7 + major 9th — lush minor color
	"min9": {0, 3, 7, 10, 14},

	// Dominant 11th: dom9 + perfect 11th — very dense, jazz voicing
	"dom11": {0, 4, 7, 10, 14, 17},

	// Major 13th: maj9 + major 13th — maximum color, jazz/film
	"maj13": {0, 4, 7, 11, 14, 21},

	// --- Added tone chords ---

	// Add9: major triad + major 9th (no 7th) — bright, pop sound
	"add9": {0, 4, 7, 14},

	// Minor add9: minor triad + major 9th (no 7th)
	"minadd9": {0, 3, 7, 14},

	// 6th chord: major triad + major 6th — sweet, retro
	"maj6": {0, 4, 7, 9},

	// Minor 6th: minor triad + major 6th — bittersweet
	"min6": {0, 3, 7, 9},
}

// BuildChord returns the MIDI note numbers for a chord voicing built upward
// from the given root note.
//
// Parameters:
//
//	root    — MIDI note number of the chord root (0–127)
//	quality — key into ChordIntervals (e.g. "major", "min7", "dom9")
//
// Each interval in ChordIntervals is added directly to root, so the chord is
// voiced in close position (all notes within roughly one octave of the root).
// For extended chords (9ths, 11ths, 13ths), notes may span up to ~1.75 octaves.
//
// Notes that would exceed MIDI's maximum (127) are silently omitted, matching
// the behaviour of ScaleNotes. This allows callers to build chords from high
// root notes without needing to check available range in advance.
//
// Example:
//
//	BuildChord(60, "maj7") → [60, 64, 67, 71]  (C E G B — Cmaj7)
//	BuildChord(69, "min7") → [69, 72, 76, 79]  (A C E G — Am7)
func BuildChord(root int, quality string) ([]int, error) {
	intervals, ok := ChordIntervals[quality]
	if !ok {
		return nil, fmt.Errorf("BuildChord: unknown chord quality %q", quality)
	}
	if root < 0 || root > 127 {
		return nil, fmt.Errorf("BuildChord: root %d outside valid MIDI range 0–127", root)
	}

	notes := make([]int, 0, len(intervals))
	for _, interval := range intervals {
		// Each interval is a semitone offset from the root.
		// e.g. root=60, interval=4 → note=64 (E4 above C4)
		note := root + interval
		if note > 127 {
			// Silently drop notes above the MIDI ceiling rather than erroring.
			// A chord voiced from a high root may simply have fewer available tones.
			continue
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// BuildChordInversion returns a chord voicing with the specified inversion applied.
//
// An inversion raises the lowest note(s) of a chord up by one octave, changing
// which note sits in the bass without changing the chord's identity.
//
//	inversion=0: root position    — root in bass    e.g. C E G
//	inversion=1: first inversion  — 3rd in bass     e.g. E G C
//	inversion=2: second inversion — 5th in bass     e.g. G C E
//	inversion=3: third inversion  — 7th in bass     e.g. B C E G  (7th chords only)
//
// Each inversion step takes the lowest note of the current voicing and raises
// it by 12 semitones (one octave), then re-appends it at the top.
// Notes exceeding 127 after the octave raise are silently dropped.
func BuildChordInversion(root int, quality string, inversion int) ([]int, error) {
	notes, err := BuildChord(root, quality)
	if err != nil {
		return nil, err
	}
	if inversion < 0 {
		return nil, fmt.Errorf("BuildChordInversion: inversion must be >= 0, got %d", inversion)
	}
	if inversion >= len(notes) {
		return nil, fmt.Errorf("BuildChordInversion: inversion %d out of range for %d-note chord", inversion, len(notes))
	}

	// Apply each inversion step: take the first (lowest) note, add an octave,
	// append it to the end, then remove it from the front.
	//
	// Example — Cmaj (C=60, E=64, G=67), first inversion:
	//   step 1: take 60, add 12 → 72, append → [64, 67, 72], drop front
	//   result: [64, 67, 72]  (E G C — first inversion)
	for i := 0; i < inversion; i++ {
		raised := notes[0] + 12 // raise lowest note by one octave
		notes = notes[1:]       // remove the original lowest note
		if raised <= 127 {
			notes = append(notes, raised)
		}
	}

	return notes, nil
}

// ChordQualities returns a list of all available chord quality names.
// Useful for CLI help text and random selection by the generator.
func ChordQualities() []string {
	qualities := make([]string, 0, len(ChordIntervals))
	for k := range ChordIntervals {
		qualities = append(qualities, k)
	}
	return qualities
}
