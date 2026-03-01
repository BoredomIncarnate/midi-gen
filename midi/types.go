package midi

// Event represents a single MIDI event as it will appear in a track chunk.
//
// Memory layout (logical, not packed struct):
//
//	Delta  uint32   variable-length encoded ticks when serialized (1–4 bytes on disk)
//	                represents time elapsed since the previous event in the track
//	Data   []byte   one or more bytes: status byte first, then data bytes
//	                the status byte encodes both the message type and channel
type Event struct {
	Delta uint32 // ticks since the previous event
	Data  []byte // raw MIDI status + data bytes
}

// NoteOn returns a Note On event — tells a MIDI device to start playing a note.
//
// On-disk byte layout (3 bytes in Data):
//
//	byte 0: status = 0x90 | channel
//	        0x90 = 1001 0000 — upper nibble 0x9 means "Note On"
//	        channel & 0x0F  — lower nibble clamps channel to 0–15
//	byte 1: key      — MIDI note number 0–127 (middle C = 60)
//	                   & 0x7F masks off the high bit (must be 0 for data bytes)
//	byte 2: velocity — how hard the note is struck, 0–127
//	                   & 0x7F same masking; velocity 0 is treated as Note Off by most devices
func NoteOn(delta uint32, channel, key, velocity byte) Event {
	return Event{
		Delta: delta,
		Data:  []byte{0x90 | (channel & 0x0F), key & 0x7F, velocity & 0x7F},
	}
}

// NoteOff returns a Note Off event — tells a MIDI device to stop playing a note.
//
// On-disk byte layout (3 bytes in Data):
//
//	byte 0: status = 0x80 | channel
//	        0x80 = 1000 0000 — upper nibble 0x8 means "Note Off"
//	        channel & 0x0F  — lower nibble clamps channel to 0–15
//	byte 1: key      — must match the key number used in the corresponding NoteOn
//	                   & 0x7F masks off the high bit
//	byte 2: velocity — release velocity; 0x00 is standard (most devices ignore this)
func NoteOff(delta uint32, channel, key byte) Event {
	return Event{
		Delta: delta,
		Data:  []byte{0x80 | (channel & 0x0F), key & 0x7F, 0x00},
	}
}

// Tempo returns a Set Tempo meta-event — tells the sequencer how fast to play.
//
// MIDI does not store BPM directly. It stores microseconds per quarter note (uspqn).
// Conversion: uspqn = 60,000,000 / BPM
// e.g. 120 BPM → 500,000 µs/qn → 0x07A120
//
// On-disk byte layout (6 bytes in Data):
//
//	byte 0: 0xFF     — meta-event marker (not a real MIDI channel message)
//	byte 1: 0x51     — meta-event type: Set Tempo
//	byte 2: 0x03     — length of the following data: always 3 bytes for tempo
//	byte 3: tt       — most significant byte of uspqn  (bits 16–23)
//	byte 4: tt       — middle byte of uspqn            (bits  8–15)
//	byte 5: tt       — least significant byte of uspqn (bits  0–7)
//
// This event should be placed at delta=0 at the start of the track.
// If omitted, DAWs default to 120 BPM.
func Tempo(delta uint32, bpm int) Event {
	uspqn := 60_000_000 / bpm
	return Event{
		Delta: delta,
		Data: []byte{
			0xFF, 0x51, 0x03,
			byte(uspqn >> 16),
			byte(uspqn >> 8),
			byte(uspqn),
		},
	}
}

// Track holds an ordered sequence of MIDI events for a single instrument or voice.
// When serialized, it becomes one MTrk chunk in the .mid file.
//
// Memory layout:
//
//	Events []Event   slice of events in chronological order
//	                 each Event carries its own delta time relative to the previous
type Track struct {
	Events []Event
}

// File is the top-level structure representing a complete .mid file.
//
// Memory layout mirrors the MIDI file header chunk (MThd, always 14 bytes on disk):
//
//	bytes 0–3:  "MThd"       magic bytes identifying this as a MIDI file
//	bytes 4–7:  0x00000006   length of header data: always 6 for standard MIDI
//	bytes 8–9:  Format       0 = single track, 1 = multi-track synchronous
//	bytes 10–11: num tracks  derived from len(Tracks) at serialization time
//	bytes 12–13: TicksPerQN  timing resolution; 96 or 480 are common values
//	                         e.g. 96 means one quarter note = 96 ticks
//	                         higher values = finer timing resolution
type File struct {
	Format     uint16 // 0 = single track, 1 = multi-track
	TicksPerQN uint16 // ticks per quarter note (e.g. 96, 480)
	Tracks     []Track
}
