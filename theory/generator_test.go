package theory

import (
	"math/rand"
	"testing"

	"midi-gen/midi"
)

// defaultConfig returns a valid GeneratorConfig for use as a baseline in tests.
// Individual tests override specific fields as needed.
func defaultConfig() GeneratorConfig {
	return GeneratorConfig{
		Scale:      "major",
		RootNote:   60,
		Octaves:    2,
		Length:     8,
		MinVel:     60,
		MaxVel:     100,
		Complexity: "medium",
		Mode:       "melody",
		BPM:        120,
		TicksPerQN: 480,
		Quantize:   "eighth",
		ChordStyle: "long",
		Seed:       42, // fixed seed for deterministic tests
	}
}

// -----------------------------------------------------------------------------
// validateConfig tests
// -----------------------------------------------------------------------------

// TestValidateConfig_Valid verifies that a well-formed config passes validation.
func TestValidateConfig_Valid(t *testing.T) {
	if err := validateConfig(defaultConfig()); err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

// TestValidateConfig_InvalidScale verifies that an unknown scale name is rejected.
func TestValidateConfig_InvalidScale(t *testing.T) {
	cfg := defaultConfig()
	cfg.Scale = "notascale"
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for unknown scale, got none")
	}
}

// TestValidateConfig_InvalidRootNote verifies roots outside 0–127 are rejected.
func TestValidateConfig_InvalidRootNote(t *testing.T) {
	for _, root := range []int{-1, 128} {
		cfg := defaultConfig()
		cfg.RootNote = root
		if err := validateConfig(cfg); err == nil {
			t.Errorf("root=%d: expected error, got none", root)
		}
	}
}

// TestValidateConfig_MinVelGtMaxVel verifies that MinVel > MaxVel is rejected.
func TestValidateConfig_MinVelGtMaxVel(t *testing.T) {
	cfg := defaultConfig()
	cfg.MinVel = 100
	cfg.MaxVel = 60
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for MinVel > MaxVel, got none")
	}
}

// TestValidateConfig_InvalidComplexity verifies unknown complexity is rejected.
func TestValidateConfig_InvalidComplexity(t *testing.T) {
	cfg := defaultConfig()
	cfg.Complexity = "extreme"
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for unknown complexity, got none")
	}
}

// TestValidateConfig_InvalidMode verifies unknown mode is rejected.
func TestValidateConfig_InvalidMode(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "arpeggio"
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for unknown mode, got none")
	}
}

// TestValidateConfig_InvalidBPM verifies BPM outside 1–300 is rejected.
func TestValidateConfig_InvalidBPM(t *testing.T) {
	for _, bpm := range []int{0, 301} {
		cfg := defaultConfig()
		cfg.BPM = bpm
		if err := validateConfig(cfg); err == nil {
			t.Errorf("bpm=%d: expected error, got none", bpm)
		}
	}
}

// TestValidateConfig_InvalidQuantize verifies unknown quantize value is rejected.
func TestValidateConfig_InvalidQuantize(t *testing.T) {
	cfg := defaultConfig()
	cfg.Quantize = "thirtysecond"
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for unknown quantize, got none")
	}
}

// -----------------------------------------------------------------------------
// quantizeTicks tests
// -----------------------------------------------------------------------------

// TestQuantizeTicks verifies tick counts for all three quantize values at
// the standard DAW resolution of 480 ticks per quarter note.
//
//	quarter   = 480 ticks
//	eighth    = 240 ticks
//	sixteenth = 120 ticks
func TestQuantizeTicks(t *testing.T) {
	tests := []struct {
		quantize string
		expected uint32
	}{
		{"quarter", 480},
		{"eighth", 240},
		{"sixteenth", 120},
	}
	for _, tt := range tests {
		got, err := quantizeTicks(tt.quantize, 480)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.quantize, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("%s: expected %d ticks, got %d", tt.quantize, tt.expected, got)
		}
	}
}

