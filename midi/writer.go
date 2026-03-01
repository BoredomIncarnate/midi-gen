package midi

import (
	"bytes"
	"encoding/binary"
	"os"
)

// encodeVarLen encodes a uint32 value as a MIDI variable-length quantity (VLQ).
//
// MIDI uses a compact variable-length encoding for delta times and some length
// fields. The scheme works as follows:
//
//   - The value is split into 7-bit groups, most significant group first.
//   - Each group is stored in one byte.
//   - The most significant bit (bit 7) of each byte is used as a "continuation
//     flag": 1 means "another byte follows", 0 means "this is the last byte".
//
// This means a single byte can represent values 0–127, two bytes 128–16383, etc.
//
// Examples:
//
//	0       → 0x00                         (1 byte,  binary: 0000 0000)
//	127     → 0x7F                         (1 byte,  binary: 0111 1111)
//	128     → 0x81 0x00                    (2 bytes, binary: 1000 0001  0000 0000)
//	255     → 0x81 0x7F                    (2 bytes, binary: 1000 0001  0111 1111)
//	16383   → 0xFF 0x7F                    (2 bytes, binary: 1111 1111  0111 1111)
//	16384   → 0x81 0x80 0x00               (3 bytes)
//	0xFFFFFFF → 0xFF 0xFF 0xFF 0x7F        (4 bytes, maximum representable value)
//
// Algorithm:
//  1. Collect 7-bit groups from least significant to most significant.
//  2. Set the high bit on every group except the last.
//  3. Reverse so the most significant group is written first.
func encodeVarLen(n uint32) []byte {
	// buf holds up to 4 bytes (uint32 max needs 4 x 7-bit groups = 28 bits)
	buf := make([]byte, 0, 4)

	// Step 1: extract 7-bit groups LSB-first
	// On each iteration:
	//   n & 0x7F  → isolate the lowest 7 bits (one group)
	//   n >>= 7   → shift right by 7 to move to the next group
	buf = append(buf, byte(n&0x7F)) // least significant 7-bit group, no continuation bit yet
	n >>= 7
	for n > 0 {
		// Set bit 7 (0x80) on this byte to signal "more bytes follow"
		buf = append(buf, byte(n&0x7F)|0x80)
		n >>= 7
	}

	// Step 2: reverse so the most significant group comes first
	// e.g. for value 128: buf starts as [0x00, 0x81], reversed → [0x81, 0x00]
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}

	return buf
}

// serializeTrack encodes a single Track into its raw MTrk chunk bytes.
//
// MTrk chunk layout:
//
//	bytes 0–3:   "MTrk"        4-byte ASCII magic identifier
//	bytes 4–7:   length        uint32 big-endian, number of bytes that follow
//	                           (does NOT include these 8 header bytes)
//	bytes 8–N:   event data    each event is: <varlen delta> <status> [<data bytes>...]
//	bytes N+1–N+3: end of track  meta-event: 0x00 0xFF 0x2F 0x00
//	                              delta=0x00, marker=0xFF, type=0x2F, length=0x00
//
// The length field at bytes 4–7 must reflect the total size of all serialized
// events INCLUDING the mandatory End of Track meta-event. Because we don't know
// this size until all events are encoded, we serialize events into a temporary
// buffer first, measure it, then write the header.
func serializeTrack(t Track) []byte {
	// --- Phase 1: serialize all events into a temporary buffer ---
	// We use a bytes.Buffer so we can measure its length after writing.
	var eventBuf bytes.Buffer

	for _, evt := range t.Events {
		// Write the variable-length delta time for this event.
		// Delta is the number of ticks since the previous event (or since the
		// start of the track for the very first event).
		eventBuf.Write(encodeVarLen(evt.Delta))

		// Write the raw event data bytes (status byte + data bytes).
		// These are already fully formed in Event.Data — no further encoding needed.
		eventBuf.Write(evt.Data)
	}

	// End of Track meta-event — mandatory, must be the last event in every track.
	// Without it, most DAWs and MIDI parsers will reject or misread the file.
	//
	// Byte layout:
	//   0x00  delta time = 0 (happens immediately after the last event)
	//   0xFF  meta-event marker
	//   0x2F  meta-event type: End of Track
	//   0x00  data length = 0 (no additional data follows)
	eventBuf.Write([]byte{0x00, 0xFF, 0x2F, 0x00})

	// --- Phase 2: build the MTrk chunk header ---
	var chunk bytes.Buffer

	// bytes 0–3: "MTrk" magic — identifies this chunk as a track chunk
	chunk.WriteString("MTrk")

	// bytes 4–7: chunk data length as a uint32 big-endian
	// This is the byte count of everything that follows these 8 header bytes,
	// i.e. all serialized events including the End of Track event.
	chunkLen := uint32(eventBuf.Len())
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, chunkLen)
	chunk.Write(lenBytes)

	// bytes 8–N: the serialized event data
	chunk.Write(eventBuf.Bytes())

	return chunk.Bytes()
}

