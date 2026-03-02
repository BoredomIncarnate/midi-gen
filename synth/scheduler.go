package synth

import (
	"midi-gen/midi"
)

// Scheduler converts a sequence of midi.Event values into a flat []float32
// PCM buffer suitable for playback through an audio output device.
//
// The conversion process has two phases:
//
//  1. Event parsing: walk the event list, converting delta times to absolute
//     sample positions. Build a timeline of NoteOn and NoteOff actions at
//     specific sample offsets.
//
//  2. Sample rendering: walk sample-by-sample through the timeline. At each
//     sample, activate new voices, release finishing voices, and mix all
//     active voices into the output buffer.
//
// The output buffer is mono float32 at SampleRate (44100 Hz). Conversion to
// stereo or other formats is handled by output.go before passing to oto.
//
// Memory layout:
//
//	BPM         int          tempo, used to convert ticks → seconds → samples
//	TicksPerQN  int          MIDI timing resolution (e.g. 480)
//	ADSR        ADSRConfig   envelope shape applied to every voice
//	MasterGain  float64      global amplitude scale, prevents clipping on summed voices
//	                         Typical value: 0.3 (safe for up to ~10 simultaneous voices)
type Scheduler struct {
	BPM        int
	TicksPerQN int
	ADSR       ADSRConfig
	MasterGain float64
}

// NewScheduler creates a Scheduler with the given tempo and timing resolution.
//
// Parameters:
//
//	bpm         — beats per minute (matches the GeneratorConfig BPM)
//	ticksPerQN  — ticks per quarter note (matches the MIDI file TicksPerQN)
//	adsr        — envelope configuration for all voices
//	masterGain  — global volume scale (0.3 is a safe default for polyphony)
func NewScheduler(bpm, ticksPerQN int, adsr ADSRConfig, masterGain float64) *Scheduler {
	return &Scheduler{
		BPM:        bpm,
		TicksPerQN: ticksPerQN,
		ADSR:       adsr,
		MasterGain: masterGain,
	}
}

// TicksToSamples converts a MIDI tick count to a PCM sample count.
//
// Conversion formula:
//
//	secondsPerBeat = 60.0 / BPM
//	beatsPerTick   = 1.0 / TicksPerQN
//	samples        = ticks * beatsPerTick * secondsPerBeat * SampleRate
//
// Example at BPM=120, TicksPerQN=480, SampleRate=44100:
//
//	480 ticks (one quarter note) → 0.5 seconds → 22050 samples
//	240 ticks (one eighth note)  → 0.25 seconds → 11025 samples
//
// The result is truncated (not rounded) to an integer sample count.
// Fractional sample positions are not supported — all events snap to the
// nearest sample boundary, which introduces at most ~22 microseconds of
// timing error at 44100 Hz. This is well below the threshold of human
// perception (~1 millisecond).
func (s *Scheduler) TicksToSamples(ticks uint32) int {
	secondsPerBeat := 60.0 / float64(s.BPM)
	beatsPerTick := 1.0 / float64(s.TicksPerQN)
	return int(float64(ticks) * beatsPerTick * secondsPerBeat * SampleRate)
}

// scheduledEvent represents a single MIDI action at a specific sample position.
// It is an internal type used to build the rendering timeline.
//
// Rather than walking MIDI events sample-by-sample during rendering (which
// would require re-scanning the event list on every sample), we pre-process
// all events into a flat slice of scheduledEvents sorted by sample position.
// The renderer then walks this slice linearly alongside the sample counter.
//
// Memory layout:
//
//	samplePos  int    absolute sample position when this event fires
//	eventType  byte   0x90 = Note On, 0x80 = Note Off (upper nibble of status byte)
//	key        byte   MIDI note number (0–127)
//	velocity   byte   MIDI velocity (0–127), only meaningful for Note On events
type scheduledEvent struct {
	samplePos int
	eventType byte
	key       byte
	velocity  byte
}

