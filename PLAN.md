# MIDI Generator CLI — Implementation Plan

## Overview

A Go CLI tool that generates random MIDI chords and melodies and outputs `.mid`
files. Built from scratch with minimal dependencies — the MIDI writer is pure Go,
and audio playback uses a single lightweight library (`oto`).

---

## Architecture

```
midi-gen/
├── main.go              # CLI entry point (flag parsing, dispatch)
├── midi/
│   ├── types.go         # MIDI structs + event constructors (NoteOn, NoteOff, Tempo)
│   └── writer.go        # Raw MIDI binary serialization
├── theory/
│   ├── scales.go        # Scale definitions, note lookup, NoteNumber parser
│   ├── chords.go        # Chord builders, inversions
│   └── generator.go     # Random melody/chord/progression generation
├── synth/
│   ├── sine.go          # Sine oscillator + ADSR envelope
│   ├── reverb.go        # Schroeder reverb (4 comb + 2 allpass filters)
│   ├── scheduler.go     # MIDI event → PCM timing engine
│   └── output.go        # Audio device output via oto
└── go.mod
```

---

## Phase 1 — MIDI File Writer ✅

### Binary Format

A `.mid` file is a binary format written byte by byte.

**Header Chunk** (14 bytes, always fixed):
```
4D546864  → "MThd" magic bytes
00000006  → chunk length = 6 (always)
0001      → format type (0=single track, 1=multi-track)
0001      → number of tracks
0060      → ticks per quarter note (480 is the DAW standard)
```

**Track Chunk**:
```
4D54726B  → "MTrk" magic bytes
XXXXXXXX  → chunk length (computed after events are serialized, then backfilled)
[events]
FF 2F 00  → End of Track meta-event (mandatory — DAWs reject files without it)
```

**Variable-length delta times**: each event is preceded by how many ticks since
the last event. Encoding uses 7 bits per byte; MSB=1 means more bytes follow,
MSB=0 is the last byte.

**Note events**:
```
Note On:  [delta] 9n kk vv   (n=channel, kk=key 0–127, vv=velocity)
Note Off: [delta] 8n kk vv
```

### Key Implementation Decisions

- `encodeVarLen` is unit-tested at every byte-width boundary: 0, 127, 128, 255,
  16383, 16384, and 0xFFFFFFF.
- Track chunk length is computed by serializing events into a temporary buffer
  first, measuring it, then writing the header — the length is not known in advance.
- `NoteOn`, `NoteOff`, and `Tempo` are constructor functions on `types.go` that
  handle all bit manipulation internally. Callers never touch raw status bytes.
- All data bytes are masked (`& 0x7F`, `& 0x0F`) to prevent corruption of the
  MIDI byte stream.

---

## Phase 2 — Music Theory Engine ✅

### Note Representation

MIDI note numbers 0–127. Middle C = C4 = 60.

`NoteNumber(string)` parses human-readable notation (`"C4"`, `"F#3"`, `"Bb2"`)
into MIDI note numbers. Handles sharps, flats, enharmonic equivalents, and the
`B`/`Bb` ambiguity (uppercase `B` is both a note name and a flat symbol after
conversion).

### Scales

```go
var Scales = map[string][]int{
    "major":        {0,2,4,5,7,9,11},
    "minor":        {0,2,3,5,7,8,10},
    "pentatonic":   {0,2,4,7,9},
    "blues":        {0,3,5,6,7,10},
    "dorian":       {0,2,3,5,7,9,10},
    "phrygian":     {0,1,3,5,7,8,10},
    "lydian":       {0,2,4,6,7,9,11},
    "mixolydian":   {0,2,4,5,7,9,10},
    "harmonicminor":{0,2,3,5,7,8,11},
    "wholetone":    {0,2,4,6,8,10},
    "diminished":   {0,1,3,4,6,7,9,10},
}
```

### Chords

22 chord qualities from triads through 13th chords. `BuildChordInversion` raises
the lowest note of a voicing by one octave per inversion step, enabling smooth
voice leading between chord changes.

### Generator

**`GeneratorConfig`** is the single configuration struct passed from the CLI.

**Timing model**: two independent tick counters per generator function:
- `absoluteTick` — advances by exactly `stepTicks` each step, always a clean
  multiple. This is what Note On events are anchored to.
- `lastEventTick` — tracks where the last MIDI event actually landed in the
  stream. Note On delta = `absoluteTick - lastEventTick`.

This separation ensures Note Ons are always grid-locked regardless of how long
the previous note rang. Note Offs fire at `noteDuration` ticks after their
Note On and may land between grid lines.

**Dynamic note lengths**: three layers applied in order:
1. **Phrase pattern** — a repeating slice of base durations generated once per
   track. Length is always `phraseLengthBars * stepsPerBar(quantize)` to
   guarantee bar alignment.
2. **Velocity correlation** — louder notes ring longer. Velocity range maps to
   a duration multiplier of `[0.75, 1.10]`.
3. **Per-note jitter** — a small `±durationJitterFraction` random offset breaks
   mechanical regularity.

**Chord styles** (chords and progression modes only):

| Style | Duration range | Use case |
|-------|---------------|----------|
| `long` | 85–95% of stepTicks | Pads, sustained chords, no overlap |
| `bouncy` | 25–45% of stepTicks | Rhythm guitar, funk, staccato |
| `overlap` | unclamped (phrase-driven) | Lush blurred textures, opt-in only |

Overlap is intentionally opt-in. `long` is the default because unclamped
durations caused unintended chord bleed in early testing.

