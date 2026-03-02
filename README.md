# midi-gen

A CLI tool for generating random MIDI melodies, chords, and progressions,
written in Go. Outputs standard `.mid` files compatible with any DAW, and can
play back audio directly via a built-in sine wave synth with ADSR envelopes
and Schroeder reverb.

One external dependency (`oto`) for audio playback. All MIDI generation and
music theory is zero-dependency.

---

## Installation

```sh
git clone <your-repo>
cd midi-gen
go mod tidy
go build -o midi-gen .
```

Requires Go 1.22 or later. Audio playback (`-play`) requires macOS with
CoreAudio available.

---

## Quick Start

```sh
# Generate a melody and write to a .mid file
./midi-gen -mode melody -scale minor -root A3 -length 32 -out melody.mid

# Generate and play back immediately
./midi-gen -mode melody -scale minor -root A3 -length 32 -play -out melody.mid

# Chord progression with long sustained chords
./midi-gen -mode progression -scale dorian -root D3 -chordstyle long -play -out prog.mid

# Reproducible output — same seed always produces the same file
./midi-gen -mode melody -scale pentatonic -root G4 -seed 42 -out repro.mid
```

---

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-mode` | string | `melody` | `melody` \| `chords` \| `progression` |
| `-scale` | string | `major` | See scales below |
| `-root` | string | `C4` | Root note, e.g. `C4`, `F#3`, `Bb2` |
| `-octaves` | int | `2` | Note pool range in octaves |
| `-length` | int | `16` | Number of steps to generate |
| `-bpm` | int | `120` | Tempo in beats per minute (1–300) |
| `-complexity` | string | `medium` | `simple` \| `medium` \| `complex` |
| `-quantize` | string | `eighth` | `quarter` \| `eighth` \| `sixteenth` |
| `-chordstyle` | string | `long` | `long` \| `bouncy` \| `overlap` (chords/progression only) |
| `-minvel` | int | `60` | Minimum MIDI velocity (0–127) |
| `-maxvel` | int | `100` | Maximum MIDI velocity (0–127) |
| `-ticks` | int | `480` | Ticks per quarter note (96 \| 480 \| 960) |
| `-out` | string | `output.mid` | Output file path |
| `-seed` | int64 | `0` | Random seed — `0` means random each run |
| `-play` | bool | `false` | Play back via sine synth after generating |

---

## Modes

### `melody`
Generates a single-note melodic line by selecting notes from the scale's note
pool at each rhythmic step. Note lengths vary dynamically — longer notes for
louder velocities, shorter for quieter, with per-note timing jitter to break
mechanical regularity.

```sh
./midi-gen -mode melody -scale blues -root E3 -length 64 -bpm 100 -complexity complex
```

### `chords`
Generates a sequence of chords using a single randomly chosen chord quality.
The `-chordstyle` flag controls how long chords sustain relative to the step grid.

```sh
./midi-gen -mode chords -scale major -root C3 -chordstyle bouncy -bpm 110
```

### `progression`
Generates a diatonic chord progression — chords are built on each degree of
the chosen scale using only in-key notes. The tonic chord (I) is weighted to
appear more frequently than other degrees, producing progressions that feel
harmonically grounded.

```sh
./midi-gen -mode progression -scale dorian -root D3 -complexity medium -bpm 85
```

---

## Scales

| Name | Intervals |
|------|-----------|
| `major` | 0 2 4 5 7 9 11 |
| `minor` | 0 2 3 5 7 8 10 |
| `pentatonic` | 0 2 4 7 9 |
| `blues` | 0 3 5 6 7 10 |
| `dorian` | 0 2 3 5 7 9 10 |
| `phrygian` | 0 1 3 5 7 8 10 |
| `lydian` | 0 2 4 6 7 9 11 |
| `mixolydian` | 0 2 4 5 7 9 10 |
| `harmonicminor` | 0 2 3 5 7 8 11 |
| `wholetone` | 0 2 4 6 8 10 |
| `diminished` | 0 1 3 4 6 7 9 10 |

---

## Chord Styles

Only applies to `chords` and `progression` modes.

| Style | Duration | Use case |
|-------|----------|----------|
| `long` | 85–95% of step | Pads, sustained chords, ambient textures |
| `bouncy` | 25–45% of step | Funk, rhythm guitar, staccato parts |
| `overlap` | unclamped | Lush blurred textures — opt-in only |

