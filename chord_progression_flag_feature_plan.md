# Feature Plan: Custom Progression Specification

## Status

- `theory/progression.go` — complete ✅
- `theory/progression_test.go` — complete ✅
- `GeneratorConfig` additions — pending
- Generator function changes — pending
- `main.go` wiring — pending

---

## Overview

A `-prog` flag lets the user specify a chord progression explicitly rather than
relying on random generation. The progression works across all three modes
(`melody`, `chords`, `progression`) and controls which chords are active at
each point in the sequence.

A companion `-chordrate` flag controls how many steps each chord occupies
before moving to the next.

---

## New Flags

```
-prog       string   Space-separated chord list. Omit for random behaviour (existing default).
                     Examples:
                       "1 4 5 1"                    scale degrees
                       "C F G C"                    note names (octave inferred from -root)
                       "C:maj7 F:minor G:dom7 C"    note names with explicit quality
                       "2:min7 5:dom7 1:maj7"       scale degrees with explicit quality

-chordrate  string   How many steps each chord occupies before advancing.
                     beat  — one beat worth of steps (e.g. 2 steps at eighth quantize)
                     bar   — one full bar of steps  (e.g. 8 steps at eighth quantize)
                     Default: bar
```

---

## Input Format

Each token in `-prog` is one chord separated by spaces. All tokens must use
the same format — mixing scale degrees and note names in one string returns
an error.

### Scale degrees (`1`–`7`)

A single integer from 1 to 7 selects the note at that position in the scale
(1-indexed). The root note for the chord is `ScaleNotes[degree-1]`.

```
"1 4 5 1"   →  I  IV  V  I   (classic cadence)
"1 6 4 5"   →  I  vi  IV  V  (pop progression)
"2 5 1"     →  ii  V   I     (jazz cadence)
```

Quality is inferred from the scale unless overridden with `:quality`.

Scales with fewer than 7 notes (e.g. pentatonic = 5 notes) only support
degrees 1 through the scale length.

### Note names (`C`, `F#`, `Bb`, etc.)

A note name without an octave number. The octave is inferred from `-root`:
the chord root is placed in the same octave as the root note, or the next
octave up if the named note's pitch class is below the root's pitch class.

```
"C F G C"   →  C  F  G  C  (I IV V I from C)
"A C E G"   →  A  C  E  G  (ascending thirds)
```

Quality is inferred from the scale unless overridden with `:quality`.

### Explicit quality override (`:quality`)

Any token can include a `:quality` suffix to override diatonic inference.
The quality must be a canonical key in `ChordIntervals`:

```
major, minor, dim, aug, sus2, sus4
dom7, maj7, min7, minmaj7, dim7, halfdim7, augmaj7, dom7sus4
maj9, dom9, min9, dom11, maj13
add9, minadd9, maj6, min6
```

```
"1 4:maj7 5:dom7 1"
"C:maj7 F:minor G:dom7 C"
"2:min7 5:dom7 1:maj7"
```

---

## Chord Rate

`-chordrate` determines how many steps each chord occupies before advancing.
After the last chord the progression cycles back to the first.

| Quantize | `beat` | `bar` |
|----------|--------|-------|
| `quarter` | 1 step | 4 steps |
| `eighth` | 2 steps | 8 steps |
| `sixteenth` | 4 steps | 16 steps |

4/4 time is assumed throughout (4 beats per bar).

---

## Behaviour Per Mode

### `melody` with `-prog`

The progression defines a harmonic map over the sequence. At each step the
note pool is filtered to only the tones of the currently active chord. This
makes the melody outline the chord changes rather than drawing freely from
the full scale.

If the filtered pool is empty for a step (chord tones outside the configured
octave range), it falls back to the full scale pool for that step.

```
-prog "1 4 5 1" -chordrate bar -mode melody -quantize eighth
→ steps  0–7:  melody draws from chord I tones
→ steps  8–15: melody draws from chord IV tones
→ steps 16–23: melody draws from chord V tones
→ steps 24–31: melody draws from chord I tones
→ cycles if -length exceeds 32 steps
```

### `chords` with `-prog`

Each chord slot plays the specified chord for `stepsPerChord` steps then
advances. The `-chordstyle` flag still controls note duration within each step.

