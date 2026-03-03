package theory

import (
	"fmt"
	"math/rand"
	"midi-gen/midi"
)

// GeneratorConfig holds all parameters that control music generation.
// It is the single point of configuration passed from the CLI into Generate().
type GeneratorConfig struct {
	// Scale is the name of the scale to draw notes from.
	// Must be a key in theory.Scales (e.g. "major", "blues", "dorian").
	Scale string

	// RootNote is the MIDI note number of the scale/chord root.
	// e.g. 60 = C4 (middle C). Use NoteNumber() to convert from a name.
	RootNote int

	// Octaves controls how many octaves of the scale are available to the
	// generator as a note pool. Higher values = wider melodic range.
	// Minimum 1. Typical values: 1–3.
	Octaves int

	// Length is the number of steps to generate. Each step is one grid slot
	// of stepTicks width. Steps may produce a note, a rest, or a held note
	// from the previous step.
	Length int

	// MinVel and MaxVel define the velocity (volume) range for generated notes.
	// MIDI velocity: 0–127. A narrow range sounds mechanical; a wider range
	// produces humanized dynamics. Typical: MinVel=60, MaxVel=100.
	MinVel int
	MaxVel int

	// Complexity controls rhythmic and harmonic density:
	//   "simple"  — longer notes, triads only, gentle variation
	//   "medium"  — mixed lengths, 7th chords, moderate variation
	//   "complex" — short notes mixed with long holds, all chord types, high variation
	Complexity string

	// Mode controls what is generated:
	//   "melody"      — single-note melodic line
	//   "chords"      — repeated random chords (same quality throughout)
	//   "progression" — chord progression using diatonic harmony rules
	Mode string

	// BPM is the tempo in beats per minute. Used to set the MIDI tempo
	// meta-event and to calculate note durations in ticks.
	BPM int

	// TicksPerQN is the timing resolution of the output MIDI file.
	// Common values: 96 (basic), 480 (DAW standard), 960 (high resolution).
	// A quarter note = TicksPerQN ticks. An eighth note = TicksPerQN/2 ticks.
	TicksPerQN int

	// Quantize controls the step grid — the rhythmic unit that Note Ons snap to.
	// Note Offs are independent and can fall between grid lines.
	//   "quarter"   — note ons on quarter note boundaries
	//   "eighth"    — note ons on eighth note boundaries
	//   "sixteenth" — note ons on sixteenth note boundaries
	Quantize string

	// Progression is the parsed output of -prog. When non-nil, the generator
	// uses these chords instead of random selection. Each mode interprets the
	// progression differently — see generateMelody, generateChords,
	// generateProgression for details.
	// nil = use random behaviour (existing default).
	Progression []ProgChord

	// ChordRate controls how many steps each chord in the progression occupies
	// before advancing to the next. Only used when Progression is non-nil.
	//   "beat" — one beat worth of steps (e.g. 2 steps at eighth quantize)
	//   "bar"  — one full bar of steps  (e.g. 8 steps at eighth quantize)
	// Default: "bar"
	ChordRate string

	// ChordStyle controls how long chords sustain relative to the step grid.
	// Only applies to "chords" and "progression" modes. Ignored in "melody" mode.
	//   "long"    — chords sustain for ~95% of stepTicks, no bleed into next step.
	//               Good for pads, slow progressions, ambient textures.
	//   "bouncy"  — chords sustain for ~35% of stepTicks, crisp gap before next chord.
	//               Good for rhythm guitar, funk, staccato parts.
	//   "overlap" — chords sustain from the phrase pattern and may ring into the
	//               next step. Good for lush, blurred harmonic textures.
	ChordStyle string

	// Seed controls the random number generator.
	// 0 = use a random seed (different output each run).
	// Any other value = deterministic output (same seed → same output).
	Seed int64
}

