package theory

import (
	"testing"
)

// -----------------------------------------------------------------------------
// NoteNumber tests
// -----------------------------------------------------------------------------

// TestNoteNumber_MiddleC verifies the most commonly referenced MIDI anchor point.
// Middle C = C4 = MIDI note 60. This is the universal reference used by all DAWs.
func TestNoteNumber_MiddleC(t *testing.T) {
	n, err := NoteNumber("C4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 60 {
		t.Errorf("C4: expected 60, got %d", n)
	}
}

// TestNoteNumber_ConcertPitch verifies A4 = 69, the standard concert tuning
// reference (440 Hz). This is the anchor for MIDINoteToFreq in the synth package.
func TestNoteNumber_ConcertPitch(t *testing.T) {
	n, err := NoteNumber("A4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 69 {
		t.Errorf("A4: expected 69, got %d", n)
	}
}

// TestNoteNumber_NaturalNotes verifies all 7 natural notes in octave 4.
// These form the reference row from which all other calculations derive.
//
//	C4=60, D4=62, E4=64, F4=65, G4=67, A4=69, B4=71
func TestNoteNumber_NaturalNotes(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"C4", 60},
		{"D4", 62},
		{"E4", 64},
		{"F4", 65},
		{"G4", 67},
		{"A4", 69},
		{"B4", 71},
	}
	for _, tt := range tests {
		n, err := NoteNumber(tt.input)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.input, err)
			continue
		}
		if n != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.input, tt.expected, n)
		}
	}
}

// TestNoteNumber_Sharps verifies all 5 sharp notes in octave 4.
//
//	C#4=61, D#4=63, F#4=66, G#4=68, A#4=70
func TestNoteNumber_Sharps(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"C#4", 61},
		{"D#4", 63},
		{"F#4", 66},
		{"G#4", 68},
		{"A#4", 70},
	}
	for _, tt := range tests {
		n, err := NoteNumber(tt.input)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.input, err)
			continue
		}
		if n != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.input, tt.expected, n)
		}
	}
}

// TestNoteNumber_Flats verifies that flat notation resolves to the same MIDI
// number as its enharmonic sharp equivalent.
//
//	Db4 = C#4 = 61
//	Eb4 = D#4 = 63
//	Gb4 = F#4 = 66
//	Ab4 = G#4 = 68
//	Bb4 = A#4 = 70
func TestNoteNumber_Flats(t *testing.T) {
	tests := []struct {
		flat     string
		sharp    string
		expected int
	}{
		{"Db4", "C#4", 61},
		{"Eb4", "D#4", 63},
		{"Gb4", "F#4", 66},
		{"Ab4", "G#4", 68},
		{"Bb4", "A#4", 70},
	}
	for _, tt := range tests {
		nFlat, err := NoteNumber(tt.flat)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.flat, err)
			continue
		}
		nSharp, _ := NoteNumber(tt.sharp)
		if nFlat != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.flat, tt.expected, nFlat)
		}
		if nFlat != nSharp {
			t.Errorf("%s and %s should be enharmonic (same number), got %d and %d",
				tt.flat, tt.sharp, nFlat, nSharp)
		}
	}
}

// TestNoteNumber_OctaveBoundaries verifies correct behaviour at the edges of
// the MIDI range and across octave crossings.
//
//	C-1 = 0    (MIDI minimum)
//	B-1 = 11   (last note before octave 0)
//	C0  = 12   (start of octave 0)
//	C8  = 108  (top of standard piano)
//	G9  = 127  (MIDI maximum)
func TestNoteNumber_OctaveBoundaries(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"C-1", 0},
		{"B-1", 11},
		{"C0", 12},
		{"C8", 108},
		{"G9", 127},
	}
	for _, tt := range tests {
		n, err := NoteNumber(tt.input)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.input, err)
			continue
		}
		if n != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.input, tt.expected, n)
		}
	}
}

// TestNoteNumber_OctaveIncrement verifies that moving up one octave always
// adds exactly 12 semitones, regardless of the note name.
func TestNoteNumber_OctaveIncrement(t *testing.T) {
	pairs := []struct{ low, high string }{
		{"C3", "C4"},
		{"F#2", "F#3"},
		{"Bb1", "Bb2"},
		{"A4", "A5"},
	}
	for _, p := range pairs {
		lo, err1 := NoteNumber(p.low)
		hi, err2 := NoteNumber(p.high)
		if err1 != nil || err2 != nil {
			t.Errorf("unexpected error for %s/%s: %v %v", p.low, p.high, err1, err2)
			continue
		}
		if hi-lo != 12 {
			t.Errorf("%s to %s: expected difference of 12, got %d", p.low, p.high, hi-lo)
		}
	}
}

// TestNoteNumber_CaseInsensitive verifies that input is case-insensitive,
// so "c4", "C4", and "c#4" all parse correctly.
func TestNoteNumber_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"c4", 60},
		{"C4", 60},
		{"c#4", 61},
		{"C#4", 61},
		{"bb3", 58},
		{"BB3", 58},
	}
	for _, tt := range tests {
		n, err := NoteNumber(tt.input)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.input, err)
			continue
		}
		if n != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.input, tt.expected, n)
		}
	}
}

