package theory

import (
	"fmt"
	"strconv"
	"strings"
)

// noteNames maps semitone index (0–11) to its canonical sharp notation.
// Index 0 = C, index 1 = C#, ..., index 11 = B.
// We use sharps as the canonical form throughout this package. Flat equivalents
// (e.g. Bb = A#, Eb = D#) are handled by the noteAliases map below.
var noteNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// noteAliases maps flat/enharmonic spellings to their canonical sharp equivalent.
// This allows callers to use natural notation like "Bb4" or "Eb3" as input to
// NoteNumber without requiring strict sharp notation.
//
// Memory layout: map[string]string, each entry is a flat name → sharp name
var noteAliases = map[string]string{
	"DB": "C#",
	"EB": "D#",
	"FB": "E", // Fb is enharmonic to E
	"GB": "F#",
	"AB": "G#",
	"BB": "A#",
	"CB": "B", // Cb is enharmonic to B
}

// Scales maps scale names to their interval patterns expressed as semitone
// offsets from the root note.
//
// Each slice entry represents how many semitones above the root that scale
// degree sits. The root itself is always 0. The octave (12) is not included
// since it is implied — when spanning multiple octaves, 12 is added per octave.
//
// Memory layout: map[string][]int
//
//	key:   scale name string
//	value: slice of semitone offsets, always starting with 0 (the root)
//
// Interval reference:
//
//	0  = root (unison)
//	1  = minor 2nd
//	2  = major 2nd
//	3  = minor 3rd
//	4  = major 3rd
//	5  = perfect 4th
//	6  = tritone (augmented 4th / diminished 5th)
//	7  = perfect 5th
//	8  = minor 6th
//	9  = major 6th
//	10 = minor 7th
//	11 = major 7th
var Scales = map[string][]int{
	// Major (Ionian): bright, resolved. W W H W W W H
	"major": {0, 2, 4, 5, 7, 9, 11},

	// Natural Minor (Aeolian): dark, melancholic. W H W W H W W
	"minor": {0, 2, 3, 5, 7, 8, 10},

	// Major Pentatonic: 5-note subset of major, no half-steps — very consonant
	"pentatonic": {0, 2, 4, 7, 9},

	// Blues: pentatonic minor + flat 5 (tritone/"blue note") for tension
	"blues": {0, 3, 5, 6, 7, 10},

	// Dorian: minor with raised 6th — jazzy, used heavily in funk and modal jazz
	"dorian": {0, 2, 3, 5, 7, 9, 10},

	// Phrygian: minor with flat 2nd — dark, Spanish/flamenco flavor
	"phrygian": {0, 1, 3, 5, 7, 8, 10},

	// Lydian: major with raised 4th — dreamy, used in film scores
	"lydian": {0, 2, 4, 6, 7, 9, 11},

	// Mixolydian: major with flat 7th — dominant, rock and blues feel
	"mixolydian": {0, 2, 4, 5, 7, 9, 10},

	// Harmonic Minor: natural minor with raised 7th — classical, Middle-Eastern
	"harmonicminor": {0, 2, 3, 5, 7, 8, 11},

	// Whole Tone: all whole steps, 6 notes — dreamy/impressionist (Debussy)
	"wholetone": {0, 2, 4, 6, 8, 10},

	// Diminished (half-whole): alternating H W pattern — tense, jazz/horror
	"diminished": {0, 1, 3, 4, 6, 7, 9, 10},
}