// complexitySettings holds the derived per-complexity parameters.
// Computed once from Complexity in resolveComplexity() and passed through
// the generation pipeline.
type complexitySettings struct {
	// chordQualities is the pool of chord types available at this complexity.
	chordQualities []string

	// noteLengthMin and noteLengthMax are the bounds for note duration as a
	// fraction of stepTicks. e.g. min=0.5, max=2.0 means notes can be as
	// short as half a step or as long as two steps.
	// These bounds apply to the phrase pattern durations before velocity
	// scaling and jitter are applied.
	noteLengthMin float64
	noteLengthMax float64

	// restProbability is the 0.0–1.0 chance that a step produces a rest.
	restProbability float64

	// velocityJitter is the max random deviation above MinVel per note.
	velocityJitter int

	// phraseLengthBars controls how many bars the phrase pattern spans.
	// The phrase is generated once and repeated for the full length.
	// Values: 1 = one bar, 2 = two bars.
	phraseLengthBars int

	// durationJitterFraction is the max ± random fraction applied to each
	// note duration on top of velocity scaling. e.g. 0.1 = ±10%.
	durationJitterFraction float64
}

// quantizeTicks converts a Quantize string to the number of ticks per step.
//
// With TicksPerQN=480:
//
//	"quarter"   → 480 ticks  (one beat)
//	"eighth"    → 240 ticks  (half a beat)
//	"sixteenth" → 120 ticks  (quarter of a beat)
func quantizeTicks(quantize string, ticksPerQN int) (uint32, error) {
	switch quantize {
	case "quarter":
		return uint32(ticksPerQN), nil
	case "eighth":
		return uint32(ticksPerQN / 2), nil
	case "sixteenth":
		return uint32(ticksPerQN / 4), nil
	default:
		return 0, fmt.Errorf("quantizeTicks: unknown quantize value %q", quantize)
	}
}

// stepsPerBar returns how many steps fit in one 4/4 bar at the given quantize.
//
//	quarter   → 4 steps per bar
//	eighth    → 8 steps per bar
//	sixteenth → 16 steps per bar
func stepsPerBar(quantize string) int {
	switch quantize {
	case "quarter":
		return 4
	case "eighth":
		return 8
	case "sixteenth":
		return 16
	default:
		return 8
	}
}

// resolveComplexity builds a complexitySettings from a config.
//
// Note length range per complexity:
//
//	simple:  0.8–2.0x stepTicks — mostly longer than one step, legato feel
//	medium:  0.5–2.0x stepTicks — balanced short and long, natural phrasing
//	complex: 0.3–3.0x stepTicks — wide range, short stabs and long holds mixed
func resolveComplexity(cfg GeneratorConfig) (complexitySettings, error) {
	switch cfg.Complexity {
	case "simple":
		return complexitySettings{
			chordQualities:         []string{"major", "minor", "dim", "aug", "sus2", "sus4"},
			noteLengthMin:          0.8,
			noteLengthMax:          2.0,
			restProbability:        0.05,
			velocityJitter:         15,
			phraseLengthBars:       1,
			durationJitterFraction: 0.05,
		}, nil
	case "medium":
		return complexitySettings{
			chordQualities:         []string{"major", "minor", "dom7", "maj7", "min7", "dim", "sus2", "sus4"},
			noteLengthMin:          0.5,
			noteLengthMax:          2.0,
			restProbability:        0.1,
			velocityJitter:         25,
			phraseLengthBars:       2,
			durationJitterFraction: 0.10,
		}, nil
	case "complex":
		return complexitySettings{
			chordQualities:         ChordQualities(),
			noteLengthMin:          0.3,
			noteLengthMax:          3.0,
			restProbability:        0.2,
			velocityJitter:         40,
			phraseLengthBars:       2,
			durationJitterFraction: 0.15,
		}, nil
	default:
		return complexitySettings{}, fmt.Errorf("resolveComplexity: unknown complexity %q", cfg.Complexity)
	}
}

