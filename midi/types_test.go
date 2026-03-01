package midi

import (
	"testing"
)

// -----------------------------------------------------------------------------
// NoteOn tests
// -----------------------------------------------------------------------------

// TestNoteOn_HappyPath verifies that a standard Note On event is encoded
// correctly for a typical use case (channel 0, middle C, moderate velocity).
//
// Expected Data bytes:
//
//	byte 0: 0x90 | 0x00 = 0x90  (Note On, channel 0)
//	byte 1: 0x3C             (key 60 = middle C)
//	byte 2: 0x64             (velocity 100)
func TestNoteOn_HappyPath(t *testing.T) {
	evt := NoteOn(0, 0, 60, 100)

	if evt.Delta != 0 {
		t.Errorf("expected Delta=0, got %d", evt.Delta)
	}
	if len(evt.Data) != 3 {
		t.Fatalf("expected 3 data bytes, got %d", len(evt.Data))
	}
	if evt.Data[0] != 0x90 {
		t.Errorf("byte 0 (status): expected 0x90, got 0x%02X", evt.Data[0])
	}
	if evt.Data[1] != 60 {
		t.Errorf("byte 1 (key): expected 60, got %d", evt.Data[1])
	}
	if evt.Data[2] != 100 {
		t.Errorf("byte 2 (velocity): expected 100, got %d", evt.Data[2])
	}
}

// TestNoteOn_ChannelEncoding verifies that the channel nibble is correctly
// OR'd into the lower 4 bits of the status byte across all 16 MIDI channels.
//
// MIDI channels are 0–15. The status byte is: 0x90 | channel.
// Channel 0  → 0x90
// Channel 1  → 0x91
// Channel 9  → 0x99  (General MIDI drums channel)
// Channel 15 → 0x9F
func TestNoteOn_ChannelEncoding(t *testing.T) {
	tests := []struct {
		channel  byte
		expected byte
	}{
		{0, 0x90},
		{1, 0x91},
		{9, 0x99}, // GM drums
		{15, 0x9F},
	}

	for _, tt := range tests {
		evt := NoteOn(0, tt.channel, 60, 100)
		if evt.Data[0] != tt.expected {
			t.Errorf("channel %d: expected status 0x%02X, got 0x%02X",
				tt.channel, tt.expected, evt.Data[0])
		}
	}
}

// TestNoteOn_ChannelClamping verifies that channel values above 15 are clamped
// to 4 bits via & 0x0F, preventing corruption of the status byte's upper nibble.
//
// channel=16 (0x10) → 0x10 & 0x0F = 0x00 → status = 0x90 | 0x00 = 0x90
// channel=17 (0x11) → 0x11 & 0x0F = 0x01 → status = 0x90 | 0x01 = 0x91
// channel=255(0xFF) → 0xFF & 0x0F = 0x0F → status = 0x90 | 0x0F = 0x9F
func TestNoteOn_ChannelClamping(t *testing.T) {
	tests := []struct {
		channel  byte
		expected byte
	}{
		{16, 0x90},  // wraps to channel 0
		{17, 0x91},  // wraps to channel 1
		{255, 0x9F}, // wraps to channel 15
	}

	for _, tt := range tests {
		evt := NoteOn(0, tt.channel, 60, 100)
		if evt.Data[0] != tt.expected {
			t.Errorf("channel %d: expected status 0x%02X, got 0x%02X",
				tt.channel, tt.expected, evt.Data[0])
		}
	}
}

// TestNoteOn_KeyClamping verifies that key values with bit 7 set are masked
// to 7 bits via & 0x7F. MIDI data bytes must always have bit 7 = 0.
//
// key=128 (0x80) → 0x80 & 0x7F = 0x00
// key=255 (0xFF) → 0xFF & 0x7F = 0x7F
func TestNoteOn_KeyClamping(t *testing.T) {
	tests := []struct {
		key      byte
		expected byte
	}{
		{127, 127}, // max valid key, no clamping
		{128, 0},   // 0x80 & 0x7F = 0x00
		{255, 127}, // 0xFF & 0x7F = 0x7F
	}

	for _, tt := range tests {
		evt := NoteOn(0, 0, tt.key, 100)
		if evt.Data[1] != tt.expected {
			t.Errorf("key %d: expected %d, got %d", tt.key, tt.expected, evt.Data[1])
		}
	}
}

// TestNoteOn_VelocityClamping mirrors the key clamping test for velocity.
// Velocity 0 is a special case — most devices treat NoteOn(vel=0) as NoteOff.
func TestNoteOn_VelocityClamping(t *testing.T) {
	tests := []struct {
		velocity byte
		expected byte
	}{
		{0, 0},     // silent / treated as note off by many devices
		{1, 1},     // minimum audible velocity
		{127, 127}, // maximum velocity, no clamping
		{128, 0},   // 0x80 & 0x7F = 0x00
		{255, 127}, // 0xFF & 0x7F = 0x7F
	}

	for _, tt := range tests {
		evt := NoteOn(0, 0, 60, tt.velocity)
		if evt.Data[2] != tt.expected {
			t.Errorf("velocity %d: expected %d, got %d",
				tt.velocity, tt.expected, evt.Data[2])
		}
	}
}

