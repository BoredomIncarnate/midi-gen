package theory

import (
	"testing"
)

// -----------------------------------------------------------------------------
// BuildChord tests
// -----------------------------------------------------------------------------

// TestBuildChord_CMajor verifies the most fundamental chord — C major triad from C4.
//
// C major = root + major 3rd + perfect 5th
// C4(60) + 4 semitones = E4(64)
// C4(60) + 7 semitones = G4(67)
// Expected: [60, 64, 67]
func TestBuildChord_CMajor(t *testing.T) {
	notes, err := BuildChord(60, "major")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{60, 64, 67}
	if !intsEqual(notes, expected) {
		t.Errorf("C major: expected %v, got %v", expected, notes)
	}
}

// TestBuildChord_AMinor verifies A minor triad from A3 (note 57).
//
// A minor = root + minor 3rd + perfect 5th
// A3(57) + 3 = C4(60)
// A3(57) + 7 = E4(64)
// Expected: [57, 60, 64]
func TestBuildChord_AMinor(t *testing.T) {
	notes, err := BuildChord(57, "minor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{57, 60, 64}
	if !intsEqual(notes, expected) {
		t.Errorf("A minor: expected %v, got %v", expected, notes)
	}
}

// TestBuildChord_Cmaj7 verifies C major 7th chord — a 4-note chord.
//
// Cmaj7 = root + major 3rd + perfect 5th + major 7th
// C4(60) + 4 = E4(64)
// C4(60) + 7 = G4(67)
// C4(60) + 11 = B4(71)
// Expected: [60, 64, 67, 71]
func TestBuildChord_Cmaj7(t *testing.T) {
	notes, err := BuildChord(60, "maj7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{60, 64, 67, 71}
	if !intsEqual(notes, expected) {
		t.Errorf("Cmaj7: expected %v, got %v", expected, notes)
	}
}

// TestBuildChord_Dom9 verifies a 5-note extended chord (dominant 9th from G3).
//
// G3 = note 55
// G dom9 = root + maj3 + 5th + min7 + maj9
// 55 + 4=59, 55+7=62, 55+10=65, 55+14=69
// Expected: [55, 59, 62, 65, 69]
func TestBuildChord_Dom9(t *testing.T) {
	notes, err := BuildChord(55, "dom9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{55, 59, 62, 65, 69}
	if !intsEqual(notes, expected) {
		t.Errorf("G dom9: expected %v, got %v", expected, notes)
	}
}

// TestBuildChord_RootIsFirstNote verifies that for every chord quality, the
// first note in the returned slice is always the root. This is a structural
// invariant the generator depends on when identifying chord roots.
func TestBuildChord_RootIsFirstNote(t *testing.T) {
	root := 60
	for quality := range ChordIntervals {
		notes, err := BuildChord(root, quality)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", quality, err)
			continue
		}
		if len(notes) == 0 {
			t.Errorf("%s: returned empty slice", quality)
			continue
		}
		if notes[0] != root {
			t.Errorf("%s: first note should be root %d, got %d", quality, root, notes[0])
		}
	}
}

// TestBuildChord_NotesAscending verifies that all returned notes are in
// strictly ascending order. Close-position voicing (intervals always positive
// and increasing) guarantees this, but we verify it explicitly since the
// generator and scheduler both assume ascending order.
func TestBuildChord_NotesAscending(t *testing.T) {
	for quality := range ChordIntervals {
		notes, err := BuildChord(60, quality)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", quality, err)
			continue
		}
		for i := 1; i < len(notes); i++ {
			if notes[i] <= notes[i-1] {
				t.Errorf("%s: notes not ascending at index %d: %d then %d",
					quality, i, notes[i-1], notes[i])
			}
		}
	}
}

// TestBuildChord_AllQualitiesLoad verifies every entry in ChordIntervals can
// be built from a standard root without error. Guards against typos or invalid
// interval slices being silently present in the map.
func TestBuildChord_AllQualitiesLoad(t *testing.T) {
	for quality := range ChordIntervals {
		notes, err := BuildChord(60, quality)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", quality, err)
			continue
		}
		if len(notes) < 2 {
			t.Errorf("%s: expected at least 2 notes, got %d", quality, len(notes))
		}
	}
}

// TestBuildChord_MIDICeiling verifies that notes above 127 are silently
// dropped rather than included or causing an error.
// A maj13 chord from note 120 would extend to 120+21=141, which is above 127.
func TestBuildChord_MIDICeiling(t *testing.T) {
	notes, err := BuildChord(120, "maj13")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range notes {
		if n > 127 {
			t.Errorf("note %d exceeds MIDI maximum 127", n)
		}
	}
}

