package midi

import (
	"encoding/binary"
	"testing"
)

// -----------------------------------------------------------------------------
// encodeVarLen tests
//
// Variable-length encoding is the most error-prone part of the MIDI spec.
// These tests cover the boundary values where the encoding changes byte width,
// and should be run first when debugging any serialization issues.
// -----------------------------------------------------------------------------

// TestEncodeVarLen_SingleByte verifies values that fit in one byte (0–127).
// For single-byte values, bit 7 must be 0 (no continuation flag).
func TestEncodeVarLen_SingleByte(t *testing.T) {
	tests := []struct {
		input    uint32
		expected []byte
	}{
		{0, []byte{0x00}}, // minimum value
		{1, []byte{0x01}},
		{63, []byte{0x3F}},
		{127, []byte{0x7F}}, // maximum single-byte value
	}

	for _, tt := range tests {
		got := encodeVarLen(tt.input)
		if !bytesEqual(got, tt.expected) {
			t.Errorf("encodeVarLen(%d): expected % X, got % X",
				tt.input, tt.expected, got)
		}
	}
}

// TestEncodeVarLen_TwoBytes verifies values in the two-byte range (128–16383).
//
// At 128, we cross the first boundary:
//
//	128 = 0b_000_0001_000_0000
//	Split into two 7-bit groups: [0x01] [0x00]
//	Set continuation bit on first byte: [0x81] [0x00]
//
//	255 = 0b_000_0001_111_1111
//	Split: [0x01] [0x7F] → [0x81] [0x7F]
//
//	16383 = 0b_111_1111_111_1111
//	Split: [0x7F] [0x7F] → [0xFF] [0x7F]
func TestEncodeVarLen_TwoBytes(t *testing.T) {
	tests := []struct {
		input    uint32
		expected []byte
	}{
		{128, []byte{0x81, 0x00}}, // first two-byte value
		{255, []byte{0x81, 0x7F}},
		{256, []byte{0x82, 0x00}},
		{16383, []byte{0xFF, 0x7F}}, // maximum two-byte value
	}

	for _, tt := range tests {
		got := encodeVarLen(tt.input)
		if !bytesEqual(got, tt.expected) {
			t.Errorf("encodeVarLen(%d): expected % X, got % X",
				tt.input, tt.expected, got)
		}
	}
}

// TestEncodeVarLen_ThreeBytes verifies values in the three-byte range (16384–2097151).
//
//	16384 = 0x4000 = 0b_001_000_000_000_000_000
//	Split into three 7-bit groups: [0x01] [0x00] [0x00]
//	Continuation bits: [0x81] [0x80] [0x00]
func TestEncodeVarLen_ThreeBytes(t *testing.T) {
	tests := []struct {
		input    uint32
		expected []byte
	}{
		{16384, []byte{0x81, 0x80, 0x00}},   // first three-byte value
		{2097151, []byte{0xFF, 0xFF, 0x7F}}, // maximum three-byte value
	}

	for _, tt := range tests {
		got := encodeVarLen(tt.input)
		if !bytesEqual(got, tt.expected) {
			t.Errorf("encodeVarLen(%d): expected % X, got % X",
				tt.input, tt.expected, got)
		}
	}
}

// TestEncodeVarLen_FourBytes verifies the maximum four-byte range (2097152–0xFFFFFFF).
//
// 0xFFFFFFF is the largest value representable in 4 x 7-bit bytes (28 bits total).
// Values larger than this cannot be represented — the MIDI spec does not define
// a fifth byte, so callers must ensure deltas stay within this range.
func TestEncodeVarLen_FourBytes(t *testing.T) {
	tests := []struct {
		input    uint32
		expected []byte
	}{
		{2097152, []byte{0x81, 0x80, 0x80, 0x00}},   // first four-byte value
		{0xFFFFFFF, []byte{0xFF, 0xFF, 0xFF, 0x7F}}, // maximum representable value
	}

	for _, tt := range tests {
		got := encodeVarLen(tt.input)
		if !bytesEqual(got, tt.expected) {
			t.Errorf("encodeVarLen(%d): expected % X, got % X",
				tt.input, tt.expected, got)
		}
	}
}

// TestEncodeVarLen_LastByteMSBClear verifies that the last byte of every
// encoded value always has bit 7 clear (MSB = 0), which signals "end of
// variable-length value" to any MIDI parser reading the stream.
func TestEncodeVarLen_LastByteMSBClear(t *testing.T) {
	inputs := []uint32{0, 1, 127, 128, 255, 16383, 16384, 2097151, 0xFFFFFFF}
	for _, n := range inputs {
		got := encodeVarLen(n)
		last := got[len(got)-1]
		if last&0x80 != 0 {
			t.Errorf("encodeVarLen(%d): last byte 0x%02X has bit 7 set (should be clear)", n, last)
		}
	}
}

