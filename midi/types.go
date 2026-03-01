package midi

// Event represents a single MIDI event with a delta-time and raw data bytes.
type Event struct {
	Delta uint32 // ticks since the previous event
	Data  []byte // raw MIDI status + data bytes
}

// Track holds an ordered sequence of MIDI events.
type Track struct {
	Events []Event
}

// File represents a complete MIDI file (Format 0 or 1).
type File struct {
	Format     uint16  // 0 = single track, 1 = multi-track
	TicksPerQN uint16  // ticks per quarter note (e.g. 96, 480)
	Tracks     []Track
}
