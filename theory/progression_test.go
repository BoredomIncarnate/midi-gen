package theory

import (
	"testing"
)

// defaultProgConfig returns a consistent base config for progression parsing tests.
// C major from C4 is the simplest case to reason about — all white keys, all
// diatonic qualities well-defined.
func defaultProgConfig() (rootNote int, scaleName string, complexity string) {
	return 60, "major", "simple" // C4, major, simple
}

// -----------------------------------------------------------------------------
// ParseProgression — empty and nil cases
// -----------------------------------------------------------------------------

// TestParseProgression_EmptyStringReturnsNil verifies that an empty -prog
// string returns (nil, nil) — no error, no progression. This is the signal
// to the generator to use random behaviour.
func TestParseProgression_EmptyStringReturnsNil(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("", root, scale, complexity)
	if err != nil {
		t.Errorf("expected nil error for empty string, got %v", err)
	}
	if chords != nil {
		t.Errorf("expected nil chords for empty string, got %v", chords)
	}
}

// TestParseProgression_WhitespaceOnlyReturnsNil verifies that a string of only
// spaces is treated the same as an empty string.
func TestParseProgression_WhitespaceOnlyReturnsNil(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("   ", root, scale, complexity)
	if err != nil {
		t.Errorf("expected nil error for whitespace-only string, got %v", err)
	}
	if chords != nil {
		t.Errorf("expected nil chords for whitespace-only string, got %v", chords)
	}
}

// -----------------------------------------------------------------------------
// ParseProgression — scale degree format
// -----------------------------------------------------------------------------

// TestParseProgression_DegreesLength verifies that "1 4 5 1" produces exactly
// 4 ProgChords.
func TestParseProgression_DegreesLength(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("1 4 5 1", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chords) != 4 {
		t.Errorf("expected 4 chords, got %d", len(chords))
	}
}

