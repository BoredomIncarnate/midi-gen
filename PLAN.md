# MIDI Generator CLI — Implementation Plan

## Overview

A Go CLI tool that generates random chords/melodies and outputs `.mid` files. Built from scratch with minimal dependencies — the MIDI writer will be pure Go, and audio playback will use a single lightweight library.

---

## Architecture

```
midi-gen/
├── main.go              # CLI entry point (flag parsing, dispatch)
├── midi/
│   ├── writer.go        # Raw MIDI file serialization (no libs)
│   └── types.go         # MIDI structs: File, Track, Event
├── theory/
│   ├── scales.go        # Scale definitions, note lookup tables
│   ├── chords.go        # Chord builders (triads, 7ths, extensions)
│   └── generator.go     # Random melody/chord progression logic
├── synth/
│   ├── sine.go          # Sine wave oscillator
│   ├── scheduler.go     # MIDI event → audio timing engine
│   └── output.go        # Audio device output (oto)
└── go.mod
```

---

## Phase 1 — MIDI File Writer (Pure Go)

### The Binary Format

A `.mid` file is a well-documented binary format. You write it byte by byte:

**Header Chunk** (14 bytes always):
```
4D546864  → "MThd" magic bytes
00000006  → chunk length = 6
0001      → format type (0=single track, 1=multi-track)
0001      → number of tracks
0060      → ticks per quarter note (96 is common)
```

**Track Chunk**:
```
4D54726B  → "MTrk" magic bytes
XXXXXXXX  → chunk length (computed after events are serialized)
[events]
FF 2F 00  → End of Track meta-event (required)
```

**Events** use *variable-length delta times* (the main gotcha). Each event is preceded by how many ticks since the last event. Delta time encoding: 7 bits per byte, MSB=1 means "more bytes follow", MSB=0 is last byte. Example: 0 = `0x00`, 96 = `0x60`, 128 = `0x81 0x00`.