// Serialize encodes the entire MIDI File into a byte slice ready to write to disk.
//
// Overall .mid file layout:
//
//	[MThd chunk — 14 bytes total]
//	  bytes  0–3:  "MThd"       magic bytes, identifies this as a MIDI file
//	  bytes  4–7:  0x00000006   header data length, always 6 for standard MIDI
//	  bytes  8–9:  Format       uint16 big-endian; 0=single track, 1=multi-track
//	  bytes 10–11: NumTracks    uint16 big-endian; derived from len(f.Tracks)
//	  bytes 12–13: TicksPerQN   uint16 big-endian; timing resolution
//
//	[MTrk chunk — variable length, one per track]
//	  bytes  0–3:  "MTrk"       magic bytes
//	  bytes  4–7:  length       uint32 big-endian, byte count of event data
//	  bytes  8–N:  event data   variable-length encoded events
func (f *File) Serialize() []byte {
	var buf bytes.Buffer

	// --- MThd header chunk ---

	// bytes 0–3: magic identifier — ASCII "MThd"
	buf.WriteString("MThd")

	// bytes 4–7: header data length — always exactly 6 bytes for standard MIDI.
	// This field exists for forward compatibility; a parser that encounters an
	// unknown length here should skip ahead rather than fail.
	buf.Write([]byte{0x00, 0x00, 0x00, 0x06})

	// bytes 8–9: file format
	//   0 = Format 0: single track, all channels multiplexed together
	//   1 = Format 1: multiple tracks, played simultaneously (most common for DAWs)
	//   2 = Format 2: multiple tracks, each representing an independent pattern
	formatBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(formatBytes, f.Format)
	buf.Write(formatBytes) // bytes 8–9

	// bytes 10–11: number of tracks
	// For Format 0 this must be 1. For Format 1 it is the count of MTrk chunks.
	numTracksBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(numTracksBytes, uint16(len(f.Tracks)))
	buf.Write(numTracksBytes) // bytes 10–11

	// bytes 12–13: ticks per quarter note (also called PPQN — pulses per quarter note)
	// This defines the timing resolution of the file.
	// Common values:
	//   96   — standard, compatible with most old hardware
	//   480  — higher resolution, preferred by modern DAWs (e.g. Ableton default)
	//   960  — very high resolution for detailed humanization
	// A value of 480 means one quarter note = 480 ticks, so a 16th note = 120 ticks.
	// Note: if bit 15 is set, the field uses SMPTE timecode format instead —
	// we do not use that here.
	tpqnBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(tpqnBytes, f.TicksPerQN)
	buf.Write(tpqnBytes) // bytes 12–13

	// --- MTrk chunks, one per track ---
	// Each track is fully self-contained. In Format 1, all tracks are played
	// back simultaneously, synced by their shared TicksPerQN resolution.
	for _, track := range f.Tracks {
		buf.Write(serializeTrack(track))
	}

	return buf.Bytes()
}

// WriteFile serializes the MIDI File and writes it to the file at path.
// The file is created or truncated, with permissions 0644 (owner rw, others r).
func WriteFile(path string, f *File) error {
	data := f.Serialize()
	return os.WriteFile(path, data, 0644)
}