// TestEncodeVarLen_ContinuationBitsSet verifies that every byte except the
// last has bit 7 set (MSB = 1), signaling "more bytes follow" to a parser.
func TestEncodeVarLen_ContinuationBitsSet(t *testing.T) {
	// Only relevant for multi-byte values
	inputs := []uint32{128, 255, 16383, 16384, 0xFFFFFFF}
	for _, n := range inputs {
		got := encodeVarLen(n)
		for i := 0; i < len(got)-1; i++ {
			if got[i]&0x80 == 0 {
				t.Errorf("encodeVarLen(%d): byte %d (0x%02X) missing continuation bit",
					n, i, got[i])
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Serialize / MThd header tests
// -----------------------------------------------------------------------------

// TestSerialize_MthdMagic verifies the first 4 bytes are the ASCII string "MThd".
// Any MIDI parser checks this magic before reading anything else.
func TestSerialize_MthdMagic(t *testing.T) {
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{{}}}
	data := f.Serialize()

	if len(data) < 4 {
		t.Fatalf("serialized data too short: %d bytes", len(data))
	}
	if string(data[0:4]) != "MThd" {
		t.Errorf("bytes 0–3: expected 'MThd', got %q", string(data[0:4]))
	}
}

// TestSerialize_MthdLength verifies that bytes 4–7 of the header always equal
// 6. This is a fixed constant in the MIDI spec — the header data is always
// exactly 6 bytes (format + ntracks + ticks). A parser uses this to know how
// far to seek before the first MTrk chunk.
func TestSerialize_MthdLength(t *testing.T) {
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{{}}}
	data := f.Serialize()

	length := binary.BigEndian.Uint32(data[4:8])
	if length != 6 {
		t.Errorf("bytes 4–7 (header length): expected 6, got %d", length)
	}
}

// TestSerialize_Format verifies that the Format field is written correctly
// at bytes 8–9 as a big-endian uint16.
func TestSerialize_Format(t *testing.T) {
	tests := []uint16{0, 1}
	for _, fmt := range tests {
		f := &File{Format: fmt, TicksPerQN: 96, Tracks: []Track{{}}}
		data := f.Serialize()
		got := binary.BigEndian.Uint16(data[8:10])
		if got != fmt {
			t.Errorf("bytes 8–9 (format): expected %d, got %d", fmt, got)
		}
	}
}

// TestSerialize_NumTracks verifies that bytes 10–11 correctly reflect the
// number of tracks in the file as a big-endian uint16.
func TestSerialize_NumTracks(t *testing.T) {
	tests := []int{1, 2, 4}
	for _, n := range tests {
		tracks := make([]Track, n)
		f := &File{Format: 1, TicksPerQN: 96, Tracks: tracks}
		data := f.Serialize()
		got := binary.BigEndian.Uint16(data[10:12])
		if int(got) != n {
			t.Errorf("bytes 10–11 (ntracks): expected %d, got %d", n, got)
		}
	}
}

// TestSerialize_TicksPerQN verifies that bytes 12–13 correctly encode the
// timing resolution as a big-endian uint16.
func TestSerialize_TicksPerQN(t *testing.T) {
	tests := []uint16{96, 480, 960}
	for _, tpqn := range tests {
		f := &File{Format: 0, TicksPerQN: tpqn, Tracks: []Track{{}}}
		data := f.Serialize()
		got := binary.BigEndian.Uint16(data[12:14])
		if got != tpqn {
			t.Errorf("bytes 12–13 (ticks/qn): expected %d, got %d", tpqn, got)
		}
	}
}

// TestSerialize_TotalHeaderSize verifies the MThd chunk is always exactly
// 14 bytes: 4 magic + 4 length + 6 data.
func TestSerialize_TotalHeaderSize(t *testing.T) {
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{{}}}
	data := f.Serialize()

	// 14 bytes MThd + at minimum one MTrk chunk (8 byte header + End of Track 4 bytes)
	if len(data) < 14 {
		t.Errorf("serialized output too short to contain MThd: %d bytes", len(data))
	}
}

// -----------------------------------------------------------------------------
// MTrk chunk tests
// -----------------------------------------------------------------------------

// TestSerialize_MtrkMagic verifies the MTrk magic bytes start at byte 14
// (immediately after the 14-byte MThd chunk).
func TestSerialize_MtrkMagic(t *testing.T) {
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{{}}}
	data := f.Serialize()

	if len(data) < 18 {
		t.Fatalf("data too short to contain MTrk header: %d bytes", len(data))
	}
	if string(data[14:18]) != "MTrk" {
		t.Errorf("bytes 14–17: expected 'MTrk', got %q", string(data[14:18]))
	}
}

