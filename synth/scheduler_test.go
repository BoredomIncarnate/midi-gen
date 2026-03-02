package synth

import (
	"math"
	"testing"

	"midi-gen/midi"
)

// defaultScheduler returns a Scheduler configured for deterministic testing.
// BPM=120, TicksPerQN=480 are the standard DAW defaults and produce round
// sample counts that are easy to reason about in tests.
func defaultScheduler() *Scheduler {
	return NewScheduler(120, 480, DefaultADSR(), 0.3)
}

// -----------------------------------------------------------------------------
// TicksToSamples tests
// -----------------------------------------------------------------------------

// TestTicksToSamples_QuarterNote verifies that one quarter note (480 ticks at
// TicksPerQN=480) at 120 BPM converts to exactly 22050 samples.
//
// Derivation:
//
//	120 BPM → 0.5 seconds per beat
//	0.5 seconds * 44100 Hz = 22050 samples
func TestTicksToSamples_QuarterNote(t *testing.T) {
	s := defaultScheduler()
	got := s.TicksToSamples(480)
	if got != 22050 {
		t.Errorf("quarter note: expected 22050 samples, got %d", got)
	}
}

// TestTicksToSamples_EighthNote verifies that half a quarter note (240 ticks)
// converts to 11025 samples at 120 BPM.
func TestTicksToSamples_EighthNote(t *testing.T) {
	s := defaultScheduler()
	got := s.TicksToSamples(240)
	if got != 11025 {
		t.Errorf("eighth note: expected 11025 samples, got %d", got)
	}
}

// TestTicksToSamples_ZeroTicks verifies that zero ticks always converts to
// zero samples. A simultaneous event (delta=0) must fire at sample 0.
func TestTicksToSamples_ZeroTicks(t *testing.T) {
	s := defaultScheduler()
	got := s.TicksToSamples(0)
	if got != 0 {
		t.Errorf("zero ticks: expected 0 samples, got %d", got)
	}
}

// TestTicksToSamples_ScalesWithBPM verifies that doubling BPM halves the
// sample count for the same tick value (faster tempo = shorter time per tick).
func TestTicksToSamples_ScalesWithBPM(t *testing.T) {
	s120 := NewScheduler(120, 480, DefaultADSR(), 0.3)
	s240 := NewScheduler(240, 480, DefaultADSR(), 0.3)

	samples120 := s120.TicksToSamples(480)
	samples240 := s240.TicksToSamples(480)

	if samples120 != samples240*2 {
		t.Errorf("doubling BPM should halve samples: 120BPM=%d, 240BPM=%d",
			samples120, samples240)
	}
}

// TestTicksToSamples_ScalesWithTicksPerQN verifies that doubling TicksPerQN
// halves the sample count per tick (higher resolution = fewer samples per tick).
func TestTicksToSamples_ScalesWithTicksPerQN(t *testing.T) {
	s480 := NewScheduler(120, 480, DefaultADSR(), 0.3)
	s960 := NewScheduler(120, 960, DefaultADSR(), 0.3)

	// At 960 TPQN, a quarter note is 960 ticks — same real time but more ticks
	samples480 := s480.TicksToSamples(480) // one quarter note at 480 TPQN
	samples960 := s960.TicksToSamples(960) // one quarter note at 960 TPQN

	if samples480 != samples960 {
		t.Errorf("one quarter note should be the same samples regardless of TPQN: 480TPQN=%d, 960TPQN=%d",
			samples480, samples960)
	}
}

// -----------------------------------------------------------------------------
// buildTimeline tests
// -----------------------------------------------------------------------------

// TestBuildTimeline_EmptyEvents verifies that an empty event slice produces
// an empty timeline without panicking.
func TestBuildTimeline_EmptyEvents(t *testing.T) {
	s := defaultScheduler()
	tl := s.buildTimeline([]midi.Event{})
	if len(tl) != 0 {
		t.Errorf("expected empty timeline, got %d events", len(tl))
	}
}