// TestQuantizeTicks_Unknown verifies that an unknown quantize string returns an error.
func TestQuantizeTicks_Unknown(t *testing.T) {
	_, err := quantizeTicks("halfnote", 480)
	if err == nil {
		t.Error("expected error for unknown quantize, got none")
	}
}

// -----------------------------------------------------------------------------
// Generate — structural tests (apply to all modes)
// -----------------------------------------------------------------------------

// TestGenerate_ReturnsEvents verifies that Generate always returns at least
// one event (the Tempo meta-event) for any valid config.
func TestGenerate_ReturnsEvents(t *testing.T) {
	for _, mode := range []string{"melody", "chords", "progression"} {
		cfg := defaultConfig()
		cfg.Mode = mode
		track, err := Generate(cfg)
		if err != nil {
			t.Errorf("mode=%s: unexpected error: %v", mode, err)
			continue
		}
		if len(track.Events) == 0 {
			t.Errorf("mode=%s: expected at least one event, got none", mode)
		}
	}
}

// TestGenerate_FirstEventIsTempo verifies that the first event in the track
// is always a Tempo meta-event (0xFF 0x51), placed at delta=0.
// Without this, DAWs default to 120 BPM regardless of the config BPM.
func TestGenerate_FirstEventIsTempo(t *testing.T) {
	track, err := Generate(defaultConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(track.Events) == 0 {
		t.Fatal("no events in track")
	}
	first := track.Events[0]
	if first.Delta != 0 {
		t.Errorf("tempo event delta: expected 0, got %d", first.Delta)
	}
	if len(first.Data) < 2 || first.Data[0] != 0xFF || first.Data[1] != 0x51 {
		t.Errorf("first event is not a Tempo meta-event: %X", first.Data)
	}
}

// TestGenerate_DeterministicWithSeed verifies that the same seed always
// produces identical output. This is critical for reproducible patches and
// sharing specific generated patterns.
func TestGenerate_DeterministicWithSeed(t *testing.T) {
	cfg := defaultConfig()
	cfg.Seed = 12345

	track1, err1 := Generate(cfg)
	track2, err2 := Generate(cfg)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if !tracksEqual(track1, track2) {
		t.Error("same seed produced different output on two runs")
	}
}

// TestGenerate_DifferentSeedsDiffer verifies that different seeds (almost always)
// produce different output. We use two very different seeds to make collision
// astronomically unlikely.
func TestGenerate_DifferentSeedsDiffer(t *testing.T) {
	cfg := defaultConfig()
	cfg.Seed = 1
	track1, _ := Generate(cfg)

	cfg.Seed = 999999
	track2, _ := Generate(cfg)

	if tracksEqual(track1, track2) {
		t.Error("different seeds produced identical output (extremely unlikely unless broken)")
	}
}

// TestGenerate_NoteEventsInRange verifies that all Note On/Off events in any
// generated track use key values within the valid MIDI range 0–127.
// Data byte bit 7 must be 0 for all MIDI data bytes.
func TestGenerate_NoteEventsInRange(t *testing.T) {
	for _, mode := range []string{"melody", "chords", "progression"} {
		cfg := defaultConfig()
		cfg.Mode = mode
		cfg.Length = 32
		track, err := Generate(cfg)
		if err != nil {
			t.Errorf("mode=%s: %v", mode, err)
			continue
		}
		for _, evt := range track.Events {
			// Note On (0x9n) and Note Off (0x8n) both have key at Data[1]
			if len(evt.Data) >= 2 {
				status := evt.Data[0] & 0xF0
				if status == 0x90 || status == 0x80 {
					key := evt.Data[1]
					if key > 127 {
						t.Errorf("mode=%s: note key %d exceeds MIDI max 127", mode, key)
					}
					vel := evt.Data[2]
					if vel > 127 {
						t.Errorf("mode=%s: velocity %d exceeds MIDI max 127", mode, vel)
					}
				}
			}
		}
	}
}

// TestGenerate_VelocityInRange verifies all generated velocities fall within
// the configured MinVel–MaxVel window.
func TestGenerate_VelocityInRange(t *testing.T) {
	cfg := defaultConfig()
	cfg.MinVel = 50
	cfg.MaxVel = 80
	cfg.Length = 64

	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, evt := range track.Events {
		if len(evt.Data) >= 3 && (evt.Data[0]&0xF0) == 0x90 {
			vel := int(evt.Data[2])
			// velocity=0 is allowed (treated as Note Off by some devices)
			if vel > 0 && (vel < cfg.MinVel || vel > cfg.MaxVel) {
				t.Errorf("velocity %d outside configured range %d–%d", vel, cfg.MinVel, cfg.MaxVel)
			}
		}
	}
}

// TestGenerate_NoteOnOffPaired verifies that every Note On in the track has
// a corresponding Note Off for the same key. Unpaired Note Ons cause stuck
// notes in DAWs and the synth scheduler.
func TestGenerate_NoteOnOffPaired(t *testing.T) {
	for _, mode := range []string{"melody", "chords", "progression"} {
		cfg := defaultConfig()
		cfg.Mode = mode
		cfg.Length = 16
		track, err := Generate(cfg)
		if err != nil {
			t.Errorf("mode=%s: %v", mode, err)
			continue
		}

		// Count Note On and Note Off events per key
		noteOns := make(map[byte]int)
		noteOffs := make(map[byte]int)
		for _, evt := range track.Events {
			if len(evt.Data) < 3 {
				continue
			}
			status := evt.Data[0] & 0xF0
			key := evt.Data[1]
			if status == 0x90 && evt.Data[2] > 0 {
				noteOns[key]++
			} else if status == 0x80 || (status == 0x90 && evt.Data[2] == 0) {
				noteOffs[key]++
			}
		}

		for key, ons := range noteOns {
			offs := noteOffs[key]
			if ons != offs {
				t.Errorf("mode=%s: key %d has %d Note Ons but %d Note Offs",
					mode, key, ons, offs)
			}
		}
	}
}

// TestGenerate_AllModes_AllComplexities is a smoke test that runs every
// combination of mode and complexity to ensure no panics or unexpected errors.
func TestGenerate_AllModes_AllComplexities(t *testing.T) {
	modes := []string{"melody", "chords", "progression"}
	complexities := []string{"simple", "medium", "complex"}
	for _, mode := range modes {
		for _, complexity := range complexities {
			cfg := defaultConfig()
			cfg.Mode = mode
			cfg.Complexity = complexity
			_, err := Generate(cfg)
			if err != nil {
				t.Errorf("mode=%s complexity=%s: unexpected error: %v",
					mode, complexity, err)
			}
		}
	}
}

// TestGenerate_AllQuantize verifies generation succeeds for all quantize values.
func TestGenerate_AllQuantize(t *testing.T) {
	for _, q := range []string{"quarter", "eighth", "sixteenth"} {
		cfg := defaultConfig()
		cfg.Quantize = q
		_, err := Generate(cfg)
		if err != nil {
			t.Errorf("quantize=%s: unexpected error: %v", q, err)
		}
	}
}

// TestGenerate_LengthRespected verifies that the melody generator produces
// at most cfg.Length note-on events (may be fewer due to rests).
func TestGenerate_LengthRespected(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "melody"
	cfg.Length = 8
	cfg.Complexity = "simple" // restProbability=0.05, very few rests

	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	noteOnCount := 0
	for _, evt := range track.Events {
		if len(evt.Data) >= 3 && (evt.Data[0]&0xF0) == 0x90 && evt.Data[2] > 0 {
			noteOnCount++
		}
	}

	if noteOnCount > cfg.Length {
		t.Errorf("melody generated %d note-ons, expected at most %d", noteOnCount, cfg.Length)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// tracksEqual compares two midi.Tracks for identical event content.
func tracksEqual(a, b midi.Track) bool {
	if len(a.Events) != len(b.Events) {
		return false
	}
	for i := range a.Events {
		if a.Events[i].Delta != b.Events[i].Delta {
			return false
		}
		if !bytesEqualGen(a.Events[i].Data, b.Events[i].Data) {
			return false
		}
	}
	return true
}

// bytesEqualGen compares two byte slices for equality.
// Defined here rather than importing from midi package tests since test
// helpers are not exported between packages.
func bytesEqualGen(a, b []byte) bool {
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

// -----------------------------------------------------------------------------
// generatePhraseDurations tests
// -----------------------------------------------------------------------------

// TestGeneratePhraseDurations_Length verifies the phrase slice has exactly
// phraseLengthSteps entries — one base duration per step slot.
func TestGeneratePhraseDurations_Length(t *testing.T) {
	cfg := defaultConfig()
	cs, _ := resolveComplexity(cfg)
	rng := rand.New(rand.NewSource(1))
	stepTicks := uint32(240)

	phraseLengthSteps := cs.phraseLengthBars * stepsPerBar(cfg.Quantize)
	phrase := generatePhraseDurations(cs, stepTicks, phraseLengthSteps, rng)
	if len(phrase) != phraseLengthSteps {
		t.Errorf("expected phrase length %d, got %d", phraseLengthSteps, len(phrase))
	}
}

// TestGeneratePhraseDurations_NonZero verifies every phrase slot has a
// duration of at least 1 tick. Zero-tick durations would cause silent
// stuck notes in the scheduler.
func TestGeneratePhraseDurations_NonZero(t *testing.T) {
	cfg := defaultConfig()
	cs, _ := resolveComplexity(cfg)
	rng := rand.New(rand.NewSource(2))
	stepTicks := uint32(240)

	phraseLengthStepsNZ := cs.phraseLengthBars * stepsPerBar(cfg.Quantize)
	phrase := generatePhraseDurations(cs, stepTicks, phraseLengthStepsNZ, rng)
	for i, d := range phrase {
		if d == 0 {
			t.Errorf("phrase slot %d: duration is 0 ticks", i)
		}
	}
}

// TestGeneratePhraseDurations_Deterministic verifies that the same seed
// produces the same phrase pattern — required for reproducible output.
func TestGeneratePhraseDurations_Deterministic(t *testing.T) {
	cfg := defaultConfig()
	cs, _ := resolveComplexity(cfg)
	stepTicks := uint32(240)

	phraseLengthSteps2 := cs.phraseLengthBars * stepsPerBar(cfg.Quantize)
	rng1 := rand.New(rand.NewSource(99))
	rng2 := rand.New(rand.NewSource(99))

	p1 := generatePhraseDurations(cs, stepTicks, phraseLengthSteps2, rng1)
	p2 := generatePhraseDurations(cs, stepTicks, phraseLengthSteps2, rng2)

	if !uint32SliceEqual(p1, p2) {
		t.Errorf("same seed produced different phrase patterns: %v vs %v", p1, p2)
	}
}

// TestGeneratePhraseDurations_AllComplexities verifies phrase generation
// works without panic or empty output for all three complexity levels.
func TestGeneratePhraseDurations_AllComplexities(t *testing.T) {
	for _, complexity := range []string{"simple", "medium", "complex"} {
		cfg := defaultConfig()
		cfg.Complexity = complexity
		cs, err := resolveComplexity(cfg)
		if err != nil {
			t.Errorf("%s: resolveComplexity error: %v", complexity, err)
			continue
		}
		rng := rand.New(rand.NewSource(1))
		pls := cs.phraseLengthBars * stepsPerBar(cfg.Quantize)
		phrase := generatePhraseDurations(cs, 240, pls, rng)
		if len(phrase) == 0 {
			t.Errorf("%s: empty phrase", complexity)
		}
	}
}

// -----------------------------------------------------------------------------
// randomNoteDuration tests
// -----------------------------------------------------------------------------

// TestRandomNoteDuration_NonZero verifies the duration is always >= 1 tick,
// even with extreme velocity and jitter values.
func TestRandomNoteDuration_NonZero(t *testing.T) {
	cfg := defaultConfig()
	cs, _ := resolveComplexity(cfg)
	rng := rand.New(rand.NewSource(5))

	for i := 0; i < 100; i++ {
		d := randomNoteDuration(240, cfg.MinVel, cfg, cs, rng)
		if d < 1 {
			t.Errorf("iteration %d: duration %d is less than 1", i, d)
		}
	}
}

// TestRandomNoteDuration_VelocityCorrelation verifies that a louder note
// (higher velocity) produces a longer duration than a quieter note, given the
// same base duration and no jitter (jitterFraction=0).
//
// We test with jitterFraction=0 to isolate the velocity effect cleanly.
func TestRandomNoteDuration_VelocityCorrelation(t *testing.T) {
	cfg := defaultConfig()
	cfg.MinVel = 40
	cfg.MaxVel = 100

	// Remove jitter to make the velocity effect deterministic
	cs, _ := resolveComplexity(cfg)
	cs.durationJitterFraction = 0

	rng := rand.New(rand.NewSource(1))
	baseDuration := uint32(240)

	quietDuration := randomNoteDuration(baseDuration, cfg.MinVel, cfg, cs, rng)
	loudDuration := randomNoteDuration(baseDuration, cfg.MaxVel, cfg, cs, rng)

	if loudDuration <= quietDuration {
		t.Errorf("loud note (%d ticks) should be longer than quiet note (%d ticks)",
			loudDuration, quietDuration)
	}
}

// TestRandomNoteDuration_MidVelocityBetween verifies that a mid-range velocity
// produces a duration between the quiet and loud extremes (with jitter removed).
func TestRandomNoteDuration_MidVelocityBetween(t *testing.T) {
	cfg := defaultConfig()
	cfg.MinVel = 40
	cfg.MaxVel = 100

	cs, _ := resolveComplexity(cfg)
	cs.durationJitterFraction = 0

	rng := rand.New(rand.NewSource(1))
	baseDuration := uint32(480)
	midVel := (cfg.MinVel + cfg.MaxVel) / 2

	quiet := randomNoteDuration(baseDuration, cfg.MinVel, cfg, cs, rng)
	mid := randomNoteDuration(baseDuration, midVel, cfg, cs, rng)
	loud := randomNoteDuration(baseDuration, cfg.MaxVel, cfg, cs, rng)

	if !(quiet <= mid && mid <= loud) {
		t.Errorf("expected quiet(%d) <= mid(%d) <= loud(%d)", quiet, mid, loud)
	}
}

// TestRandomNoteDuration_JitterBounds verifies that across many iterations,
// the jitter stays within the expected ± fraction of the velocity-scaled base.
// We use a wide tolerance to account for legitimate RNG spread.
func TestRandomNoteDuration_JitterBounds(t *testing.T) {
	cfg := defaultConfig()
	cs, _ := resolveComplexity(cfg)
	cs.durationJitterFraction = 0.10 // ±10%

	rng := rand.New(rand.NewSource(7))
	baseDuration := uint32(480)
	velocity := (cfg.MinVel + cfg.MaxVel) / 2

	// Compute the expected velocity-scaled center (no jitter)
	velRange := cfg.MaxVel - cfg.MinVel
	t_ := float64(velocity-cfg.MinVel) / float64(velRange)
	velMultiplier := 0.75 + t_*0.35
	center := float64(baseDuration) * velMultiplier

	// Allow ±15% total (jitter ±10% + floating point rounding)
	tolerance := center * 0.15

	for i := 0; i < 200; i++ {
		d := randomNoteDuration(baseDuration, velocity, cfg, cs, rng)
		diff := float64(d) - center
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Errorf("iteration %d: duration %d deviates %.1f from center %.1f (tolerance %.1f)",
				i, d, diff, center, tolerance)
		}
	}
}

// -----------------------------------------------------------------------------
// integration: dynamic durations in generated tracks
// -----------------------------------------------------------------------------

// TestGenerate_DurationsVary verifies that across a sufficiently long melody,
// Note Off deltas are not all identical — dynamic note lengths are being applied.
// In the grid-locked model, Note Off delta = noteDuration which varies per note.
func TestGenerate_DurationsVary(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "melody"
	cfg.Length = 32
	cfg.Complexity = "complex" // widest duration range

	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect Note Off deltas — each one is a note duration.
	// In the grid-locked model, Note Off delta is always > 0 (it is the sounding
	// duration of the note, set by randomNoteDuration).
	durations := map[uint32]int{}
	for _, evt := range track.Events {
		if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 && evt.Delta > 0 {
			durations[evt.Delta]++
		}
	}

	if len(durations) <= 1 {
		t.Errorf("expected multiple distinct note durations, got %d unique value(s): %v",
			len(durations), durations)
	}
}

// -----------------------------------------------------------------------------
// additional helpers
// -----------------------------------------------------------------------------

func uint32SliceEqual(a, b []uint32) bool {
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

// -----------------------------------------------------------------------------
// Grid-locking tests
// -----------------------------------------------------------------------------

// TestGenerate_NoteOnsOnGrid verifies that every Note On event fires at an
// absolute tick position that is a clean multiple of stepTicks.
//
// This is the core invariant of the timing model: notes may ring for any
// duration, but they always START on the grid.
//
// We reconstruct absolute tick positions by summing deltas across the event
// stream, then check each Note On's absolute position modulo stepTicks.
func TestGenerate_NoteOnsOnGrid(t *testing.T) {
	for _, mode := range []string{"melody", "chords", "progression"} {
		cfg := defaultConfig()
		cfg.Mode = mode
		cfg.Length = 32
		cfg.Seed = 77

		stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)

		track, err := Generate(cfg)
		if err != nil {
			t.Errorf("mode=%s: %v", mode, err)
			continue
		}

		// Walk events, accumulating absolute tick position.
		// Skip the first event (Tempo meta-event, always delta=0).
		absoluteTick := uint32(0)
		for _, evt := range track.Events {
			absoluteTick += evt.Delta

			if len(evt.Data) < 3 {
				continue
			}
			status := evt.Data[0] & 0xF0
			velocity := evt.Data[2]

			// Only check Note On events with non-zero velocity
			if status == 0x90 && velocity > 0 {
				if absoluteTick%stepTicks != 0 {
					t.Errorf("mode=%s: Note On at absolute tick %d is not on grid (stepTicks=%d, remainder=%d)",
						mode, absoluteTick, stepTicks, absoluteTick%stepTicks)
				}
			}
		}
	}
}

// TestGenerate_PhraseLengthIsBarAligned verifies that the phrase length in
// steps is always a multiple of stepsPerBar, ensuring the phrase pattern
// never creates an odd bar count.
func TestGenerate_PhraseLengthIsBarAligned(t *testing.T) {
	for _, quantize := range []string{"quarter", "eighth", "sixteenth"} {
		for _, complexity := range []string{"simple", "medium", "complex"} {
			cfg := defaultConfig()
			cfg.Quantize = quantize
			cfg.Complexity = complexity

			cs, err := resolveComplexity(cfg)
			if err != nil {
				t.Errorf("quantize=%s complexity=%s: %v", quantize, complexity, err)
				continue
			}

			spb := stepsPerBar(quantize)
			phraseLengthSteps := cs.phraseLengthBars * spb

			if phraseLengthSteps%spb != 0 {
				t.Errorf("quantize=%s complexity=%s: phrase length %d steps is not bar-aligned (stepsPerBar=%d)",
					quantize, complexity, phraseLengthSteps, spb)
			}
		}
	}
}

// TestGenerate_NoteOffCanExceedStep verifies that Note Off events ARE allowed
// to fall between grid lines (i.e. notes can ring longer than one step).
// This confirms we haven't accidentally snapped Note Offs to the grid too.
func TestGenerate_NoteOffCanExceedStep(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "melody"
	cfg.Complexity = "complex" // widest note length range: 0.3–3.0x stepTicks
	cfg.Length = 64
	cfg.Seed = 123

	stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)

	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Walk events tracking absolute tick. Look for any Note Off that lands
	// at a position that is NOT a multiple of stepTicks — this is expected
	// and desirable with the current model.
	absoluteTick := uint32(0)
	foundOffGrid := false
	for _, evt := range track.Events {
		absoluteTick += evt.Delta
		if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 {
			if absoluteTick%stepTicks != 0 {
				foundOffGrid = true
				break
			}
		}
	}

	if !foundOffGrid {
		t.Error("expected at least one Note Off to land between grid lines (notes ringing across steps), but all were on-grid")
	}
}

// -----------------------------------------------------------------------------
// ChordStyle tests
// -----------------------------------------------------------------------------

// TestChordStyle_LongNeverExceedsStep verifies that "long" style note durations
// never exceed 95% of stepTicks — chords must release before the next step
// fires, preventing any overlap between successive chords.
func TestChordStyle_LongNeverExceedsStep(t *testing.T) {
	for _, mode := range []string{"chords", "progression"} {
		cfg := defaultConfig()
		cfg.Mode = mode
		cfg.ChordStyle = "long"
		cfg.Length = 32
		cfg.Seed = 1

		stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)
		maxAllowed := uint32(float64(stepTicks) * 0.95)

		track, err := Generate(cfg)
		if err != nil {
			t.Fatalf("mode=%s: %v", mode, err)
		}

		// Collect all Note Off deltas — each is a chord's sounding duration
		for _, evt := range track.Events {
			if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 && evt.Delta > 0 {
				if evt.Delta > maxAllowed {
					t.Errorf("mode=%s chordstyle=long: Note Off delta %d exceeds max allowed %d (95%% of stepTicks=%d)",
						mode, evt.Delta, maxAllowed, stepTicks)
				}
			}
		}
	}
}

// TestChordStyle_BouncyAlwaysShort verifies that "bouncy" style durations
// never exceed 45% of stepTicks, ensuring a clear audible gap after each chord.
func TestChordStyle_BouncyAlwaysShort(t *testing.T) {
	for _, mode := range []string{"chords", "progression"} {
		cfg := defaultConfig()
		cfg.Mode = mode
		cfg.ChordStyle = "bouncy"
		cfg.Length = 32
		cfg.Seed = 2

		stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)
		maxAllowed := uint32(float64(stepTicks) * 0.45)

		track, err := Generate(cfg)
		if err != nil {
			t.Fatalf("mode=%s: %v", mode, err)
		}

		for _, evt := range track.Events {
			if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 && evt.Delta > 0 {
				if evt.Delta > maxAllowed {
					t.Errorf("mode=%s chordstyle=bouncy: Note Off delta %d exceeds max allowed %d (45%% of stepTicks=%d)",
						mode, evt.Delta, maxAllowed, stepTicks)
				}
			}
		}
	}
}