```
-prog "1 4 5 1" -chordrate bar -mode chords -quantize eighth
→ steps  0–7:  chord I
→ steps  8–15: chord IV
→ steps 16–23: chord V
→ steps 24–31: chord I
→ cycles
```

### `progression` with `-prog`

Same as `chords` but with inversions applied based on complexity. The `-prog`
flag controls which chords appear and in what order; the `progression` mode
controls how they are voiced.

---

## Implementation

### `theory/progression.go` ✅

**Types:**

```go
type ProgChord struct {
    Root    int     // MIDI note number of the chord root
    Quality string  // chord quality key from ChordIntervals
}
```

**Exported functions:**

```go
// ParseProgression parses a -prog flag string into []ProgChord.
// Returns (nil, nil) for empty input — signals random behaviour to generator.
func ParseProgression(prog string, rootNote int, scaleName string, complexity string) ([]ProgChord, error)

// StepsPerChord returns steps per chord for a given chordrate and quantize.
func StepsPerChord(chordRate string, quantize string) (int, error)

// ProgChordAt returns the chord active at a given step, cycling as needed.
func ProgChordAt(prog []ProgChord, step int, stepsPerChord int) ProgChord
```

**Internal functions:**

```go
func splitToken(token string) (base, quality string)
func parseDegree(degree int, qualityOverride string, scaleNotes []int, scaleSet map[int]bool, complexity string) (ProgChord, error)
func parseNoteName(noteName string, qualityOverride string, rootNote int, scaleSet map[int]bool, complexity string) (ProgChord, error)
func inferQuality(root int, scaleSet map[int]bool, complexity string) string
```

### `GeneratorConfig` additions (pending)

```go
Progression []ProgChord  // nil = random (existing behaviour preserved)
ChordRate   string       // "beat" | "bar" — default "bar"
```

### Generator changes (pending)

- `generateMelody`: when `cfg.Progression != nil`, filter note pool to chord
  tones at each step using `ProgChordAt`.
- `generateChords`: when `cfg.Progression != nil`, use `ProgChordAt` instead
  of random root selection.
- `generateProgression`: when `cfg.Progression != nil`, use parsed chords
  instead of diatonic random selection, still applying inversions.
- `validateConfig`: validate `ChordRate` is `"beat"` or `"bar"` when
  `Progression` is non-nil.

### `main.go` additions (pending)

```go
prog      := flag.String("prog", "", `chord progression e.g. "1 4 5 1" or "C F G C"`)
chordRate := flag.String("chordrate", "bar", "chord duration per step: beat | bar")
```

Parse `-prog` via `theory.ParseProgression` after flag parsing. Pass the
result as `cfg.Progression` and `*chordRate` as `cfg.ChordRate`.

---

## Examples

```sh
# Classic I-IV-V-I, one bar each
./midi-gen -mode chords -scale major -root C3 -prog "1 4 5 1" -chordrate bar -chordstyle long

# ii-V-I jazz cadence with explicit 7th chords
./midi-gen -mode chords -scale major -root C3 -prog "2:min7 5:dom7 1:maj7" -chordrate bar

# Melody that follows a I-VI-IV-V progression
./midi-gen -mode melody -scale major -root C4 -prog "1 6 4 5" -chordrate bar -length 32

# One chord per beat for a busier rhythmic feel
./midi-gen -mode chords -scale major -root C3 -prog "1 4 5 1" -chordrate beat -chordstyle bouncy

# Note names instead of degrees
./midi-gen -mode progression -scale major -root C3 -prog "C F G C" -chordrate bar

# Explicit quality on the V chord
./midi-gen -mode chords -scale major -root C3 -prog "C F G:dom7 C" -chordrate bar

# Play back immediately
./midi-gen -mode chords -scale major -root C3 -prog "1 4 5 1" -chordrate bar -play -out prog.mid
```

---

## What Does Not Change

- Omitting `-prog` preserves all existing random behaviour exactly.
- `-chordrate` is ignored when `-prog` is not specified.
- All existing flags, modes, complexity levels, and chord styles work as before.
- The `-seed` flag still produces deterministic output when `-prog` is set —
  the RNG is still used for velocity, jitter, and rests.