// TestNoteNumber_OutOfRange verifies that notes outside 0–127 return an error
// rather than silently wrapping or producing invalid MIDI data.
func TestNoteNumber_OutOfRange(t *testing.T) {
	tests := []string{
		"C-2", // below 0
		"A#9", // above 127
		"G#9", // above 127
	}
	for _, input := range tests {
		_, err := NoteNumber(input)
		if err == nil {
			t.Errorf("%s: expected an error for out-of-range note, got none", input)
		}
	}
}

// TestNoteNumber_InvalidInput verifies that malformed input returns an error.
func TestNoteNumber_InvalidInput(t *testing.T) {
	tests := []string{
		"",   // empty
		"4",  // no note name
		"X4", // invalid note letter
		"C",  // missing octave
		"Cz", // non-numeric octave
	}
	for _, input := range tests {
		_, err := NoteNumber(input)
		if err == nil {
			t.Errorf("%q: expected an error for invalid input, got none", input)
		}
	}
}

// -----------------------------------------------------------------------------
// ScaleNotes tests
// -----------------------------------------------------------------------------

// TestScaleNotes_CMajorOneOctave verifies the canonical C major scale from C4.
// Expected: C D E F G A B = [60 62 64 65 67 69 71]
func TestScaleNotes_CMajorOneOctave(t *testing.T) {
	notes, err := ScaleNotes(60, "major", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{60, 62, 64, 65, 67, 69, 71}
	if !intsEqual(notes, expected) {
		t.Errorf("C major: expected %v, got %v", expected, notes)
	}
}

// TestScaleNotes_AMinorOneOctave verifies A natural minor from A3 (note 57).
// Expected: A B C D E F G = [57 59 60 62 64 65 67]
func TestScaleNotes_AMinorOneOctave(t *testing.T) {
	notes, err := ScaleNotes(57, "minor", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{57, 59, 60, 62, 64, 65, 67}
	if !intsEqual(notes, expected) {
		t.Errorf("A minor: expected %v, got %v", expected, notes)
	}
}

// TestScaleNotes_TwoOctaves verifies that requesting 2 octaves returns 2x the
// notes, with the second octave offset by exactly 12 semitones.
func TestScaleNotes_TwoOctaves(t *testing.T) {
	one, _ := ScaleNotes(60, "major", 1)
	two, err := ScaleNotes(60, "major", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have exactly twice as many notes
	if len(two) != len(one)*2 {
		t.Errorf("two octaves: expected %d notes, got %d", len(one)*2, len(two))
	}

	// Second octave should be first octave + 12
	for i, n := range one {
		if two[i+len(one)] != n+12 {
			t.Errorf("note %d: expected %d in second octave, got %d", i, n+12, two[i+len(one)])
		}
	}
}

// TestScaleNotes_PentatonicCount verifies the pentatonic scale has 5 notes per octave.
func TestScaleNotes_PentatonicCount(t *testing.T) {
	notes, err := ScaleNotes(60, "pentatonic", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 5 {
		t.Errorf("pentatonic: expected 5 notes, got %d", len(notes))
	}
}

// TestScaleNotes_MIDICeiling verifies that notes exceeding 127 are silently
// dropped rather than wrapping or erroring, since it is valid to request a
// range that partially exceeds the MIDI ceiling.
func TestScaleNotes_MIDICeiling(t *testing.T) {
	// G9 = 127 is the highest MIDI note. Requesting major scale from G8 (note 115)
	// across 2 octaves will produce some notes above 127 — these should be dropped.
	notes, err := ScaleNotes(115, "major", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range notes {
		if n > 127 {
			t.Errorf("note %d exceeds MIDI maximum of 127", n)
		}
	}
}

// TestScaleNotes_UnknownScale verifies that an unrecognised scale name returns
// an error with a useful message rather than silently returning empty results.
func TestScaleNotes_UnknownScale(t *testing.T) {
	_, err := ScaleNotes(60, "madeupscale", 1)
	if err == nil {
		t.Error("expected error for unknown scale, got none")
	}
}

// TestScaleNotes_InvalidOctaves verifies that octaves < 1 returns an error.
func TestScaleNotes_InvalidOctaves(t *testing.T) {
	_, err := ScaleNotes(60, "major", 0)
	if err == nil {
		t.Error("expected error for octaves=0, got none")
	}
}

// TestScaleNotes_AllScalesLoad verifies every entry in the Scales map can be
// used without error from a standard root (C4 = 60). This guards against typos
// or malformed interval slices being added to the map.
func TestScaleNotes_AllScalesLoad(t *testing.T) {
	for name := range Scales {
		notes, err := ScaleNotes(60, name, 1)
		if err != nil {
			t.Errorf("scale %q: unexpected error: %v", name, err)
			continue
		}
		if len(notes) == 0 {
			t.Errorf("scale %q: returned 0 notes", name)
		}
		// First note must always be the root
		if notes[0] != 60 {
			t.Errorf("scale %q: first note should be root 60, got %d", name, notes[0])
		}
	}
}

// TestScaleNotes_Ascending verifies that notes are always returned in
// ascending order. The generator depends on this to build melodies correctly.
func TestScaleNotes_Ascending(t *testing.T) {
	for name := range Scales {
		notes, _ := ScaleNotes(60, name, 2)
		for i := 1; i < len(notes); i++ {
			if notes[i] <= notes[i-1] {
				t.Errorf("scale %q: notes not ascending at index %d: %d then %d",
					name, i, notes[i-1], notes[i])
			}
		}
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