// TestBuildTimeline_NoteOnNoteOff verifies that a basic Note On followed by
// Note Off pair produces exactly two timeline entries with correct types,
// keys, and sample positions.
//
// Setup: NoteOn at delta=0, NoteOff at delta=480 (one quarter note at 120 BPM)
// Expected sample positions: 0, 22050
func TestBuildTimeline_NoteOnNoteOff(t *testing.T) {
	s := defaultScheduler()
	events := []midi.Event{
		midi.NoteOn(0, 0, 60, 100),
		midi.NoteOff(480, 0, 60),
	}

	tl := s.buildTimeline(events)

	if len(tl) != 2 {
		t.Fatalf("expected 2 timeline events, got %d", len(tl))
	}

	// First event: Note On at sample 0
	if tl[0].eventType != 0x90 {
		t.Errorf("event 0: expected Note On (0x90), got 0x%02X", tl[0].eventType)
	}
	if tl[0].samplePos != 0 {
		t.Errorf("event 0: expected samplePos=0, got %d", tl[0].samplePos)
	}
	if tl[0].key != 60 {
		t.Errorf("event 0: expected key=60, got %d", tl[0].key)
	}

	// Second event: Note Off at sample 22050
	if tl[1].eventType != 0x80 {
		t.Errorf("event 1: expected Note Off (0x80), got 0x%02X", tl[1].eventType)
	}
	if tl[1].samplePos != 22050 {
		t.Errorf("event 1: expected samplePos=22050, got %d", tl[1].samplePos)
	}
}

// TestBuildTimeline_DeltaAccumulation verifies that delta times accumulate
// correctly across a sequence of events.
//
// Three events with deltas 0, 240, 240:
// Absolute ticks: 0, 240, 480
// Expected sample positions: 0, 11025, 22050
func TestBuildTimeline_DeltaAccumulation(t *testing.T) {
	s := defaultScheduler()
	events := []midi.Event{
		midi.NoteOn(0, 0, 60, 100),
		midi.NoteOn(240, 0, 64, 100),
		midi.NoteOn(240, 0, 67, 100),
	}

	tl := s.buildTimeline(events)

	if len(tl) != 3 {
		t.Fatalf("expected 3 timeline events, got %d", len(tl))
	}

	expectedPositions := []int{0, 11025, 22050}
	for i, expected := range expectedPositions {
		if tl[i].samplePos != expected {
			t.Errorf("event %d: expected samplePos=%d, got %d", i, expected, tl[i].samplePos)
		}
	}
}

// TestBuildTimeline_NoteOnVelocityZeroTreatedAsNoteOff verifies that a
// Note On event with velocity=0 is converted to a Note Off in the timeline.
// This is standard MIDI running-status practice.
func TestBuildTimeline_NoteOnVelocityZeroTreatedAsNoteOff(t *testing.T) {
	s := defaultScheduler()
	// Manually construct a NoteOn with velocity=0
	events := []midi.Event{
		{Delta: 0, Data: []byte{0x90, 60, 0}}, // NoteOn vel=0
	}

	tl := s.buildTimeline(events)

	if len(tl) != 1 {
		t.Fatalf("expected 1 timeline event, got %d", len(tl))
	}
	if tl[0].eventType != 0x80 {
		t.Errorf("NoteOn vel=0 should become Note Off (0x80), got 0x%02X", tl[0].eventType)
	}
}

// TestBuildTimeline_MetaEventsSkipped verifies that meta-events (0xFF, such
// as the Tempo event prepended by Generate()) do not appear in the timeline.
// Only note events should be included.
func TestBuildTimeline_MetaEventsSkipped(t *testing.T) {
	s := defaultScheduler()
	events := []midi.Event{
		midi.Tempo(0, 120),         // meta-event — should be skipped
		midi.NoteOn(0, 0, 60, 100), // should appear
		midi.NoteOff(480, 0, 60),   // should appear
	}

	tl := s.buildTimeline(events)

	if len(tl) != 2 {
		t.Errorf("expected 2 timeline events (meta skipped), got %d", len(tl))
	}
}

// TestBuildTimeline_SamplePositionsNonDecreasing verifies that all sample
// positions in the timeline are non-decreasing (events are in chronological
// order). This is guaranteed by delta accumulation but we verify it explicitly
// since the renderer depends on this invariant.
func TestBuildTimeline_SamplePositionsNonDecreasing(t *testing.T) {
	s := defaultScheduler()
	events := []midi.Event{
		midi.NoteOn(0, 0, 60, 100),
		midi.NoteOn(120, 0, 64, 100),
		midi.NoteOff(240, 0, 60),
		midi.NoteOff(120, 0, 64),
	}

	tl := s.buildTimeline(events)

	for i := 1; i < len(tl); i++ {
		if tl[i].samplePos < tl[i-1].samplePos {
			t.Errorf("timeline not sorted: event %d samplePos=%d < event %d samplePos=%d",
				i, tl[i].samplePos, i-1, tl[i-1].samplePos)
		}
	}
}