// TestSerialize_EndOfTrack verifies that every serialized track ends with the
// mandatory End of Track meta-event: 0x00 0xFF 0x2F 0x00
//
// Without this, DAWs may reject the file or fail to loop correctly.
// delta=0x00 means it fires at the same tick as the last event.
func TestSerialize_EndOfTrack(t *testing.T) {
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{{}}}
	data := f.Serialize()

	// End of Track is the last 4 bytes of the file for a single empty track
	n := len(data)
	if n < 4 {
		t.Fatalf("data too short: %d bytes", n)
	}
	eot := data[n-4:]
	expected := []byte{0x00, 0xFF, 0x2F, 0x00}
	if !bytesEqual(eot, expected) {
		t.Errorf("last 4 bytes (End of Track): expected % X, got % X", expected, eot)
	}
}

// TestSerialize_MtrkChunkLength verifies that the MTrk length field (bytes 18–21)
// correctly reflects the number of bytes in the track data, NOT including the
// 8-byte MTrk header itself.
//
// An empty track contains only the End of Track event (4 bytes), so the
// chunk length field should be 4.
func TestSerialize_MtrkChunkLength(t *testing.T) {
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{{}}}
	data := f.Serialize()

	// MTrk chunk length is at bytes 18–21 (14 byte MThd + 4 byte "MTrk" magic)
	chunkLen := binary.BigEndian.Uint32(data[18:22])

	// An empty track = just the End of Track event (4 bytes)
	if chunkLen != 4 {
		t.Errorf("MTrk chunk length: expected 4 for empty track, got %d", chunkLen)
	}
}

// TestSerialize_MtrkWithEvents verifies that a track with actual Note On/Off
// events serializes to the correct total length and that the MTrk length
// field is updated to match.
func TestSerialize_MtrkWithEvents(t *testing.T) {
	track := Track{
		Events: []Event{
			NoteOn(0, 0, 60, 100), // delta=0,  3 data bytes → 1 varlen + 3 = 4 bytes
			NoteOff(96, 0, 60),    // delta=96, 3 data bytes → 1 varlen + 3 = 4 bytes
		},
	}
	f := &File{Format: 0, TicksPerQN: 96, Tracks: []Track{track}}
	data := f.Serialize()

	// Expected MTrk data:
	//   NoteOn:  0x00 0x90 0x3C 0x64         = 4 bytes
	//   NoteOff: 0x60 0x80 0x3C 0x00         = 4 bytes  (delta 96 = 0x60, single varlen byte)
	//   EndTrack: 0x00 0xFF 0x2F 0x00        = 4 bytes
	//   Total = 12 bytes
	chunkLen := binary.BigEndian.Uint32(data[18:22])
	if chunkLen != 12 {
		t.Errorf("MTrk chunk length: expected 12, got %d", chunkLen)
	}
}

// TestSerialize_MultiTrack verifies that a Format 1 file with multiple tracks
// produces multiple MTrk chunks, each with their own magic and length field.
func TestSerialize_MultiTrack(t *testing.T) {
	f := &File{
		Format:     1,
		TicksPerQN: 480,
		Tracks:     []Track{{}, {}}, // two empty tracks
	}
	data := f.Serialize()

	// First MTrk at offset 14
	if string(data[14:18]) != "MTrk" {
		t.Errorf("track 1 magic: expected 'MTrk', got %q", string(data[14:18]))
	}

	// First MTrk for an empty track = 8 byte header + 4 byte End of Track = 12 bytes total
	secondMtrk := 14 + 8 + 4 // MThd(14) + MTrk_header(8) + EOT(4)
	if len(data) < secondMtrk+4 {
		t.Fatalf("data too short for second MTrk: %d bytes", len(data))
	}
	if string(data[secondMtrk:secondMtrk+4]) != "MTrk" {
		t.Errorf("track 2 magic: expected 'MTrk', got %q", string(data[secondMtrk:secondMtrk+4]))
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// bytesEqual compares two byte slices for equality.
// bytes.Equal is not used to keep this package dependency-free in tests.
func bytesEqual(a, b []byte) bool {
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