**Note events** (the two you'll use most):
```
Note On:  [delta] 9n kk vv   (n=channel, kk=key 0-127, vv=velocity)
Note Off: [delta] 8n kk vv
```

**`midi/types.go`** — define these structs:
```go
type Event struct {
    Delta   uint32  // ticks since last event
    Data    []byte  // raw MIDI bytes
}

type Track struct {
    Events []Event
}

type File struct {
    Format     uint16
    TicksPerQN uint16
    Tracks     []Track
}
```

**`midi/writer.go`** — key functions to implement:
- `encodeVarLen(n uint32) []byte` — the variable-length encoding
- `(f *File) Serialize() []byte` — walks the struct, writes chunks
- `WriteFile(path string, f *File) error` — calls Serialize, writes to disk

This is ~100 lines of pure Go. No external packages needed.

---

## Phase 2 — Music Theory Engine

### Note Representation

Use MIDI note numbers (0–127). Middle C = 60.

```go
// In scales.go
var noteNames = []string{"C","C#","D","D#","E","F","F#","G","G#","A","A#","B"}

func NoteNumber(name string, octave int) int {
    // returns e.g. NoteNumber("C", 4) → 60
}
```

**Scale definitions** — store as semitone intervals from root:
```go
var Scales = map[string][]int{
    "major":          {0,2,4,5,7,9,11},
    "minor":          {0,2,3,5,7,8,10},
    "pentatonic":     {0,2,4,7,9},
    "blues":          {0,3,5,6,7,10},
    "dorian":         {0,2,3,5,7,9,10},
}
```

**`chords.go`** — build chords from a root + quality:
```go
var ChordIntervals = map[string][]int{
    "major":   {0,4,7},
    "minor":   {0,3,7},
    "dom7":    {0,4,7,10},
    "maj7":    {0,4,7,11},
    "min7":    {0,3,7,10},
    "dim":     {0,3,6},
    "aug":     {0,4,8},
    "sus2":    {0,2,7},
    "sus4":    {0,5,7},
}

func BuildChord(root int, quality string) []int {
    // returns slice of MIDI note numbers
}
```

**`generator.go`** — the randomization layer:
```go
type GeneratorConfig struct {
    Scale      string
    RootNote   int     // e.g. 60 for C4
    Octaves    int     // range to span
    Length     int     // number of notes/chords
    MinVel     int
    MaxVel     int
    Complexity string  // "simple" | "medium" | "complex"
    Mode       string  // "melody" | "chords" | "progression"
    BPM        int
    Quantize   string  // "quarter" | "eighth" | "sixteenth"
}
```

Complexity controls things like: note density, chord extension depth (triads vs 7ths vs 9ths), rhythmic variation (straight vs syncopated deltas), velocity humanization range.

---

## Phase 3 — CLI Interface

Use only `flag` from stdlib. Keep it simple:

```
midi-gen \
  -mode melody \
  -scale minor \
  -root C4 \
  -length 16 \
  -bpm 120 \
  -complexity medium \
  -quantize eighth \
  -out output.mid
```

`main.go` parses flags → builds `GeneratorConfig` → calls generator → calls `midi.WriteFile`. Add a `-seed` flag for reproducible outputs.

---

## Phase 4 — Sine Synth Playback (MVP+)

**The one external dependency**: [`oto`](https://github.com/ebitengine/oto) by the Ebitengine team. It's the thinnest possible cross-platform audio output layer — gives you a `Write([]byte)` interface for raw PCM samples. No higher-level abstractions.

```
go get github.com/ebitengine/oto/v3
```

**`synth/sine.go`** — oscillator:
```go
const SampleRate = 44100

func MIDINoteToFreq(note int) float64 {
    return 440.0 * math.Pow(2, float64(note-69)/12.0)
}

// Generates N samples of a sine wave at given freq + amplitude
func GenerateSine(freq, amplitude float64, numSamples int) []float32 { ... }
```

**`synth/scheduler.go`** — the bridge between MIDI events and audio:

This is the interesting part. Walk through your `[]Event` list, convert delta ticks to real time using BPM + TicksPerQN, maintain a list of "active notes", and fill PCM buffers tick by tick. For each moment in time, sum all active oscillators. Apply a simple ADSR envelope (at minimum: a short linear fade-in/out to avoid clicks).

```go
type ActiveNote struct {
    Freq      float64
    Phase     float64  // current sine phase, advances each sample
    Remaining int      // samples left to play
}
```

**`synth/output.go`** — wire it to oto:
```go
func Play(ctx *oto.Context, events []midi.Event, bpm int, tpqn int) {
    player := ctx.NewPlayer(...)
    // feed PCM buffers from scheduler
}
```

Add a `-play` flag to `main.go` that skips file output and pipes directly to the synth instead (or does both).

---

## Build Order

| Step | What you're building | Testable when done? |
|------|---------------------|---------------------|
| 1 | `midi/types.go` + `writer.go` | Yes — open output in GarageBand/MIDI viewer |
| 2 | `theory/scales.go` + `chords.go` | Yes — unit test note numbers |
| 3 | `theory/generator.go` | Yes — full CLI works, output .mid files |
| 4 | `synth/sine.go` | Yes — generate a test tone to file |
| 5 | `synth/scheduler.go` | Yes — play a hardcoded C major scale |
| 6 | Wire `-play` flag end-to-end | Yes — full tool complete |

---

## Key Gotchas to Watch For

- **Variable-length encoding** is where most people get stuck. Write unit tests for it immediately with values: 0, 127, 128, 255, 16383, 16384.
- **Track length field** must be computed *after* all events are serialized, then backfilled into bytes 4–7 of the track header.
- **End of Track** meta-event (`FF 2F 00`) is mandatory or DAWs will reject the file.
- **Note Off timing**: you must emit a Note Off event for every Note On. Store scheduled-off events in a priority queue or sort events by absolute tick before serializing.
- **Sine clicks**: without at minimum a 5–10ms fade at note boundaries, every note transition will click. Even a simple linear ramp is enough for MVP.
- **oto context**: create it once at startup, not per-note. It's expensive to initialize.

---

## Go Module Setup

```
go mod init midi-gen
go get github.com/ebitengine/oto/v3   # only added for Phase 4
```

Total external dependencies for the full tool: **1** (oto). MIDI file generation is zero-dependency.