`overlap` allows chords to ring into the next step. Use deliberately — it is
not the default because it can produce muddy results at faster tempos.

---

## Complexity

| Level | Note lengths | Rest probability | Chord pool | Phrase |
|-------|-------------|-----------------|------------|--------|
| `simple` | 0.8–2.0× step | 5% | Triads only | 1 bar |
| `medium` | 0.5–2.0× step | 10% | Triads + 7ths | 2 bars |
| `complex` | 0.3–3.0× step | 20% | All 22 types | 2 bars |

---

## Timing Model

Note Ons are always snapped to the step grid (a clean multiple of the quantize
unit in ticks). Note Offs are independent — notes can ring for any duration
including across step boundaries. This means:

- The rhythmic pulse stays locked regardless of note length.
- Notes can overlap the next step when complexity is high or `-chordstyle overlap` is set.
- The same `-seed` always produces byte-identical `.mid` output.

---

## Synth Playback (`-play`)

The built-in synth renders the generated MIDI directly to PCM audio without
requiring an external synthesizer or DAW.

**Signal chain:**

```
MIDI events
    → Scheduler (delta times → sample positions)
    → Voice pool (sine oscillator + ADSR envelope per note)
    → Mono PCM buffer
    → Stereo expansion (L = R)
    → Schroeder reverb (4 comb + 2 allpass filters)
    → int16 PCM bytes
    → oto → CoreAudio → speakers
```

**ADSR envelope** — shaped per complexity level:

| Complexity | Attack | Decay | Sustain | Release |
|-----------|--------|-------|---------|---------|
| `simple` | 5ms | 20ms | 0.8 | 30ms |
| `medium` | 3ms | 15ms | 0.7 | 50ms |
| `complex` | 2ms | 10ms | 0.6 | 80ms |

The synth is intentionally simple — it produces a clean sine wave tone
suitable for auditioning generated patterns before loading into a DAW.

---

## Examples

```sh
# Simple major scale melody, slow and steady
./midi-gen -mode melody -scale major -root C4 -complexity simple -bpm 80 -play

# Minor blues, fast and dense
./midi-gen -mode melody -scale blues -root E3 -complexity complex -bpm 140 -quantize sixteenth -play

# Sustained jazz chords
./midi-gen -mode chords -scale major -root C3 -chordstyle long -complexity medium -bpm 70 -play

# Bouncy funk chords
./midi-gen -mode chords -scale minor -root A2 -chordstyle bouncy -bpm 110 -play

# Diatonic progression in Dorian
./midi-gen -mode progression -scale dorian -root D3 -bpm 90 -complexity medium -play

# Reproducible output — share a seed to share a pattern
./midi-gen -mode melody -scale pentatonic -root G4 -seed 42 -length 32 -out pattern.mid
```

---

## Project Structure

```
midi-gen/
├── main.go                  # CLI flag parsing and dispatch
├── go.mod                   # Module definition
├── go.sum                   # Dependency checksums (commit this)
├── midi/
│   ├── types.go             # MIDI event structs and constructors
│   ├── writer.go            # Binary MIDI serialization
│   ├── types_test.go
│   └── writer_test.go
├── theory/
│   ├── scales.go            # Scale definitions, NoteNumber parser
│   ├── chords.go            # Chord builders and inversions
│   ├── generator.go         # Melody, chord, and progression generation
│   ├── scales_test.go
│   ├── chords_test.go
│   └── generator_test.go
├── synth/
│   ├── sine.go              # Sine oscillator and ADSR envelope
│   ├── reverb.go            # Schroeder reverb
│   ├── scheduler.go         # MIDI events → PCM buffer
│   ├── output.go            # oto audio output
│   ├── sine_test.go
│   ├── reverb_test.go
│   ├── scheduler_test.go
│   └── output_test.go
├── PLAN.md                  # Implementation decisions and architecture
└── .gitignore
```

---

## Running Tests

```sh
go test ./...          # all packages
go test ./midi/...     # MIDI writer only
go test ./theory/...   # music theory only
go test ./synth/...    # synth only
go test ./... -v       # verbose output
```

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/ebitengine/oto/v3` | v3.3.2 | Audio output via CoreAudio (macOS) |

All MIDI generation, music theory, and audio rendering is zero-dependency pure Go.