// -----------------------------------------------------------------------------
// Render tests
// -----------------------------------------------------------------------------

// TestRender_EmptyEventsReturnsEmpty verifies that rendering an empty event
// slice returns an empty buffer without panicking.
func TestRender_EmptyEventsReturnsEmpty(t *testing.T) {
	s := defaultScheduler()
	buf := s.Render([]midi.Event{})
	if len(buf) != 0 {
		t.Errorf("expected empty buffer for empty events, got %d samples", len(buf))
	}
}

// TestRender_ProducesNonZeroOutput verifies that rendering a Note On/Off pair
// produces a buffer with at least some non-zero samples.
// A zero buffer would mean the oscillator or mixer is broken.
func TestRender_ProducesNonZeroOutput(t *testing.T) {
	s := defaultScheduler()
	events := []midi.Event{
		midi.Tempo(0, 120),
		midi.NoteOn(0, 0, 69, 100), // A4
		midi.NoteOff(480, 0, 69),
	}

	buf := s.Render(events)

	if len(buf) == 0 {
		t.Fatal("render produced empty buffer")
	}

	nonZero := 0
	for _, sample := range buf {
		if sample != 0.0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Error("render produced all-zero buffer — oscillator or mixer is silent")
	}
}

// TestRender_BufferLengthCoversLastEvent verifies that the output buffer is
// long enough to contain all note events plus the release tail.
// A buffer that ends at the last Note Off would cut off the release envelope.
func TestRender_BufferLengthCoversLastEvent(t *testing.T) {
	s := defaultScheduler()
	events := []midi.Event{
		midi.NoteOn(0, 0, 60, 100),
		midi.NoteOff(480, 0, 60), // note off at 22050 samples
	}

	buf := s.Render(events)

	noteOffSample := s.TicksToSamples(480)
	if len(buf) <= noteOffSample {
		t.Errorf("buffer length %d does not extend past last Note Off at sample %d",
			len(buf), noteOffSample)
	}
}

// TestRender_NoteOnFiresAtCorrectSample verifies that the oscillator begins
// producing non-zero output at or near the Note On sample position.
// Samples before the Note On should be silent (zero).
//
// We place a Note On at 240 ticks (11025 samples at 120BPM/480TPQN) and
// verify that samples 0–11023 are zero and sample 11025+ is non-zero.
func TestRender_NoteOnFiresAtCorrectSample(t *testing.T) {
	s := defaultScheduler()
	noteOnTicks := uint32(240)
	noteOnSample := s.TicksToSamples(noteOnTicks)

	events := []midi.Event{
		midi.NoteOn(noteOnTicks, 0, 69, 127),
		midi.NoteOff(480, 0, 69),
	}

	buf := s.Render(events)

	// All samples before the Note On should be exactly zero
	for i := 0; i < noteOnSample-1; i++ {
		if buf[i] != 0.0 {
			t.Errorf("sample %d (before Note On at %d): expected 0.0, got %f",
				i, noteOnSample, buf[i])
			break
		}
	}

	// At least one sample after the Note On should be non-zero.
	// We check a window starting slightly after to allow for the attack ramp.
	attackSamples := int(s.ADSR.AttackMs * SampleRate / 1000.0)
	checkFrom := noteOnSample + attackSamples
	if checkFrom >= len(buf) {
		t.Skip("buffer too short to check post-attack samples")
	}

	nonZero := false
	for i := checkFrom; i < checkFrom+100 && i < len(buf); i++ {
		if buf[i] != 0.0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Errorf("no non-zero samples found after Note On at sample %d (post-attack check from %d)",
			noteOnSample, checkFrom)
	}
}