// generatePhraseDurations builds a slice of base note durations (in ticks)
// for one complete phrase of phraseLengthSteps steps.
//
// Each slot is assigned a random duration in the range
// [noteLengthMin * stepTicks, noteLengthMax * stepTicks].
//
// These durations control only how long notes ring (Note Off timing).
// They do NOT affect when Note Ons fire — Note Ons always snap to the step
// grid regardless of how long the previous note rang.
//
// Memory layout of returned slice:
//
//	[]uint32 of length phraseLengthSteps
//	index i: base note duration in MIDI ticks for phrase slot i
func generatePhraseDurations(cs complexitySettings, stepTicks uint32, phraseLengthSteps int, rng *rand.Rand) []uint32 {
	phrase := make([]uint32, phraseLengthSteps)
	for i := range phrase {
		// Draw a random multiplier in [noteLengthMin, noteLengthMax]
		// rng.Float64() returns [0.0, 1.0), so we scale and shift into our range.
		rangeWidth := cs.noteLengthMax - cs.noteLengthMin
		multiplier := cs.noteLengthMin + rng.Float64()*rangeWidth

		// Convert to ticks, ensure at least 1 tick
		ticks := uint32(float64(stepTicks) * multiplier)
		if ticks < 1 {
			ticks = 1
		}
		phrase[i] = ticks
	}
	return phrase
}

// randomNoteDuration computes the final sounding duration for a single note,
// combining two layers on top of the phrase base duration:
//
//  1. Velocity correlation — louder notes ring longer, quieter notes are
//     released sooner, mimicking natural player behaviour.
//
//     The velocity range [MinVel, MaxVel] maps to a duration multiplier
//     of [0.75, 1.10]:
//     t = (velocity - MinVel) / (MaxVel - MinVel)   → 0.0..1.0
//     velMultiplier = 0.75 + t * 0.35               → 0.75..1.10
//
//  2. Per-note jitter — a small random ± fraction breaks mechanical regularity.
//     jitter = (rand * 2 - 1) * durationJitterFraction * scaledDuration
//
// Result is clamped to a minimum of 1 tick.
func randomNoteDuration(baseDuration uint32, velocity int, cfg GeneratorConfig, cs complexitySettings, rng *rand.Rand) uint32 {
	// --- Layer 1: velocity correlation ---
	velRange := cfg.MaxVel - cfg.MinVel
	var t float64
	if velRange > 0 {
		t = float64(velocity-cfg.MinVel) / float64(velRange)
	} else {
		t = 0.5
	}
	velMultiplier := 0.75 + t*0.35
	scaledDuration := float64(baseDuration) * velMultiplier

	// --- Layer 2: per-note jitter ---
	// (rng.Float64()*2 - 1) produces a value in (-1.0, 1.0)
	jitter := (rng.Float64()*2 - 1) * cs.durationJitterFraction * scaledDuration
	finalDuration := scaledDuration + jitter

	if finalDuration < 1 {
		finalDuration = 1
	}
	return uint32(finalDuration)
}

