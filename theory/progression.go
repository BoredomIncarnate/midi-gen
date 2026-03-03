package theory

import (
	"fmt"
	"strconv"
	"strings"
)

// ProgChord represents one chord in a user-specified progression.
// It is the output of ParseProgression and the input to the generator
// functions when cfg.Progression is non-nil.
//
// Memory layout:
//
//	Root    int     MIDI note number of the chord root (0–127)
//	Quality string  chord quality key from ChordIntervals (e.g. "major", "min7")
type ProgChord struct {
	Root    int
	Quality string
}

// tokenFormat describes which input format was detected for a -prog string.
// All tokens in a single -prog string must use the same format — mixing
// degrees and note names in one progression is rejected to avoid ambiguity.
type tokenFormat int

const (
	formatUnknown  tokenFormat = iota
	formatDegree               // "1" "4" "5:dom7"
	formatNoteName             // "C" "F#" "Bb:maj7"
)

// ParseProgression parses the -prog flag string into a slice of ProgChord.
//
// The input string is a space-separated list of chord tokens. Each token is
// either a scale degree (1–7) or a note name (C, F#, Bb, etc.), optionally
// followed by a colon and a chord quality (e.g. "1:maj7", "C:min").
//
// All tokens must use the same format (all degrees or all note names).
// Mixing formats in one string returns an error.
//
// If prog is an empty string, ParseProgression returns (nil, nil) — nil
// signals to the generator that no progression was specified and random
// behaviour should be used. This is not an error.
//
// Parameters:
//
//	prog       — the raw -prog flag string, e.g. "1 4 5 1" or "C F G C"
//	rootNote   — MIDI note number of the -root flag (used for octave inference
//	             when resolving bare note names)
//	scaleName  — name of the scale (used for diatonic quality inference)
//	complexity — complexity level (used to upgrade triads to 7ths at medium/complex)
func ParseProgression(prog string, rootNote int, scaleName string, complexity string) ([]ProgChord, error) {
	prog = strings.TrimSpace(prog)
	if prog == "" {
		// Empty string is not an error — it means "use random progression"
		return nil, nil
	}

	// Build the scale note set for quality inference.
	// scaleNotes gives us the MIDI note numbers of one octave from the root.
	scaleNotes, err := ScaleNotes(rootNote, scaleName, 1)
	if err != nil {
		return nil, fmt.Errorf("ParseProgression: could not build scale: %w", err)
	}

	// scaleSet maps pitch class (note % 12) → true for fast membership testing.
	// Used by inferQuality to check whether the major third above a root is in-scale.
	scaleSet := make(map[int]bool)
	for _, n := range scaleNotes {
		scaleSet[n%12] = true
	}

	tokens := strings.Fields(prog)
	if len(tokens) == 0 {
		return nil, nil
	}

	// Detect the format of the first non-empty token to enforce consistency.
	// We detect by attempting to parse the base token (before any colon) as
	// an integer — if it succeeds it is a degree, otherwise a note name.
	detectedFormat := formatUnknown

	chords := make([]ProgChord, 0, len(tokens))

	for _, token := range tokens {
		// Split token into base and optional quality override.
		// e.g. "C:maj7" → base="C", qualityOverride="maj7"
		//      "1:dom7" → base="1", qualityOverride="dom7"
		//      "F#"     → base="F#", qualityOverride=""
		base, qualityOverride := splitToken(token)

		// Validate explicit quality override if present
		if qualityOverride != "" {
			if _, ok := ChordIntervals[qualityOverride]; !ok {
				return nil, fmt.Errorf("ParseProgression: unknown chord quality %q in token %q (valid qualities: use ChordQualities())", qualityOverride, token)
			}
		}

		// Attempt to parse base as an integer to detect format
		if degree, err := strconv.Atoi(base); err == nil {
			// Successfully parsed as integer — this is a scale degree token
			if detectedFormat == formatNoteName {
				return nil, fmt.Errorf("ParseProgression: mixed formats detected — token %q looks like a degree but earlier tokens were note names; use one format throughout", token)
			}
			detectedFormat = formatDegree

			chord, err := parseDegree(degree, qualityOverride, scaleNotes, scaleSet, complexity)
			if err != nil {
				return nil, fmt.Errorf("ParseProgression: %w", err)
			}
			chords = append(chords, chord)

		} else {
			// Not an integer — treat as a note name
			if detectedFormat == formatDegree {
				return nil, fmt.Errorf("ParseProgression: mixed formats detected — token %q looks like a note name but earlier tokens were scale degrees; use one format throughout", token)
			}
			detectedFormat = formatNoteName

			chord, err := parseNoteName(base, qualityOverride, rootNote, scaleSet, complexity)
			if err != nil {
				return nil, fmt.Errorf("ParseProgression: %w", err)
			}
			chords = append(chords, chord)
		}
	}

	return chords, nil
}