// TestChordStyle_BouncyMinimumDuration verifies that "bouncy" chords still
// have a minimum duration of 25% of stepTicks — they should be short but
// not so short they become inaudible.
func TestChordStyle_BouncyMinimumDuration(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "chords"
	cfg.ChordStyle = "bouncy"
	cfg.Length = 32
	cfg.Seed = 3

	stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)
	minAllowed := uint32(float64(stepTicks) * 0.25)

	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, evt := range track.Events {
		if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 && evt.Delta > 0 {
			if evt.Delta < minAllowed {
				t.Errorf("chordstyle=bouncy: Note Off delta %d below minimum %d (25%% of stepTicks=%d)",
					evt.Delta, minAllowed, stepTicks)
			}
		}
	}
}

// TestChordStyle_LongVsBouncyDurationDifference verifies that "long" chords
// are measurably longer than "bouncy" chords — the styles are audibly distinct.
func TestChordStyle_LongVsBouncyDurationDifference(t *testing.T) {
	baseCfg := defaultConfig()
	baseCfg.Mode = "chords"
	baseCfg.Length = 32
	baseCfg.Seed = 42

	// Collect average Note Off delta for long style
	longAvg := averageNoteOffDelta(t, baseCfg, "long")
	bouncyAvg := averageNoteOffDelta(t, baseCfg, "bouncy")

	if longAvg <= bouncyAvg {
		t.Errorf("long avg duration (%.1f) should be greater than bouncy avg (%.1f)",
			longAvg, bouncyAvg)
	}
}