// Generate produces a MIDI track from the given GeneratorConfig.
//
// Generation flow:
//  1. Validate config and seed the RNG
//  2. Compute step size (ticks) from Quantize + TicksPerQN
//  3. Resolve complexity settings
//  4. Build the note pool from Scale + RootNote + Octaves
//  5. Generate the phrase rhythm pattern (note durations only, not grid positions)
//  6. Dispatch to the appropriate mode generator
//  7. Return the populated Track
func Generate(cfg GeneratorConfig) (midi.Track, error) {
	if err := validateConfig(cfg); err != nil {
		return midi.Track{}, err
	}

	var src rand.Source
	if cfg.Seed != 0 {
		src = rand.NewSource(cfg.Seed)
	} else {
		src = rand.NewSource(rand.Int63())
	}
	rng := rand.New(src)

	stepTicks, err := quantizeTicks(cfg.Quantize, cfg.TicksPerQN)
	if err != nil {
		return midi.Track{}, err
	}

	cs, err := resolveComplexity(cfg)
	if err != nil {
		return midi.Track{}, err
	}

	notePool, err := ScaleNotes(cfg.RootNote, cfg.Scale, cfg.Octaves)
	if err != nil {
		return midi.Track{}, err
	}
	if len(notePool) == 0 {
		return midi.Track{}, fmt.Errorf("Generate: note pool is empty for root=%d scale=%s octaves=%d",
			cfg.RootNote, cfg.Scale, cfg.Octaves)
	}

	// Compute phrase length in steps: phraseLengthBars * stepsPerBar
	// e.g. medium complexity, eighth quantize: 2 bars * 8 steps/bar = 16 steps
	// This guarantees the phrase always divides evenly into complete bars.
	phraseLengthSteps := cs.phraseLengthBars * stepsPerBar(cfg.Quantize)
	phrase := generatePhraseDurations(cs, stepTicks, phraseLengthSteps, rng)

	// Compute stepsPerChord for progression-aware generation.
	// When cfg.Progression is nil this value is unused — the random generators
	// ignore it. When non-nil it controls how many steps each chord occupies
	// before the generator advances to the next chord in the progression.
	stepsPerChord := 0
	if cfg.Progression != nil {
		stepsPerChord, err = StepsPerChord(cfg.ChordRate, cfg.Quantize)
		if err != nil {
			return midi.Track{}, err
		}
	}

	track := midi.Track{}
	track.Events = append(track.Events, midi.Tempo(0, cfg.BPM))

	var events []midi.Event
	switch cfg.Mode {
	case "melody":
		events = generateMelody(cfg, cs, rng, notePool, stepTicks, phrase, stepsPerChord)
	case "chords":
		events, err = generateChords(cfg, cs, rng, stepTicks, phrase, stepsPerChord)
	case "progression":
		events, err = generateProgression(cfg, cs, rng, stepTicks, phrase, stepsPerChord)
	default:
		return midi.Track{}, fmt.Errorf("Generate: unknown mode %q", cfg.Mode)
	}
	if err != nil {
		return midi.Track{}, err
	}

	track.Events = append(track.Events, events...)
	return track, nil
}

// generateMelody produces a single-note melodic line snapped to the step grid.
//
// Timing model — two independent tick counters:
//
//	absoluteTick  tracks the current step grid position (always a multiple of stepTicks)
//	lastEventTick tracks the tick at which the last MIDI event was written
//
// Note On  delta = absoluteTick - lastEventTick
//
//	This is always >= 0. It accounts for rests and the gap between a
//	Note Off and the next Note On correctly, regardless of note duration.
//
// Note Off delta = noteDuration (from randomNoteDuration)
//
//	Fires noteDuration ticks after the Note On. May land between grid lines.
//	May also land after the next step's Note On would have fired — in that
//	case the Note Off fires first and the next Note On fires immediately after
//	(delta=0 from the Note Off's perspective).
//
// absoluteTick advances by stepTicks every step, unconditionally.
// lastEventTick is updated after every event written.
//
// When cfg.Progression is non-nil, the note pool is narrowed at each step to
// only the tones of the currently active chord. This makes the melody outline
// the harmonic changes rather than drawing freely from the full scale.
// If narrowing produces an empty pool (chord tones outside the octave range),
// the full notePool is used as a fallback for that step.
//
// stepsPerChord controls how many steps each chord occupies. It is only used
// when cfg.Progression is non-nil — pass 0 for random/unspecified behaviour.
func generateMelody(cfg GeneratorConfig, cs complexitySettings, rng *rand.Rand, notePool []int, stepTicks uint32, phrase []uint32, stepsPerChord int) []midi.Event {
	events := []midi.Event{}

	// absoluteTick: current position on the step grid in ticks.
	// Always advances by stepTicks each step, keeping Note Ons grid-locked.
	absoluteTick := uint32(0)

	// lastEventTick: the absolute tick at which the most recent MIDI event
	// (Note On or Note Off) was written. Used to compute deltas.
	lastEventTick := uint32(0)

	for i := 0; i < cfg.Length; i++ {
		if rng.Float64() < cs.restProbability {
			// Rest: advance the grid position but write no events.
			// The gap accumulates silently into the next Note On's delta.
			absoluteTick += stepTicks
			continue
		}

		// Determine the note pool for this step.
		// When a progression is specified, narrow the pool to chord tones
		// of the currently active chord so the melody outlines the harmony.
		activePool := notePool
		if cfg.Progression != nil && stepsPerChord > 0 {
			activePool = chordTonePool(notePool, cfg.Progression, i, stepsPerChord)
			// Fall back to full scale pool if no chord tones are in range
			if len(activePool) == 0 {
				activePool = notePool
			}
		}

		note := activePool[rng.Intn(len(activePool))]

		velocity := cfg.MinVel + rng.Intn(cs.velocityJitter+1)
		if velocity > cfg.MaxVel {
			velocity = cfg.MaxVel
		}

		baseDuration := phrase[i%len(phrase)]
		noteDuration := randomNoteDuration(baseDuration, velocity, cfg, cs, rng)

		// Note On fires at absoluteTick (grid-snapped).
		// delta = how many ticks since the last event (Note On or Note Off).
		noteOnDelta := absoluteTick - lastEventTick
		events = append(events, midi.NoteOn(noteOnDelta, 0, byte(note), byte(velocity)))
		lastEventTick = absoluteTick

		// Note Off fires noteDuration ticks after the Note On.
		// This is independent of the grid — it may fall between steps.
		events = append(events, midi.NoteOff(noteDuration, 0, byte(note)))
		lastEventTick = absoluteTick + noteDuration

		// Advance the grid to the next step.
		absoluteTick += stepTicks
	}

	return events
}