// TestBuildChord_UnknownQuality verifies that an unrecognised quality returns
// a descriptive error rather than silently returning empty results.
func TestBuildChord_UnknownQuality(t *testing.T) {
	_, err := BuildChord(60, "superawesomechord")
	if err == nil {
		t.Error("expected error for unknown quality, got none")
	}
}

// TestBuildChord_InvalidRoot verifies that roots outside 0–127 return an error.
func TestBuildChord_InvalidRoot(t *testing.T) {
	tests := []int{-1, 128, 255}
	for _, root := range tests {
		_, err := BuildChord(root, "major")
		if err == nil {
			t.Errorf("root %d: expected error for out-of-range root, got none", root)
		}
	}
}

// -----------------------------------------------------------------------------
// BuildChordInversion tests
// -----------------------------------------------------------------------------

// TestBuildChordInversion_RootPosition verifies that inversion=0 returns the
// same notes as BuildChord (root position, no change).
func TestBuildChordInversion_RootPosition(t *testing.T) {
	base, _ := BuildChord(60, "major")
	inv, err := BuildChordInversion(60, "major", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !intsEqual(base, inv) {
		t.Errorf("inversion=0 should match root position: base=%v, got=%v", base, inv)
	}
}

// TestBuildChordInversion_FirstInversion verifies C major first inversion.
//
// Root position: [60(C), 64(E), 67(G)]
// First inversion: raise C(60) by octave → 72
// Result: [64(E), 67(G), 72(C)] — E is now in the bass
func TestBuildChordInversion_FirstInversion(t *testing.T) {
	notes, err := BuildChordInversion(60, "major", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{64, 67, 72}
	if !intsEqual(notes, expected) {
		t.Errorf("C major 1st inversion: expected %v, got %v", expected, notes)
	}
}

// TestBuildChordInversion_SecondInversion verifies C major second inversion.
//
// First inversion: [64(E), 67(G), 72(C)]
// Second inversion: raise E(64) by octave → 76
// Result: [67(G), 72(C), 76(E)] — G is now in the bass
func TestBuildChordInversion_SecondInversion(t *testing.T) {
	notes, err := BuildChordInversion(60, "major", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{67, 72, 76}
	if !intsEqual(notes, expected) {
		t.Errorf("C major 2nd inversion: expected %v, got %v", expected, notes)
	}
}

// TestBuildChordInversion_ThirdInversion verifies a 7th chord third inversion.
//
// Cmaj7 root position: [60(C), 64(E), 67(G), 71(B)]
// 1st inv: raise C(60)→72:  [64, 67, 71, 72]
// 2nd inv: raise E(64)→76:  [67, 71, 72, 76]
// 3rd inv: raise G(67)→79:  [71, 72, 76, 79]  — B is now in the bass
func TestBuildChordInversion_ThirdInversion(t *testing.T) {
	notes, err := BuildChordInversion(60, "maj7", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{71, 72, 76, 79}
	if !intsEqual(notes, expected) {
		t.Errorf("Cmaj7 3rd inversion: expected %v, got %v", expected, notes)
	}
}

// TestBuildChordInversion_OutOfRange verifies that requesting an inversion
// index >= the number of notes returns an error.
// A triad has 3 notes, so inversion=3 is out of range (valid: 0, 1, 2).
func TestBuildChordInversion_OutOfRange(t *testing.T) {
	_, err := BuildChordInversion(60, "major", 3)
	if err == nil {
		t.Error("expected error for inversion >= chord length, got none")
	}
}

// TestBuildChordInversion_NegativeInversion verifies that a negative inversion
// value returns an error rather than panicking on a negative slice index.
func TestBuildChordInversion_NegativeInversion(t *testing.T) {
	_, err := BuildChordInversion(60, "major", -1)
	if err == nil {
		t.Error("expected error for negative inversion, got none")
	}
}

// TestBuildChordInversion_SameNoteCount verifies that inverting a chord
// preserves the note count (assuming no notes are dropped at the MIDI ceiling).
// From a low root the ceiling should not be an issue.
func TestBuildChordInversion_SameNoteCount(t *testing.T) {
	base, _ := BuildChord(48, "maj7") // C3 — safely below ceiling even after inversions
	for inv := 0; inv < len(base); inv++ {
		notes, err := BuildChordInversion(48, "maj7", inv)
		if err != nil {
			t.Errorf("inversion %d: unexpected error: %v", inv, err)
			continue
		}
		if len(notes) != len(base) {
			t.Errorf("inversion %d: expected %d notes, got %d", inv, len(base), len(notes))
		}
	}
}
