package theory

import (
	"fmt"
	"strings"
)

var noteNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// Scales maps scale names to their semitone intervals from the root.
var Scales = map[string][]int{
	"major":      {0, 2, 4, 5, 7, 9, 11},
	"minor":      {0, 2, 3, 5, 7, 8, 10},
	"pentatonic": {0, 2, 4, 7, 9},
	"blues":      {0, 3, 5, 6, 7, 10},
	"dorian":     {0, 2, 3, 5, 7, 9, 10},
}

// NoteNumber converts a note name + octave string (e.g. "C4", "F#3") to a MIDI note number.
// Middle C (C4) = 60.
func NoteNumber(nameOctave string) (int, error) {
	// TODO: parse note name and octave, return MIDI number
	_ = strings.ToUpper(nameOctave)
	return 0, fmt.Errorf("NoteNumber: not yet implemented")
}

// ScaleNotes returns all MIDI note numbers in the given scale, root, across the given number of octaves.
func ScaleNotes(root int, scaleName string, octaves int) ([]int, error) {
	intervals, ok := Scales[scaleName]
	if !ok {
		return nil, fmt.Errorf("unknown scale: %s", scaleName)
	}
	// TODO: build note list across octaves
	_ = intervals
	return nil, fmt.Errorf("ScaleNotes: not yet implemented")
}