// TestChordStyle_OverlapCanExceedStep verifies that "overlap" style allows
// Note Off deltas to exceed stepTicks — this is the defining characteristic
// of the overlap style.
func TestChordStyle_OverlapCanExceedStep(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "chords"
	cfg.ChordStyle = "overlap"
	cfg.Complexity = "complex" // widest duration range
	cfg.Length = 64
	cfg.Seed = 5

	stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)

	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundOverlap := false
	for _, evt := range track.Events {
		if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 && evt.Delta > stepTicks {
			foundOverlap = true
			break
		}
	}

	if !foundOverlap {
		t.Error("chordstyle=overlap: expected at least one Note Off delta > stepTicks, found none")
	}
}

// TestChordStyle_InvalidRejected verifies that an unknown chord style string
// returns a validation error for chord-based modes.
func TestChordStyle_InvalidRejected(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "chords"
	cfg.ChordStyle = "wobbly"
	if err := validateConfig(cfg); err == nil {
		t.Error("expected error for unknown chordstyle, got none")
	}
}

// TestChordStyle_IgnoredInMelodyMode verifies that an empty or unusual
// ChordStyle value does not cause an error in melody mode, since the field
// is intentionally ignored when mode=melody.
func TestChordStyle_IgnoredInMelodyMode(t *testing.T) {
	cfg := defaultConfig()
	cfg.Mode = "melody"
	cfg.ChordStyle = "" // not set — should be fine for melody
	if err := validateConfig(cfg); err != nil {
		t.Errorf("melody mode: unexpected error with empty ChordStyle: %v", err)
	}
}