// TestRender_PolyphonicMixing verifies that rendering two simultaneous notes
// produces a louder signal than either note alone. This confirms that the
// voice mixer is summing voices rather than replacing.
func TestRender_PolyphonicMixing(t *testing.T) {
	s := defaultScheduler()

	// Single note
	singleEvents := []midi.Event{
		midi.NoteOn(0, 0, 60, 100),
		midi.NoteOff(480, 0, 60),
	}
	singleBuf := s.Render(singleEvents)

	// Two simultaneous notes (chord)
	chordEvents := []midi.Event{
		midi.NoteOn(0, 0, 60, 100),
		midi.NoteOn(0, 0, 64, 100),
		midi.NoteOff(480, 0, 60),
		midi.NoteOff(0, 0, 64),
	}
	chordBuf := s.Render(chordEvents)

	// Compare RMS energy — chord should be louder
	singleRMS := bufRMS(singleBuf)
	chordRMS := bufRMS(chordBuf)

	if chordRMS <= singleRMS {
		t.Errorf("chord RMS (%.4f) should exceed single note RMS (%.4f)",
			chordRMS, singleRMS)
	}
}

// TestRender_SilenceAfterRelease verifies that the buffer is approximately
// silent well after the release stage has completed. Any non-silence after
// full release would indicate a voice is not being properly cleaned up.
func TestRender_SilenceAfterRelease(t *testing.T) {
	adsr := ADSRConfig{AttackMs: 1, DecayMs: 1, SustainLevel: 0.7, ReleaseMs: 10}
	s := NewScheduler(120, 480, adsr, 0.3)

	events := []midi.Event{
		midi.NoteOn(0, 0, 69, 100),
		midi.NoteOff(96, 0, 69), // short note
	}

	buf := s.Render(events)

	// Release completes within ~441 samples (10ms * 44.1)
	// Check that the last 1000 samples are effectively silent
	checkFrom := len(buf) - 1000
	if checkFrom < 0 {
		t.Skip("buffer too short to check tail silence")
	}

	for i := checkFrom; i < len(buf); i++ {
		if math.Abs(float64(buf[i])) > 0.001 {
			t.Errorf("sample %d: expected silence after release, got %f", i, buf[i])
			break
		}
	}
}

// -----------------------------------------------------------------------------
// RenderStereo tests
// -----------------------------------------------------------------------------

// TestRenderStereo_LengthDoubled verifies the stereo buffer is exactly twice
// the length of the mono input.
func TestRenderStereo_LengthDoubled(t *testing.T) {
	mono := []float32{0.1, 0.2, 0.3, 0.4}
	stereo := RenderStereo(mono)
	if len(stereo) != len(mono)*2 {
		t.Errorf("expected stereo length %d, got %d", len(mono)*2, len(stereo))
	}
}

// TestRenderStereo_ChannelsIdentical verifies that left and right channels
// carry the same signal (equal amplitude mono-to-stereo expansion).
func TestRenderStereo_ChannelsIdentical(t *testing.T) {
	mono := []float32{0.1, 0.5, -0.3, 0.8}
	stereo := RenderStereo(mono)

	for i, s := range mono {
		left := stereo[i*2]
		right := stereo[i*2+1]
		if left != s {
			t.Errorf("sample %d left channel: expected %f, got %f", i, s, left)
		}
		if right != s {
			t.Errorf("sample %d right channel: expected %f, got %f", i, s, right)
		}
	}
}

// TestRenderStereo_EmptyInput verifies that an empty mono slice produces an
// empty stereo slice without panicking.
func TestRenderStereo_EmptyInput(t *testing.T) {
	stereo := RenderStereo([]float32{})
	if len(stereo) != 0 {
		t.Errorf("expected empty stereo output, got %d samples", len(stereo))
	}
}

// TestRenderStereo_Interleaving verifies the exact byte layout of the stereo
// output. oto expects L R L R L R... interleaving.
//
//	mono[0]=0.1 → stereo[0]=0.1 (L), stereo[1]=0.1 (R)
//	mono[1]=0.9 → stereo[2]=0.9 (L), stereo[3]=0.9 (R)
func TestRenderStereo_Interleaving(t *testing.T) {
	mono := []float32{0.1, 0.9}
	stereo := RenderStereo(mono)

	expected := []float32{0.1, 0.1, 0.9, 0.9}
	for i, e := range expected {
		if stereo[i] != e {
			t.Errorf("stereo[%d]: expected %f, got %f", i, e, stereo[i])
		}
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// bufRMS computes the root mean square amplitude of a float32 buffer.
func bufRMS(buf []float32) float64 {
	if len(buf) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range buf {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(buf)))
}
