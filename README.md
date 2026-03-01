# midi-gen

A CLI tool for generating random MIDI chords and melodies, written in Go with minimal dependencies.

## Status

🚧 In progress — see [PLAN.md](./PLAN.md) for the full implementation plan.

## Structure

```
midi-gen/
├── main.go              # CLI entry point
├── midi/
│   ├── types.go         # MIDI file structs
│   └── writer.go        # Raw MIDI binary serialization
├── theory/
│   ├── scales.go        # Scale definitions and note lookup
│   ├── chords.go        # Chord builders
│   └── generator.go     # Random generation logic
├── synth/
│   ├── sine.go          # Sine wave oscillator
│   ├── scheduler.go     # MIDI → PCM timing engine
│   └── output.go        # Audio output via oto
└── PLAN.md              # Full implementation plan
```

## Usage (planned)

```sh
midi-gen -mode melody -scale minor -root C4 -length 16 -bpm 120 -out output.mid
midi-gen -mode chords -scale major -root G3 -complexity complex -play
```

## Dependencies

- Phase 1–3: zero external dependencies
- Phase 4 (synth playback): `github.com/ebitengine/oto/v3`

## Setup

```sh
git clone <your-repo>
cd midi-gen
go build ./...
```