// splitToken splits a chord token into its base part and optional quality override.
//
// A colon separates the base from the quality suffix:
//
//	"1"       → ("1", "")
//	"C"       → ("C", "")
//	"1:maj7"  → ("1", "maj7")
//	"F#:min"  → ("F#", "min")
//
// If more than one colon is present, everything after the first colon is
// treated as the quality string (allowing future extension without format changes).
func splitToken(token string) (base, quality string) {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// parseDegree resolves a scale degree (1–7) to a ProgChord.
//
// The degree selects a note from scaleNotes (1-indexed, so degree 1 = scaleNotes[0]).
// Quality is taken from qualityOverride if non-empty, otherwise inferred diatonically.
//
// Valid degrees: 1–7. Degree 0, 8, or any out-of-range value returns an error.
//
// Parameters:
//
//	degree          — integer degree value (1–7)
//	qualityOverride — explicit quality string, or "" to infer from scale
//	scaleNotes      — MIDI note numbers for one octave of the scale from the root
//	scaleSet        — pitch class membership map for quality inference
//	complexity      — controls whether triads are upgraded to 7ths
func parseDegree(degree int, qualityOverride string, scaleNotes []int, scaleSet map[int]bool, complexity string) (ProgChord, error) {
	if degree < 1 || degree > 7 {
		return ProgChord{}, fmt.Errorf("scale degree %d out of range — valid degrees are 1–7", degree)
	}
	if degree > len(scaleNotes) {
		// Some scales have fewer than 7 notes (e.g. pentatonic = 5 notes)
		return ProgChord{}, fmt.Errorf("scale degree %d exceeds scale length (%d notes) — choose a degree within the scale", degree, len(scaleNotes))
	}

	// scaleNotes is 0-indexed; degree is 1-indexed
	root := scaleNotes[degree-1]

	quality := qualityOverride
	if quality == "" {
		quality = inferQuality(root, scaleSet, complexity)
	}

	return ProgChord{Root: root, Quality: quality}, nil
}

// parseNoteName resolves a bare note name (e.g. "C", "F#", "Bb") to a ProgChord.
//
// The note name is parsed without an octave number — the octave is inferred
// from rootNote so the chord root lands in the same register as the piece.
//
// Octave inference logic:
//  1. Determine the pitch class of the named note (0–11)
//  2. Start from the octave of rootNote
//  3. If the named pitch class is below the root's pitch class in that octave,
//     use the next octave up so the chord root is above or equal to the root
//
// This ensures chord roots stay in a musically sensible register rather than
// jumping to octave 0 for notes below the root's pitch class.
//
// Parameters:
//
//	noteName        — bare note name without octave, e.g. "C", "F#", "Bb"
//	qualityOverride — explicit quality string, or "" to infer from scale
//	rootNote        — MIDI note number of the -root flag (for octave inference)
//	scaleSet        — pitch class membership map for quality inference
//	complexity      — controls whether triads are upgraded to 7ths
func parseNoteName(noteName string, qualityOverride string, rootNote int, scaleSet map[int]bool, complexity string) (ProgChord, error) {
	// Look up the pitch class (0–11) of the named note.
	// We parse it as octave 4 first to get the pitch class, then adjust octave.
	// NoteNumber requires an octave digit, so we append "4" as a placeholder.
	trial, err := NoteNumber(noteName + "4")
	if err != nil {
		return ProgChord{}, fmt.Errorf("unrecognised note name %q: %w", noteName, err)
	}

	// Pitch class of the named note (0=C, 1=C#, ..., 11=B)
	pitchClass := trial % 12

	// Pitch class and octave of the root note
	rootPitchClass := rootNote % 12
	rootOctave := (rootNote / 12) - 1 // MIDI octave: C4=60 → octave 4

	// Place the chord root in rootOctave by default.
	// If the named pitch class is below the root's pitch class, use the next
	// octave up so the chord root doesn't fall below the piece's root note.
	octave := rootOctave
	if pitchClass < rootPitchClass {
		octave++
	}

	// Reconstruct the full note name with the inferred octave and parse it
	midiNote, err := NoteNumber(fmt.Sprintf("%s%d", noteName, octave))
	if err != nil {
		return ProgChord{}, fmt.Errorf("could not resolve %s in octave %d: %w", noteName, octave, err)
	}

	// Clamp to valid MIDI range
	if midiNote < 0 || midiNote > 127 {
		return ProgChord{}, fmt.Errorf("note %s%d (MIDI %d) is outside valid range 0–127", noteName, octave, midiNote)
	}

	quality := qualityOverride
	if quality == "" {
		quality = inferQuality(midiNote, scaleSet, complexity)
	}

	return ProgChord{Root: midiNote, Quality: quality}, nil
}

// inferQuality determines whether a chord rooted at root should be major or
// minor, based on whether the major third (root+4 semitones) exists in the scale.
//
// At medium or complex complexity, triads are upgraded to 7th chords:
//
//	major → maj7
//	minor → min7
//
// This mirrors the diatonic quality inference used in generateProgression,
// keeping the sound consistent whether the user specifies the progression
// explicitly or lets the generator choose randomly.
//
// Parameters:
//
//	root       — MIDI note number of the chord root
//	scaleSet   — map of pitch class (note%12) → true for in-scale notes
//	complexity — "simple", "medium", or "complex"
func inferQuality(root int, scaleSet map[int]bool, complexity string) string {
	// Check whether the major third above root is in the scale.
	// We use pitch class arithmetic (mod 12) to avoid octave dependency.
	majorThird := (root + 4) % 12
	quality := "minor"
	if scaleSet[majorThird] {
		quality = "major"
	}

	// Upgrade to 7th chords at medium and complex complexity
	if complexity == "medium" || complexity == "complex" {
		if quality == "major" {
			quality = "maj7"
		} else {
			quality = "min7"
		}
	}

	return quality
}

// StepsPerChord returns how many generator steps each chord in the progression
// occupies before advancing to the next chord.
//
// The step count is derived from the chordRate (beat or bar) and the quantize
// grid (quarter, eighth, sixteenth), assuming 4/4 time.
//
// Steps per beat by quantize:
//
//	quarter   → 1 step per beat  (one step IS one beat)
//	eighth    → 2 steps per beat (two eighth notes per beat)
//	sixteenth → 4 steps per beat (four sixteenth notes per beat)
//
// Steps per bar = steps per beat × 4 (four beats per bar in 4/4 time).
//
//	chordrate=beat, quantize=quarter   → 1 step per chord
//	chordrate=beat, quantize=eighth    → 2 steps per chord
//	chordrate=beat, quantize=sixteenth → 4 steps per chord
//	chordrate=bar,  quantize=quarter   → 4 steps per chord
//	chordrate=bar,  quantize=eighth    → 8 steps per chord
//	chordrate=bar,  quantize=sixteenth → 16 steps per chord
func StepsPerChord(chordRate string, quantize string) (int, error) {
	// Steps per beat depends on the quantize grid
	var stepsPerBeat int
	switch quantize {
	case "quarter":
		stepsPerBeat = 1
	case "eighth":
		stepsPerBeat = 2
	case "sixteenth":
		stepsPerBeat = 4
	default:
		return 0, fmt.Errorf("StepsPerChord: unknown quantize value %q", quantize)
	}

	switch chordRate {
	case "beat":
		return stepsPerBeat, nil
	case "bar":
		// 4/4 time: 4 beats per bar
		return stepsPerBeat * 4, nil
	default:
		return 0, fmt.Errorf("StepsPerChord: unknown chordrate %q (want beat|bar)", chordRate)
	}
}

// ProgChordAt returns the ProgChord that is active at a given step index,
// cycling through the progression if the step exceeds its length.
//
// This is the primary lookup used by the generator loops. It handles
// progression cycling transparently so the generator does not need to
// manage the wrap-around logic itself.
//
// Parameters:
//
//	prog          — the parsed progression slice (length >= 1)
//	step          — the current generator step index (0-based)
//	stepsPerChord — how many steps each chord occupies (from StepsPerChord)
//
// Returns the ProgChord active at the given step.
// The chord index is: (step / stepsPerChord) % len(prog)
func ProgChordAt(prog []ProgChord, step int, stepsPerChord int) ProgChord {
	if stepsPerChord < 1 {
		stepsPerChord = 1
	}
	// Which chord slot does this step fall into?
	chordIndex := (step / stepsPerChord) % len(prog)
	return prog[chordIndex]
}