// buildTimeline converts a slice of midi.Event (delta-time encoded) into a
// slice of scheduledEvent (absolute sample positions).
//
// Delta times are cumulative — each event's delta is the number of ticks
// since the previous event, not since time zero. We accumulate them into
// an absolute tick counter, then convert each absolute tick position to
// a sample position via TicksToSamples.
//
// Only Note On (0x90) and Note Off (0x80) events are included in the timeline.
// Meta-events (0xFF), tempo events, and other non-note events are skipped —
// the scheduler only needs to know when notes start and stop.
//
// Note On events with velocity=0 are treated as Note Off events. This is
// standard MIDI practice — many sequencers use NoteOn(vel=0) as a compact
// way to encode Note Off using running status.
//
// Parameters:
//
//	events — the raw MIDI event slice from a midi.Track
//
// Returns:
//
//	[]scheduledEvent sorted by samplePos ascending (guaranteed by the
//	linear delta accumulation — absolute ticks are always non-decreasing)
func (s *Scheduler) buildTimeline(events []midi.Event) []scheduledEvent {
	timeline := make([]scheduledEvent, 0, len(events))

	// absoluteTicks accumulates delta times to produce absolute tick positions.
	// Starts at 0 (time zero = start of playback).
	absoluteTicks := uint32(0)

	for _, evt := range events {
		// Accumulate delta: absoluteTicks now points to this event's tick position
		absoluteTicks += evt.Delta

		// Skip events with fewer than 3 bytes — not a channel message
		if len(evt.Data) < 3 {
			continue
		}

		// Extract the status byte upper nibble (event type) and lower nibble (channel)
		// We ignore channel — all voices mix into the same mono output buffer.
		statusType := evt.Data[0] & 0xF0
		key := evt.Data[1]
		velocity := evt.Data[2]

		switch statusType {
		case 0x90: // Note On
			if velocity == 0 {
				// NoteOn with velocity=0 is a Note Off in disguise (running status idiom)
				timeline = append(timeline, scheduledEvent{
					samplePos: s.TicksToSamples(absoluteTicks),
					eventType: 0x80,
					key:       key,
					velocity:  0,
				})
			} else {
				timeline = append(timeline, scheduledEvent{
					samplePos: s.TicksToSamples(absoluteTicks),
					eventType: 0x90,
					key:       key,
					velocity:  velocity,
				})
			}

		case 0x80: // Note Off
			timeline = append(timeline, scheduledEvent{
				samplePos: s.TicksToSamples(absoluteTicks),
				eventType: 0x80,
				key:       key,
				velocity:  velocity,
			})
		}
		// All other event types (0xFF meta, 0xC0 program change, etc.) are ignored
	}

	return timeline
}