// TestNoteOn_DeltaPreserved verifies that the delta time is stored as-is
// in the Event struct (encoding to VLQ happens in the writer, not here).
func TestNoteOn_DeltaPreserved(t *testing.T) {
	tests := []uint32{0, 1, 96, 127, 128, 480, 0xFFFFFFF}
	for _, delta := range tests {
		evt := NoteOn(delta, 0, 60, 100)
		if evt.Delta != delta {
			t.Errorf("delta %d: expected %d, got %d", delta, delta, evt.Delta)
		}
	}
}

// -----------------------------------------------------------------------------
// NoteOff tests
// -----------------------------------------------------------------------------

// TestNoteOff_HappyPath verifies a standard Note Off event.
//
// Expected Data bytes:
//
//	byte 0: 0x80 | 0x00 = 0x80  (Note Off, channel 0)
//	byte 1: 0x3C             (key 60 = middle C)
//	byte 2: 0x00             (release velocity, always 0 for standard use)
func TestNoteOff_HappyPath(t *testing.T) {
	evt := NoteOff(96, 0, 60)

	if evt.Delta != 96 {
		t.Errorf("expected Delta=96, got %d", evt.Delta)
	}
	if len(evt.Data) != 3 {
		t.Fatalf("expected 3 data bytes, got %d", len(evt.Data))
	}
	if evt.Data[0] != 0x80 {
		t.Errorf("byte 0 (status): expected 0x80, got 0x%02X", evt.Data[0])
	}
	if evt.Data[1] != 60 {
		t.Errorf("byte 1 (key): expected 60, got %d", evt.Data[1])
	}
	if evt.Data[2] != 0x00 {
		t.Errorf("byte 2 (release velocity): expected 0x00, got 0x%02X", evt.Data[2])
	}
}

// TestNoteOff_StatusByte verifies the Note Off status byte across channels.
// Note Off status = 0x80 | channel (upper nibble 0x8 = Note Off).
func TestNoteOff_StatusByte(t *testing.T) {
	tests := []struct {
		channel  byte
		expected byte
	}{
		{0, 0x80},
		{1, 0x81},
		{15, 0x8F},
		{16, 0x80}, // clamped: 16 & 0x0F = 0
	}

	for _, tt := range tests {
		evt := NoteOff(0, tt.channel, 60)
		if evt.Data[0] != tt.expected {
			t.Errorf("channel %d: expected 0x%02X, got 0x%02X",
				tt.channel, tt.expected, evt.Data[0])
		}
	}
}

// -----------------------------------------------------------------------------
// Tempo tests
// -----------------------------------------------------------------------------

// TestTempo_HappyPath verifies the Set Tempo meta-event at 120 BPM.
//
// 120 BPM → 60,000,000 / 120 = 500,000 µs/qn = 0x07A120
//
// Expected Data bytes:
//
//	byte 0: 0xFF  meta-event marker
//	byte 1: 0x51  Set Tempo type
//	byte 2: 0x03  data length (always 3)
//	byte 3: 0x07  high byte of 500,000
//	byte 4: 0xA1  mid byte of 500,000
//	byte 5: 0x20  low byte of 500,000
func TestTempo_HappyPath(t *testing.T) {
	evt := Tempo(0, 120)

	if len(evt.Data) != 6 {
		t.Fatalf("expected 6 data bytes, got %d", len(evt.Data))
	}
	if evt.Data[0] != 0xFF {
		t.Errorf("byte 0 (meta marker): expected 0xFF, got 0x%02X", evt.Data[0])
	}
	if evt.Data[1] != 0x51 {
		t.Errorf("byte 1 (tempo type): expected 0x51, got 0x%02X", evt.Data[1])
	}
	if evt.Data[2] != 0x03 {
		t.Errorf("byte 2 (data length): expected 0x03, got 0x%02X", evt.Data[2])
	}
	if evt.Data[3] != 0x07 {
		t.Errorf("byte 3 (uspqn high): expected 0x07, got 0x%02X", evt.Data[3])
	}
	if evt.Data[4] != 0xA1 {
		t.Errorf("byte 4 (uspqn mid): expected 0xA1, got 0x%02X", evt.Data[4])
	}
	if evt.Data[5] != 0x20 {
		t.Errorf("byte 5 (uspqn low): expected 0x20, got 0x%02X", evt.Data[5])
	}
}

// TestTempo_Roundtrip verifies that BPM → uspqn → bytes encodes correctly
// for a variety of common tempos. We reconstruct uspqn from the 3 bytes and
// check it matches the expected value.
func TestTempo_Roundtrip(t *testing.T) {
	tests := []struct {
		bpm           int
		expectedUspqn uint32
	}{
		{60, 1_000_000}, // 60 BPM  → 1,000,000 µs/qn  (exactly 1 second per beat)
		{120, 500_000},  // 120 BPM →   500,000 µs/qn
		{140, 428_571},  // 140 BPM →   428,571 µs/qn (integer division)
		{200, 300_000},  // 200 BPM →   300,000 µs/qn
	}

	for _, tt := range tests {
		evt := Tempo(0, tt.bpm)

		// Reconstruct the 24-bit uspqn from bytes 3, 4, 5
		got := uint32(evt.Data[3])<<16 | uint32(evt.Data[4])<<8 | uint32(evt.Data[5])
		if got != tt.expectedUspqn {
			t.Errorf("bpm=%d: expected uspqn=%d, got %d", tt.bpm, tt.expectedUspqn, got)
		}
	}
}