// TestParseProgression_DegreeRoots verifies that scale degrees map to the
// correct MIDI root notes in C major from C4.
//
// C major scale from C4: C4=60, D4=62, E4=64, F4=65, G4=67, A4=69, B4=71
// Degree 1=60, 4=65, 5=67
func TestParseProgression_DegreeRoots(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("1 4 5", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRoots := []int{60, 65, 67} // C4, F4, G4
	for i, expected := range expectedRoots {
		if chords[i].Root != expected {
			t.Errorf("chord %d: expected root %d, got %d", i, expected, chords[i].Root)
		}
	}
}

// TestParseProgression_DegreeQualityInferred verifies diatonic quality inference
// for C major. In C major:
//   - I (C) has major third (E) in scale → major
//   - II (D) has minor third (F) but not major third (F#) → minor
//   - IV (F) has major third (A) in scale → major
//   - V (G) has major third (B) in scale → major
func TestParseProgression_DegreeQualityInferred(t *testing.T) {
	root, scale, complexity := defaultProgConfig() // simple = triads
	chords, err := ParseProgression("1 2 4 5", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"major", "minor", "major", "major"}
	for i, q := range expected {
		if chords[i].Quality != q {
			t.Errorf("chord %d: expected quality %q, got %q", i, q, chords[i].Quality)
		}
	}
}

// TestParseProgression_DegreeQualityUpgradedAtMedium verifies that at medium
// complexity, inferred qualities are upgraded to 7th chords.
//   - major → maj7
//   - minor → min7
func TestParseProgression_DegreeQualityUpgradedAtMedium(t *testing.T) {
	root, scale := 60, "major"
	chords, err := ParseProgression("1 2", root, scale, "medium")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chords[0].Quality != "maj7" {
		t.Errorf("degree 1 at medium: expected maj7, got %q", chords[0].Quality)
	}
	if chords[1].Quality != "min7" {
		t.Errorf("degree 2 at medium: expected min7, got %q", chords[1].Quality)
	}
}

// TestParseProgression_DegreeExplicitQualityOverride verifies that an explicit
// quality suffix overrides diatonic inference.
// "1:dom7" should give degree 1 with quality "dom7", not "major".
func TestParseProgression_DegreeExplicitQualityOverride(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("1:dom7 4:minor 5:dom7", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"dom7", "minor", "dom7"}
	for i, q := range expected {
		if chords[i].Quality != q {
			t.Errorf("chord %d: expected quality %q, got %q", i, q, chords[i].Quality)
		}
	}
}

// TestParseProgression_DegreeOutOfRange verifies that degree 0 and degree 8
// return an error — valid degrees are 1–7.
func TestParseProgression_DegreeOutOfRange(t *testing.T) {
	root, scale, complexity := defaultProgConfig()

	for _, bad := range []string{"0", "8", "9"} {
		_, err := ParseProgression(bad, root, scale, complexity)
		if err == nil {
			t.Errorf("degree %q: expected error, got nil", bad)
		}
	}
}

// TestParseProgression_DegreeExceedsScaleLength verifies that requesting
// degree 6 or 7 from a pentatonic scale (5 notes) returns an error.
func TestParseProgression_DegreeExceedsScaleLength(t *testing.T) {
	_, err := ParseProgression("6", 60, "pentatonic", "simple")
	if err == nil {
		t.Error("expected error for degree 6 in pentatonic scale, got nil")
	}
}

// TestParseProgression_InvalidQualitySuffix verifies that an unrecognised
// quality name after the colon returns an error.
func TestParseProgression_InvalidQualitySuffix(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	_, err := ParseProgression("1:superchord", root, scale, complexity)
	if err == nil {
		t.Error("expected error for unknown quality suffix, got nil")
	}
}

// -----------------------------------------------------------------------------
// ParseProgression — note name format
// -----------------------------------------------------------------------------

// TestParseProgression_NoteNamesLength verifies that "C F G C" produces
// exactly 4 ProgChords.
func TestParseProgression_NoteNamesLength(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("C F G C", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chords) != 4 {
		t.Errorf("expected 4 chords, got %d", len(chords))
	}
}

// TestParseProgression_NoteNamesRoots verifies that note names resolve to the
// correct MIDI note numbers when root is C4 (60).
//
// With root=C4(60): C→60, F→65, G→67 (all in octave 4, same or above root)
func TestParseProgression_NoteNamesRoots(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("C F G", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRoots := []int{60, 65, 67} // C4, F4, G4
	for i, expected := range expectedRoots {
		if chords[i].Root != expected {
			t.Errorf("chord %d (%s): expected root %d, got %d",
				i, []string{"C", "F", "G"}[i], expected, chords[i].Root)
		}
	}
}

// TestParseProgression_NoteNameOctaveInference verifies that a note name below
// the root's pitch class is placed in the next octave up, not the same octave.
//
// Root = G4 (67). Note "C" has pitch class 0, which is below G's pitch class (7).
// Expected: C should be placed in octave 5 (C5 = 72), not octave 4 (C4 = 60).
func TestParseProgression_NoteNameOctaveInference(t *testing.T) {
	chords, err := ParseProgression("C", 67, "major", "simple") // root = G4
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chords) != 1 {
		t.Fatalf("expected 1 chord, got %d", len(chords))
	}
	// C is below G in pitch class order, so it should be bumped to octave 5
	if chords[0].Root != 72 { // C5
		t.Errorf("expected C5 (72) when root=G4 and note=C, got %d", chords[0].Root)
	}
}

// TestParseProgression_NoteNameSameAsPitchClass verifies that a note name with
// the same pitch class as the root stays in the same octave.
//
// Root = C4 (60). Note "C" → pitch class 0, root pitch class 0, same octave → C4=60.
func TestParseProgression_NoteNameSameAsPitchClass(t *testing.T) {
	chords, err := ParseProgression("C", 60, "major", "simple") // root = C4
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chords[0].Root != 60 { // C4
		t.Errorf("expected C4 (60), got %d", chords[0].Root)
	}
}

// TestParseProgression_NoteNameWithSharp verifies that sharped note names
// parse correctly. F# should resolve to MIDI 66 (F#4) when root=C4.
func TestParseProgression_NoteNameWithSharp(t *testing.T) {
	chords, err := ParseProgression("F#", 60, "lydian", "simple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chords[0].Root != 66 { // F#4
		t.Errorf("expected F#4 (66), got %d", chords[0].Root)
	}
}

// TestParseProgression_NoteNameWithFlat verifies that flat note names
// parse correctly. Bb should resolve to MIDI 70 (Bb4) when root=C4.
func TestParseProgression_NoteNameWithFlat(t *testing.T) {
	chords, err := ParseProgression("Bb", 60, "major", "simple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chords[0].Root != 70 { // Bb4
		t.Errorf("expected Bb4 (70), got %d", chords[0].Root)
	}
}

// TestParseProgression_NoteNameExplicitQuality verifies quality override works
// with note name format, same as with degrees.
func TestParseProgression_NoteNameExplicitQuality(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	chords, err := ParseProgression("C:maj7 F:minor G:dom7", root, scale, complexity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"maj7", "minor", "dom7"}
	for i, q := range expected {
		if chords[i].Quality != q {
			t.Errorf("chord %d: expected quality %q, got %q", i, q, chords[i].Quality)
		}
	}
}

// TestParseProgression_UnrecognisedNoteName verifies that an unrecognised
// note name (e.g. "X", "H") returns an error.
func TestParseProgression_UnrecognisedNoteName(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	_, err := ParseProgression("X", root, scale, complexity)
	if err == nil {
		t.Error("expected error for unrecognised note name, got nil")
	}
}

// -----------------------------------------------------------------------------
// ParseProgression — mixed format rejection
// -----------------------------------------------------------------------------

// TestParseProgression_MixedFormatsRejected verifies that mixing scale degrees
// and note names in one progression string returns an error.
// "1 F 5" is ambiguous — is F a note name or a hex digit?
func TestParseProgression_MixedFormatsRejected(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	_, err := ParseProgression("1 F 5", root, scale, complexity)
	if err == nil {
		t.Error("expected error for mixed degree/note-name formats, got nil")
	}
}

// TestParseProgression_DegreeThenNoteName verifies that starting with a degree
// then using a note name is rejected.
func TestParseProgression_DegreeThenNoteName(t *testing.T) {
	root, scale, complexity := defaultProgConfig()
	_, err := ParseProgression("1 4 G 1", root, scale, complexity)
	if err == nil {
		t.Error("expected error for degree followed by note name, got nil")
	}
}

// -----------------------------------------------------------------------------
// StepsPerChord tests
// -----------------------------------------------------------------------------

// TestStepsPerChord_AllCombinations verifies every valid quantize × chordrate
// combination produces the expected step count.
func TestStepsPerChord_AllCombinations(t *testing.T) {
	tests := []struct {
		chordRate string
		quantize  string
		expected  int
	}{
		{"beat", "quarter", 1},
		{"beat", "eighth", 2},
		{"beat", "sixteenth", 4},
		{"bar", "quarter", 4},
		{"bar", "eighth", 8},
		{"bar", "sixteenth", 16},
	}

	for _, tt := range tests {
		got, err := StepsPerChord(tt.chordRate, tt.quantize)
		if err != nil {
			t.Errorf("chordrate=%s quantize=%s: unexpected error: %v",
				tt.chordRate, tt.quantize, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("chordrate=%s quantize=%s: expected %d steps, got %d",
				tt.chordRate, tt.quantize, tt.expected, got)
		}
	}
}

// TestStepsPerChord_InvalidChordRate verifies that an unknown chordrate value
// returns an error.
func TestStepsPerChord_InvalidChordRate(t *testing.T) {
	_, err := StepsPerChord("measure", "eighth")
	if err == nil {
		t.Error("expected error for unknown chordrate, got nil")
	}
}

// TestStepsPerChord_InvalidQuantize verifies that an unknown quantize value
// returns an error.
func TestStepsPerChord_InvalidQuantize(t *testing.T) {
	_, err := StepsPerChord("bar", "halfnote")
	if err == nil {
		t.Error("expected error for unknown quantize, got nil")
	}
}

// -----------------------------------------------------------------------------
// ProgChordAt tests
// -----------------------------------------------------------------------------

// TestProgChordAt_FirstChord verifies that step 0 always returns the first chord.
func TestProgChordAt_FirstChord(t *testing.T) {
	prog := []ProgChord{
		{Root: 60, Quality: "major"},
		{Root: 65, Quality: "minor"},
		{Root: 67, Quality: "major"},
	}
	chord := ProgChordAt(prog, 0, 4)
	if chord.Root != 60 {
		t.Errorf("step 0: expected root 60, got %d", chord.Root)
	}
}

// TestProgChordAt_AdvancesAfterStepsPerChord verifies that the chord index
// advances after stepsPerChord steps have elapsed.
//
// With stepsPerChord=4: steps 0–3 → chord 0, steps 4–7 → chord 1.
func TestProgChordAt_AdvancesAfterStepsPerChord(t *testing.T) {
	prog := []ProgChord{
		{Root: 60, Quality: "major"},
		{Root: 65, Quality: "minor"},
	}
	stepsPerChord := 4

	for step := 0; step < stepsPerChord; step++ {
		chord := ProgChordAt(prog, step, stepsPerChord)
		if chord.Root != 60 {
			t.Errorf("step %d: expected root 60 (chord 0), got %d", step, chord.Root)
		}
	}
	for step := stepsPerChord; step < stepsPerChord*2; step++ {
		chord := ProgChordAt(prog, step, stepsPerChord)
		if chord.Root != 65 {
			t.Errorf("step %d: expected root 65 (chord 1), got %d", step, chord.Root)
		}
	}
}

// TestProgChordAt_CyclesAfterEnd verifies that after the last chord, the
// progression cycles back to the first chord.
//
// With 2 chords and stepsPerChord=4: step 8 → chord 0 again.
func TestProgChordAt_CyclesAfterEnd(t *testing.T) {
	prog := []ProgChord{
		{Root: 60, Quality: "major"},
		{Root: 65, Quality: "minor"},
	}
	stepsPerChord := 4

	// Step 8 = chord index (8/4) % 2 = 2 % 2 = 0 → back to first chord
	chord := ProgChordAt(prog, 8, stepsPerChord)
	if chord.Root != 60 {
		t.Errorf("step 8 (should cycle to chord 0): expected root 60, got %d", chord.Root)
	}
}

// TestProgChordAt_SingleChordNeverAdvances verifies that a one-chord progression
// always returns the same chord regardless of step.
func TestProgChordAt_SingleChordNeverAdvances(t *testing.T) {
	prog := []ProgChord{{Root: 60, Quality: "major"}}
	for step := 0; step < 100; step++ {
		chord := ProgChordAt(prog, step, 4)
		if chord.Root != 60 {
			t.Errorf("step %d: single-chord progression should always return root 60, got %d",
				step, chord.Root)
		}
	}
}

// TestProgChordAt_StepsPerChordOneAdvancesEveryStep verifies that with
// stepsPerChord=1, each step advances to the next chord.
func TestProgChordAt_StepsPerChordOneAdvancesEveryStep(t *testing.T) {
	prog := []ProgChord{
		{Root: 60, Quality: "major"},
		{Root: 62, Quality: "minor"},
		{Root: 64, Quality: "minor"},
		{Root: 65, Quality: "major"},
	}

	expectedRoots := []int{60, 62, 64, 65, 60, 62} // wraps at step 4
	for step, expected := range expectedRoots {
		chord := ProgChordAt(prog, step, 1)
		if chord.Root != expected {
			t.Errorf("step %d: expected root %d, got %d", step, expected, chord.Root)
		}
	}
}

// -----------------------------------------------------------------------------
// inferQuality tests
// -----------------------------------------------------------------------------

// TestInferQuality_MajorDegreeInCMajor verifies that a root with a major third
// in the C major scale produces "major" quality at simple complexity.
func TestInferQuality_MajorDegreeInCMajor(t *testing.T) {
	// C major scale pitch classes: 0,2,4,5,7,9,11
	scaleSet := map[int]bool{0: true, 2: true, 4: true, 5: true, 7: true, 9: true, 11: true}

	// C (root=60, pitchClass=0): major third = E (pitchClass=4) → in scale → major
	quality := inferQuality(60, scaleSet, "simple")
	if quality != "major" {
		t.Errorf("C in C major: expected major, got %q", quality)
	}
}

// TestInferQuality_MinorDegreeInCMajor verifies that a root without a major
// third in the scale produces "minor" quality.
func TestInferQuality_MinorDegreeInCMajor(t *testing.T) {
	scaleSet := map[int]bool{0: true, 2: true, 4: true, 5: true, 7: true, 9: true, 11: true}

	// D (root=62, pitchClass=2): major third = F# (pitchClass=6) → NOT in C major → minor
	quality := inferQuality(62, scaleSet, "simple")
	if quality != "minor" {
		t.Errorf("D in C major: expected minor, got %q", quality)
	}
}

// TestInferQuality_UpgradesToMaj7AtMedium verifies that a major triad becomes
// maj7 at medium complexity.
func TestInferQuality_UpgradesToMaj7AtMedium(t *testing.T) {
	scaleSet := map[int]bool{0: true, 2: true, 4: true, 5: true, 7: true, 9: true, 11: true}
	quality := inferQuality(60, scaleSet, "medium")
	if quality != "maj7" {
		t.Errorf("C major at medium: expected maj7, got %q", quality)
	}
}

// TestInferQuality_UpgradesToMin7AtComplex verifies that a minor triad becomes
// min7 at complex complexity.
func TestInferQuality_UpgradesToMin7AtComplex(t *testing.T) {
	scaleSet := map[int]bool{0: true, 2: true, 4: true, 5: true, 7: true, 9: true, 11: true}
	quality := inferQuality(62, scaleSet, "complex")
	if quality != "min7" {
		t.Errorf("D minor at complex: expected min7, got %q", quality)
	}
}