// Render converts a slice of MIDI events into a mono float32 PCM buffer.
//
// The output buffer length is determined by the last event's sample position
// plus a release tail. The release tail adds enough silence after the last
// Note Off for all active voices to complete their release stages without
// being cut off. It is computed as the longest possible release time across
// all ADSR configurations, converted to samples:
//
//	releaseTailSamples = ADSR.ReleaseMs * SampleRate / 1000
//
// Rendering algorithm:
//
//  1. Build the event timeline (absolute sample positions)
//  2. Compute total buffer length = last event sample + release tail
//  3. Allocate the output buffer (zeroed)
//  4. Walk sample by sample:
//     a. Check if any timeline events fire at the current sample
//     b. For Note On: create a new Voice and add to the active list
//     c. For Note Off: call Release() on the matching active Voice
//     d. Mix all active (non-done) Voices into the buffer via RenderSamples
//     e. Remove done Voices from the active list
//
// Voice lookup for Note Off uses a linear scan of the active voice list.
// Since polyphony is typically low (2–8 voices), this is more efficient
// than a map lookup due to cache locality.
//
// The buffer is mono — a single float32 per sample. Stereo interleaving
// is applied in output.go before the buffer is passed to oto.
func (s *Scheduler) Render(events []midi.Event) []float32 {
	// Phase 1: build the event timeline
	timeline := s.buildTimeline(events)
	if len(timeline) == 0 {
		return []float32{}
	}

	// Phase 2: compute total buffer length
	// Last event's sample position + release tail
	lastSample := timeline[len(timeline)-1].samplePos
	releaseTailSamples := int(s.ADSR.ReleaseMs * SampleRate / 1000.0)

	// Add a generous tail multiplier so long release envelopes don't get cut off.
	// 4x the release time covers even very long reverberant tails.
	totalSamples := lastSample + releaseTailSamples*4
	if totalSamples <= 0 {
		return []float32{}
	}

	// Phase 3: allocate output buffer (zero-initialised by Go)
	buf := make([]float32, totalSamples)

	// activeVoices holds all currently sounding voices.
	// We use a slice rather than a map because polyphony is low and
	// slice iteration has better cache performance than map lookups.
	activeVoices := make([]*Voice, 0, 16)

	// timelineIdx tracks how far into the timeline we have consumed.
	// Events are processed in order — once an event's samplePos is passed,
	// timelineIdx advances past it and it is never re-examined.
	timelineIdx := 0

	// Phase 4: sample-by-sample rendering loop
	for sample := 0; sample < totalSamples; sample++ {

		// --- 4a: process all timeline events that fire at this sample ---
		// Multiple events may share the same samplePos (e.g. a chord burst
		// of simultaneous Note Ons). We consume all of them before rendering.
		for timelineIdx < len(timeline) && timeline[timelineIdx].samplePos <= sample {
			evt := timeline[timelineIdx]
			timelineIdx++

			switch evt.eventType {

			case 0x90: // Note On — create a new Voice and add to active list
				voice := NewVoice(evt.key, evt.velocity, s.ADSR)
				activeVoices = append(activeVoices, voice)

			case 0x80: // Note Off — find the matching Voice and release it
				// Linear scan: find the first active, non-releasing voice with matching key.
				// "First" matches the oldest Note On for that key, which is correct
				// behaviour when the same note is triggered multiple times (polyphonic
				// keyboards, or overlapping notes in a generated progression).
				for _, v := range activeVoices {
					if v.Key == evt.key && !v.Releasing && !v.IsDone() {
						v.Release()
						break // release only the oldest matching voice
					}
				}
			}
		}

		// --- 4b: mix all active voices into buf[sample] ---
		// We render one sample at a time from each voice. RenderSamples with
		// n=1 processes exactly one sample and updates the voice's phase and
		// envelope state. The single-element slice view avoids allocations.
		singleSample := buf[sample : sample+1]
		for _, v := range activeVoices {
			if !v.IsDone() {
				v.RenderSamples(singleSample, 1, s.MasterGain)
			}
		}

		// --- 4c: remove done voices ---
		// We rebuild the activeVoices slice in-place, keeping only voices
		// that are not yet done. This avoids allocating a new slice on most
		// iterations — only iterations where at least one voice finishes
		// cause the slice to shrink.
		//
		// The two-pointer pattern (i for read, write for write position)
		// compacts the slice without shifting elements unnecessarily:
		//
		//   before: [v1(done), v2(active), v3(done), v4(active)]
		//   after:  [v2, v4]
		write := 0
		for _, v := range activeVoices {
			if !v.IsDone() {
				activeVoices[write] = v
				write++
			}
		}
		// Zero out the tail to allow GC to collect done Voice objects
		for i := write; i < len(activeVoices); i++ {
			activeVoices[i] = nil
		}
		activeVoices = activeVoices[:write]
	}

	return buf
}

// RenderStereo converts a mono PCM buffer into a stereo interleaved buffer.
//
// Stereo interleaving means samples alternate between left and right channels:
//
//	mono:   [s0,  s1,  s2,  s3, ...]
//	stereo: [s0L, s0R, s1L, s1R, s2L, s2R, ...]
//
// For this implementation, both channels carry identical signal (no panning).
// The output buffer is twice the length of the input.
//
// oto on macOS expects stereo float32 PCM by default. Passing mono to oto
// would play at half speed because it interprets every two float32 values
// as one stereo frame.
//
// Parameters:
//
//	mono — float32 slice of mono samples (length N)
//
// Returns:
//
//	float32 slice of stereo interleaved samples (length 2N)
func RenderStereo(mono []float32) []float32 {
	// Stereo output is exactly 2x the mono sample count.
	// Each mono sample expands to two identical samples (L and R channels).
	stereo := make([]float32, len(mono)*2)
	for i, s := range mono {
		// Left channel: even indices  (0, 2, 4, ...)
		stereo[i*2] = s
		// Right channel: odd indices  (1, 3, 5, ...)
		stereo[i*2+1] = s
	}
	return stereo
}