// chordTonePool filters notePool to only the notes that belong to the chord
// active at the given step index.
//
// The chord tones are determined by building the chord from the active
// ProgChord's root and quality, then collecting all pitch classes that match
// any note in notePool. This preserves octave range — only notes already in
// the configured pool are kept, not arbitrary chord tones outside range.
//
// Parameters:
//
//	notePool      — the full scale note pool built from cfg.RootNote+Scale+Octaves
//	prog          — the parsed progression slice
//	step          — current generator step index
//	stepsPerChord — steps per chord (from StepsPerChord)
func chordTonePool(notePool []int, prog []ProgChord, step int, stepsPerChord int) []int {
	activeChord := ProgChordAt(prog, step, stepsPerChord)

	// Build the chord to get its intervals, then collect pitch classes
	chordNotes, err := BuildChord(activeChord.Root, activeChord.Quality)
	if err != nil || len(chordNotes) == 0 {
		return nil
	}

	// Build a set of chord tone pitch classes (0–11)
	chordPitchClasses := make(map[int]bool, len(chordNotes))
	for _, n := range chordNotes {
		chordPitchClasses[n%12] = true
	}

	// Filter notePool to notes whose pitch class is in the chord
	filtered := make([]int, 0, len(notePool))
	for _, n := range notePool {
		if chordPitchClasses[n%12] {
			filtered = append(filtered, n)
		}
	}

	return filtered
}

// chordNoteDuration computes the final note duration for a chord step,
// applying the ChordStyle clamp on top of the velocity-correlated duration.
//
// Style clamping is applied after randomNoteDuration so velocity correlation
// and jitter still contribute naturalness within the style's envelope:
//
//	"long":    clamp to [85%, 95%] of stepTicks
//	            Notes always release before the next step — no overlap.
//	            The small random range (85–95%) prevents a mechanical uniform feel.
//
//	"bouncy":  clamp to [25%, 45%] of stepTicks
//	            Short, punchy hits with a clear rest gap before the next chord.
//	            Velocity correlation still applies within this window.
//
//	"overlap": no clamp — duration comes entirely from randomNoteDuration.
//	            Chords may ring well into the next step or beyond.
//	            Only use this intentionally; it is not the default.
func chordNoteDuration(baseDuration uint32, velocity int, stepTicks uint32, cfg GeneratorConfig, cs complexitySettings, rng *rand.Rand) uint32 {
	duration := randomNoteDuration(baseDuration, velocity, cfg, cs, rng)

	switch cfg.ChordStyle {
	case "long":
		// Clamp to [85%, 95%] of stepTicks
		// The small jitter window (10% range) keeps it from sounding robotic
		// while guaranteeing no overlap with the next chord.
		minD := uint32(float64(stepTicks) * 0.85)
		maxD := uint32(float64(stepTicks) * 0.95)
		if duration < minD {
			duration = minD
		}
		if duration > maxD {
			duration = maxD
		}

	case "bouncy":
		// Clamp to [25%, 45%] of stepTicks
		// Short attack, clear gap — the chord punches and releases quickly.
		minD := uint32(float64(stepTicks) * 0.25)
		maxD := uint32(float64(stepTicks) * 0.45)
		if duration < minD {
			duration = minD
		}
		if duration > maxD {
			duration = maxD
		}

	default: // "overlap" — no clamping, phrase duration used as-is
	}

	// Safety: never return 0 ticks
	if duration < 1 {
		duration = 1
	}
	return duration
}

