package theory

import "fmt"

// ChordIntervals maps chord quality names to semitone intervals from the root.
var ChordIntervals = map[string][]int{
	"major": {0, 4, 7},
	"minor": {0, 3, 7},
	"dom7":  {0, 4, 7, 10},
	"maj7":  {0, 4, 7, 11},
	"min7":  {0, 3, 7, 10},
	"dim":   {0, 3, 6},
	"aug":   {0, 4, 8},
	"sus2":  {0, 2, 7},
	"sus4":  {0, 5, 7},
}

// BuildChord returns the MIDI note numbers for a chord given a root note and quality.
func BuildChord(root int, quality string) ([]int, error) {
	intervals, ok := ChordIntervals[quality]
	if !ok {
		return nil, fmt.Errorf("unknown chord quality: %s", quality)
	}
	// TODO: apply intervals to root, return note numbers
	_ = intervals
	return nil, fmt.Errorf("BuildChord: not yet implemented")
}