**Modes**:
- `melody` — single-note line from scale note pool
- `chords` — repeated chords of one randomly chosen quality
- `progression` — diatonic harmony: chords built on each scale degree, with a
  weighted tonic pull (I chord gets ~2x probability of other degrees)

**Complexity levels**:

| Setting | Note length range | Phrase bars | Rest prob | Chord pool |
|---------|-------------------|-------------|-----------|------------|
| simple | 0.8–2.0x step | 1 bar | 5% | triads only |
| medium | 0.5–2.0x step | 2 bars | 10% | triads + 7ths |
| complex | 0.3–3.0x step | 2 bars | 20% | all 22 types |

---

## Phase 3 — CLI Interface ✅

All flags with defaults:

```
-mode       melody | chords | progression        (default: melody)
-scale      major | minor | pentatonic | ...      (default: major)
-root       note string e.g. C4 F#3 Bb2          (default: C4)
-octaves    int, note pool range                  (default: 2)
-length     int, steps to generate               (default: 16)
-bpm        int, 1–300                            (default: 120)
-complexity simple | medium | complex             (default: medium)
-quantize   quarter | eighth | sixteenth          (default: eighth)
-minvel     int, 0–127                            (default: 60)
-maxvel     int, 0–127                            (default: 100)
-ticks      int, ticks per quarter note           (default: 480)
-chordstyle long | bouncy | overlap               (default: long)
-out        file path                             (default: output.mid)
-seed       int64, 0=random                       (default: 0)
-play       bool [Phase 4]                        (default: false)
```

Output is always MIDI Format 0 (single track) for maximum DAW compatibility.

---

## Phase 4 — Sine Synth Playback (in progress)

**One external dependency**: `github.com/ebitengine/oto/v3`

oto provides a minimal cross-platform PCM audio output interface. On macOS it
uses CoreAudio. The rest of the synth is pure Go.

### Synth Pipeline

```
midi.Track
    ↓
synth.Scheduler.Render()   MIDI events → []float32 PCM
    ↓
synth.Reverb.Process()     Schroeder reverb applied to PCM buffer
    ↓
synth.Play()               PCM fed to oto, blocks until playback complete
```

`-play` writes the `.mid` file and plays back simultaneously.

### `synth/sine.go` ✅

**`Voice`** — one sounding note. Fields:
- `Phase float64` — sine phase in radians `[0, 2π)`, advances by
  `2π * freq / SampleRate` per sample. Wrapped each sample to prevent
  float64 precision loss on long notes.
- `Velocity float64` — normalised from MIDI 0–127 to `0.0–1.0`.
- `EnvPhase envPhase` — current ADSR stage (attack/decay/sustain/release/done).
- `Releasing bool` — set by `Release()` when Note Off is received. The envelope
  transitions to release on the next `envelope()` call regardless of current stage.

**ADSR envelope**:
```
Attack:  linear ramp 0.0 → 1.0 over AttackMs
Decay:   linear fall 1.0 → SustainLevel over DecayMs
Sustain: constant at SustainLevel until Release() called
Release: linear fall SustainLevel → 0.0 over ReleaseMs
```

Per-complexity defaults:

| Complexity | Attack | Decay | Sustain | Release |
|-----------|--------|-------|---------|---------|
| simple | 5ms | 20ms | 0.8 | 30ms |
| medium | 3ms | 15ms | 0.7 | 50ms |
| complex | 2ms | 10ms | 0.6 | 80ms |

**`RenderSamples`** mixes into the buffer (`+=` not `=`) so multiple voices
can be summed by calling it on each active voice with the same buffer.
`masterGain` (typically 0.3) prevents clipping when voices are summed.

### `synth/reverb.go` (next)

Schroeder reverb model: 4 parallel comb filters feeding 2 series allpass filters.

```
Input → [Comb 1] ─┐
        [Comb 2] ─┤
        [Comb 3] ─┤→ sum → [Allpass 1] → [Allpass 2] → wet
        [Comb 4] ─┘

Output = input*dryMix + wet*wetMix
```

### `synth/scheduler.go` (next)

Converts `[]midi.Event` into a flat `[]float32` PCM buffer by walking the event
list, maintaining a pool of active `Voice` objects, and calling
`RenderSamples` on each active voice per audio buffer chunk.

### `synth/output.go` (next)

Initialises one `oto.Context` at startup (expensive, created once). Feeds the
rendered PCM buffer to an `oto.Player` and blocks until playback is complete.

---

## Go Module Setup

```
go mod init midi-gen
go get github.com/ebitengine/oto/v3   # added in Phase 4
```

Total external dependencies: **1** (oto). All MIDI generation is zero-dependency.

---

## Testing Approach

Every package has a `_test.go` file in the same package (white-box testing).
All tests use only `testing` from stdlib — no test frameworks.

Key invariants verified by tests:

- Variable-length encoding correct at every byte-width boundary
- All generated Note Ons land on exact multiples of `stepTicks` (grid-lock)
- Note Offs can land between grid lines (intentional — notes ring naturally)
- Every Note On has a paired Note Off (no stuck notes in DAW or scheduler)
- Same seed always produces byte-identical output
- Chord style duration bounds enforced (`long` ≤ 95% step, `bouncy` ≤ 45% step)
- ADSR envelope starts at 0.0 (no click), reaches sustain level, falls to 0.0
  on release including early release during attack stage
- Sine phase stays within `[0, 2π)` — no float64 precision drift