// generateChords produces a sequence of chords snapped to the step grid.
//
// Uses the same absoluteTick / lastEventTick model as generateMelody.
// All notes in a chord share the same Note On tick (simultaneous attack):
//   - First note in chord: delta = absoluteTick - lastEventTick
//   - Subsequent notes:    delta = 0  (same tick as first note)
//
// All notes in a chord share the same Note Off tick (simultaneous release):
//   - First Note Off:  delta = noteDuration (ticks after Note On)
//   - Subsequent Note Offs: delta = 0
//
// When cfg.Progression is non-nil, chords are drawn from the progression
// in order using ProgChordAt rather than randomly. stepsPerChord controls
// how many steps each chord occupies before advancing.
//
// stepsPerChord is only used when cfg.Progression is non-nil.
func generateChords(cfg GeneratorConfig, cs complexitySettings, rng *rand.Rand, stepTicks uint32, phrase []uint32, stepsPerChord int) ([]midi.Event, error) {
	events := []midi.Event{}
	absoluteTick := uint32(0)
	lastEventTick := uint32(0)

	// When no progression is specified, pick one quality for the whole track.
	// When a progression is specified, quality comes from the ProgChord at each step.
	randomQuality := cs.chordQualities[rng.Intn(len(cs.chordQualities))]

	scaleNotes, err := ScaleNotes(cfg.RootNote, cfg.Scale, cfg.Octaves)
	if err != nil {
		return nil, err
	}

	for i := 0; i < cfg.Length; i++ {
		if rng.Float64() < cs.restProbability {
			absoluteTick += stepTicks
			continue
		}

		// Determine root and quality for this step.
		// Progression mode: use the chord specified at this step index.
		// Random mode: pick a random root from the scale pool and use the
		// single quality chosen above for the whole track.
		var root int
		var quality string
		if cfg.Progression != nil && stepsPerChord > 0 {
			pc := ProgChordAt(cfg.Progression, i, stepsPerChord)
			root = pc.Root
			quality = pc.Quality
		} else {
			root = scaleNotes[rng.Intn(len(scaleNotes))]
			quality = randomQuality
		}

		chordNotes, err := BuildChord(root, quality)
		if err != nil || len(chordNotes) == 0 {
			absoluteTick += stepTicks
			continue
		}

		maxInversion := 0
		switch cfg.Complexity {
		case "medium":
			maxInversion = 1
		case "complex":
			maxInversion = len(chordNotes) - 1
		}
		if maxInversion > 0 {
			inv := rng.Intn(maxInversion + 1)
			if inverted, err := BuildChordInversion(root, quality, inv); err == nil {
				chordNotes = inverted
			}
		}

		velocity := cfg.MinVel + rng.Intn(cs.velocityJitter+1)
		if velocity > cfg.MaxVel {
			velocity = cfg.MaxVel
		}

		baseDuration := phrase[i%len(phrase)]
		noteDuration := chordNoteDuration(baseDuration, velocity, stepTicks, cfg, cs, rng)

		// Note On burst — all notes at absoluteTick
		for j, note := range chordNotes {
			delta := uint32(0)
			if j == 0 {
				delta = absoluteTick - lastEventTick
			}
			events = append(events, midi.NoteOn(delta, 0, byte(note), byte(velocity)))
		}
		lastEventTick = absoluteTick

		// Note Off burst — all notes released simultaneously after noteDuration
		for j, note := range chordNotes {
			delta := uint32(0)
			if j == 0 {
				delta = noteDuration
			}
			events = append(events, midi.NoteOff(delta, 0, byte(note)))
		}
		lastEventTick = absoluteTick + noteDuration

		absoluteTick += stepTicks
	}

	return events, nil
}