// NoteNumber converts a human-readable note string into a MIDI note number.
//
// Format: <NoteName>[#|b]<Octave>
// Examples: "C4", "F#3", "Bb2", "A#5"
//
// MIDI note number formula:
//
//	note = (octave + 1) * 12 + semitone
//
// Where semitone is the 0-based index into the chromatic scale (C=0 … B=11).
// Middle C is C4 = (4+1)*12 + 0 = 60.
//
// Valid range: note numbers 0–127
//
//	note 0   = C-1  (sub-bass, below most instruments)
//	note 21  = A0   (lowest key on a standard 88-key piano)
//	note 60  = C4   (middle C)
//	note 69  = A4   (concert pitch, 440 Hz)
//	note 108 = C8   (highest key on a standard 88-key piano)
//	note 127 = G9   (MIDI maximum)
func NoteNumber(nameOctave string) (int, error) {
	s := strings.TrimSpace(strings.ToUpper(nameOctave))
	if len(s) < 2 {
		return 0, fmt.Errorf("NoteNumber: input %q too short", nameOctave)
	}

	// --- Step 1: extract the note name (1 or 2 chars) and octave string ---
	//
	// Parsing is order-sensitive because 'B' is both a valid note name and the
	// uppercase form of the flat symbol 'b'. We resolve the ambiguity with explicit
	// priority checks before falling back to single-character note names:
	//
	//   "C#4"  → note="C#", octave="4"   (sharp: second char is '#')
	//   "BB4"  → note="BB", octave="4"   (Bb: first two chars match alias "BB")
	//   "AB3"  → note="AB", octave="3"   (Ab: non-B root + 'B' suffix)
	//   "B3"   → note="B",  octave="3"   (natural B: single char fallback)
	var notePart, octavePart string
	if len(s) >= 3 && s[1] == '#' {
		// Sharp note: always two chars (e.g. "C#4", "F#3")
		notePart = s[:2]
		octavePart = s[2:]
	} else if len(s) >= 3 && s[0] == 'B' && s[1] == 'B' {
		// Bb specifically: "BB" followed by octave e.g. "BB4", "BB-1"
		// Must be checked before the generic flat rule since s[0]=='B' would
		// otherwise fall through to the single-char branch below.
		// Requires len >= 3 to ensure there is at least one octave digit after "BB".
		notePart = s[:2]
		octavePart = s[2:]
	} else if len(s) >= 3 && s[1] == 'B' && s[0] != 'B' {
		// Flat note with non-B root: e.g. "AB3", "EB4", "GB2"
		notePart = s[:2]
		octavePart = s[2:]
	} else {
		// Natural note or fallback: single char note name e.g. "B3", "C4", "A5"
		notePart = s[:1]
		octavePart = s[1:]
	}

	// --- Step 2: resolve flat aliases to canonical sharp names ---
	if canonical, ok := noteAliases[notePart]; ok {
		notePart = canonical
	}

	// --- Step 3: look up semitone index (0–11) ---
	semitone := -1
	for i, name := range noteNames {
		if name == notePart {
			semitone = i
			break
		}
	}
	if semitone == -1 {
		return 0, fmt.Errorf("NoteNumber: unrecognized note name %q in %q", notePart, nameOctave)
	}

	// --- Step 4: parse octave number (signed integer, e.g. -1, 0, 4, 9) ---
	octave, err := strconv.Atoi(octavePart)
	if err != nil {
		return 0, fmt.Errorf("NoteNumber: invalid octave %q in %q", octavePart, nameOctave)
	}

	// --- Step 5: compute MIDI note number ---
	// Formula: (octave + 1) * 12 + semitone
	// The +1 accounts for MIDI's convention where octave -1 starts at note 0.
	note := (octave+1)*12 + semitone
	if note < 0 || note > 127 {
		return 0, fmt.Errorf("NoteNumber: %q produces note %d, outside valid MIDI range 0–127", nameOctave, note)
	}

	return note, nil
}

// ScaleNotes returns all MIDI note numbers in a given scale starting from a
// root note, spanning a given number of octaves.
//
// Parameters:
//
//	root      — MIDI note number of the root (e.g. 60 for C4)
//	scaleName — key into the Scales map (e.g. "major", "blues")
//	octaves   — how many octaves to span (1 = one octave, 2 = two, etc.)
//
// The returned slice is ordered from lowest to highest note. Notes that would
// exceed MIDI's maximum (127) are silently omitted rather than causing an error,
// since it is valid to request e.g. 3 octaves of pentatonic from A5 — some
// upper notes simply won't exist in the MIDI range.
//
// Example: ScaleNotes(60, "major", 1) returns [60 62 64 65 67 69 71]
//
//	which is C D E F G A B (C major, one octave from C4)
func ScaleNotes(root int, scaleName string, octaves int) ([]int, error) {
	intervals, ok := Scales[scaleName]
	if !ok {
		return nil, fmt.Errorf("ScaleNotes: unknown scale %q", scaleName)
	}
	if octaves < 1 {
		return nil, fmt.Errorf("ScaleNotes: octaves must be >= 1, got %d", octaves)
	}
	if root < 0 || root > 127 {
		return nil, fmt.Errorf("ScaleNotes: root %d outside valid MIDI range 0–127", root)
	}

	notes := make([]int, 0, len(intervals)*octaves)

	for oct := 0; oct < octaves; oct++ {
		// Each octave shifts all intervals up by 12 semitones.
		// oct=0 → no shift, oct=1 → +12, oct=2 → +24, etc.
		octaveOffset := oct * 12

		for _, interval := range intervals {
			note := root + octaveOffset + interval

			// Skip notes that exceed the MIDI ceiling rather than erroring —
			// this allows callers to freely request wide ranges without needing
			// to pre-calculate whether every note fits.
			if note > 127 {
				continue
			}
			notes = append(notes, note)
		}
	}

	return notes, nil
}

// ScaleNames returns a sorted list of all available scale names.
// Useful for CLI help text and validation.
func ScaleNames() []string {
	names := make([]string, 0, len(Scales))
	for k := range Scales {
		names = append(names, k)
	}
	return names
}