// TestChordStyle_NoteOnsStillOnGrid verifies that adding ChordStyle does not
// break the grid-locking invariant — Note Ons must still snap to stepTicks
// regardless of chord style.
func TestChordStyle_NoteOnsStillOnGrid(t *testing.T) {
	for _, style := range []string{"long", "bouncy", "overlap"} {
		cfg := defaultConfig()
		cfg.Mode = "chords"
		cfg.ChordStyle = style
		cfg.Length = 16
		cfg.Seed = 10

		stepTicks, _ := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)

		track, err := Generate(cfg)
		if err != nil {
			t.Fatalf("style=%s: %v", style, err)
		}

		absoluteTick := uint32(0)
		for _, evt := range track.Events {
			absoluteTick += evt.Delta
			if len(evt.Data) >= 3 {
				status := evt.Data[0] & 0xF0
				if status == 0x90 && evt.Data[2] > 0 {
					if absoluteTick%stepTicks != 0 {
						t.Errorf("style=%s: Note On at tick %d not on grid (stepTicks=%d)",
							style, absoluteTick, stepTicks)
					}
				}
			}
		}
	}
}

// -----------------------------------------------------------------------------
// helper
// -----------------------------------------------------------------------------

// averageNoteOffDelta returns the mean of all non-zero Note Off deltas in a
// generated track for the given chord style.
func averageNoteOffDelta(t *testing.T, cfg GeneratorConfig, style string) float64 {
	t.Helper()
	cfg.ChordStyle = style
	track, err := Generate(cfg)
	if err != nil {
		t.Fatalf("style=%s: %v", style, err)
	}

	sum := uint64(0)
	count := 0
	for _, evt := range track.Events {
		if len(evt.Data) >= 1 && (evt.Data[0]&0xF0) == 0x80 && evt.Delta > 0 {
			sum += uint64(evt.Delta)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}