// generateProgression produces a diatonic chord progression snapped to the grid.
// Uses the same timing model as generateChords.
//
// When cfg.Progression is non-nil, chords come from the user-specified
// progression via ProgChordAt rather than the random diatonic pool.
// Inversions and voicing from complexity still apply in both cases.
//
// stepsPerChord is only used when cfg.Progression is non-nil.
func generateProgression(cfg GeneratorConfig, cs complexitySettings, rng *rand.Rand, stepTicks uint32, phrase []uint32, stepsPerChord int) ([]midi.Event, error) {
	events := []midi.Event{}
	absoluteTick := uint32(0)
	lastEventTick := uint32(0)

	// Build the diatonic chord pool for random mode.
	// When cfg.Progression is non-nil this pool is unused — we build it
	// anyway to avoid a conditional around the scaleSet which is also
	// needed for quality inference in the random path.
	scaleNotes, err := ScaleNotes(cfg.RootNote, cfg.Scale, 1)
	if err != nil {
		return nil, err
	}

	scaleSet := make(map[int]bool)
	for _, n := range scaleNotes {
		scaleSet[n%12] = true
	}

	type diatonicChord struct {
		root    int
		quality string
	}
	chordPool := make([]diatonicChord, 0, len(scaleNotes))
	for _, root := range scaleNotes {
		majorThird := (root + 4) % 12
		quality := "minor"
		if scaleSet[majorThird] {
			quality = "major"
		}
		if cfg.Complexity == "medium" || cfg.Complexity == "complex" {
			if quality == "major" {
				quality = "maj7"
			} else {
				quality = "min7"
			}
		}
		chordPool = append(chordPool, diatonicChord{root: root, quality: quality})
	}

	if len(chordPool) == 0 && cfg.Progression == nil {
		return nil, fmt.Errorf("generateProgression: empty chord pool")
	}

	for i := 0; i < cfg.Length; i++ {
		if rng.Float64() < cs.restProbability {
			absoluteTick += stepTicks
			continue
		}

		// Determine root and quality for this step.
		// Progression mode: use ProgChordAt to look up the specified chord.
		// Random mode: weighted random selection from diatonic pool with
		// tonic pull (I chord gets 2x probability).
		var root int
		var quality string
		if cfg.Progression != nil && stepsPerChord > 0 {
			pc := ProgChordAt(cfg.Progression, i, stepsPerChord)
			root = pc.Root
			quality = pc.Quality
		} else {
			var chord diatonicChord
			if rng.Float64() < 2.0/float64(len(chordPool)+1) {
				chord = chordPool[0]
			} else {
				chord = chordPool[rng.Intn(len(chordPool))]
			}
			root = chord.root
			quality = chord.quality
		}

		// Build the chord voicing from the resolved root and quality.
		// This runs in both progression and random modes.
		chordNotes, err := BuildChord(root, quality)
		if err != nil || len(chordNotes) == 0 {
			absoluteTick += stepTicks
			continue
		}

		// Apply inversions based on complexity — same logic as generateChords.
		maxInversion := 0
		switch cfg.Complexity {
		case "medium":
			maxInversion = 1
		case "complex":
			maxInversion = len(chordNotes) - 1
		}
		if maxInversion > 0 {
			inv := rng.Intn(maxInversion + 1)
			if inverted, err := BuildChordInversion(root, quality, inv); err == nil {
				chordNotes = inverted
			}
		}

		velocity := cfg.MinVel + rng.Intn(cs.velocityJitter+1)
		if velocity > cfg.MaxVel {
			velocity = cfg.MaxVel
		}

		baseDuration := phrase[i%len(phrase)]
		noteDuration := chordNoteDuration(baseDuration, velocity, stepTicks, cfg, cs, rng)

		// Note On burst
		for j, note := range chordNotes {
			delta := uint32(0)
			if j == 0 {
				delta = absoluteTick - lastEventTick
			}
			events = append(events, midi.NoteOn(delta, 0, byte(note), byte(velocity)))
		}
		lastEventTick = absoluteTick

		// Note Off burst
		for j, note := range chordNotes {
			delta := uint32(0)
			if j == 0 {
				delta = noteDuration
			}
			events = append(events, midi.NoteOff(delta, 0, byte(note)))
		}
		lastEventTick = absoluteTick + noteDuration

		absoluteTick += stepTicks
	}

	return events, nil
}

// validateConfig checks all GeneratorConfig fields for validity.
func validateConfig(cfg GeneratorConfig) error {
	if _, ok := Scales[cfg.Scale]; !ok {
		return fmt.Errorf("validateConfig: unknown scale %q", cfg.Scale)
	}
	if cfg.RootNote < 0 || cfg.RootNote > 127 {
		return fmt.Errorf("validateConfig: root note %d outside MIDI range 0–127", cfg.RootNote)
	}
	if cfg.Octaves < 1 {
		return fmt.Errorf("validateConfig: octaves must be >= 1, got %d", cfg.Octaves)
	}
	if cfg.Length < 1 {
		return fmt.Errorf("validateConfig: length must be >= 1, got %d", cfg.Length)
	}
	if cfg.MinVel < 0 || cfg.MinVel > 127 {
		return fmt.Errorf("validateConfig: MinVel %d outside range 0–127", cfg.MinVel)
	}
	if cfg.MaxVel < 0 || cfg.MaxVel > 127 {
		return fmt.Errorf("validateConfig: MaxVel %d outside range 0–127", cfg.MaxVel)
	}
	if cfg.MinVel > cfg.MaxVel {
		return fmt.Errorf("validateConfig: MinVel %d > MaxVel %d", cfg.MinVel, cfg.MaxVel)
	}
	validComplexity := map[string]bool{"simple": true, "medium": true, "complex": true}
	if !validComplexity[cfg.Complexity] {
		return fmt.Errorf("validateConfig: unknown complexity %q", cfg.Complexity)
	}
	validModes := map[string]bool{"melody": true, "chords": true, "progression": true}
	if !validModes[cfg.Mode] {
		return fmt.Errorf("validateConfig: unknown mode %q", cfg.Mode)
	}
	if cfg.BPM < 1 || cfg.BPM > 300 {
		return fmt.Errorf("validateConfig: BPM %d outside reasonable range 1–300", cfg.BPM)
	}
	if cfg.TicksPerQN < 1 {
		return fmt.Errorf("validateConfig: TicksPerQN must be >= 1, got %d", cfg.TicksPerQN)
	}
	validQuantize := map[string]bool{"quarter": true, "eighth": true, "sixteenth": true}
	if !validQuantize[cfg.Quantize] {
		return fmt.Errorf("validateConfig: unknown quantize %q", cfg.Quantize)
	}
	// ChordRate is only validated when a Progression is specified.
	if cfg.Progression != nil {
		validChordRate := map[string]bool{"beat": true, "bar": true}
		if !validChordRate[cfg.ChordRate] {
			return fmt.Errorf("validateConfig: unknown chordrate %q (want beat|bar)", cfg.ChordRate)
		}
	}
	// ChordStyle is only required for chord-based modes.
	// In melody mode it is ignored, so we only validate when mode is chord-related.
	if cfg.Mode == "chords" || cfg.Mode == "progression" {
		validChordStyle := map[string]bool{"long": true, "bouncy": true, "overlap": true}
		if !validChordStyle[cfg.ChordStyle] {
			return fmt.Errorf("validateConfig: unknown chordstyle %q (want long|bouncy|overlap)", cfg.ChordStyle)
		}
	}
	return nil
}
