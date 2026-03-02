# Feature Plan: Custom Progression Specification

## Overview

Add a `-prog` flag that lets the user specify a chord progression explicitly
rather than relying on random generation. The progression works across all
three modes (`melody`, `chords`, `progression`) and controls which chords are
active at each point in the sequence.

A companion `-chordrate` flag controls how many steps each chord occupies
before moving to the next.

---

## New Flags

```
-prog       string   Space-separated chord list. Omit for random behaviour (existing default).
                     Examples:
                       "1 4 5 1"           scale degrees
                       "C F G C"           note names (octave inferred from -root)
                       "C:maj7 F:min G:dom7 C"  note names with explicit quality
                       "1 4:min7 5 1"      scale degrees with explicit quality

-chordrate  string   How many steps each chord occupies before advancing.
                     beat  — one beat worth of steps (e.g. 2 steps at eighth quantize)
                     bar   — one full bar of steps  (e.g. 8 steps at eighth quantize)
                     Default: bar
```

---

## Input Format

Each token in `-prog` is one chord. Tokens are separated by spaces.

### Scale degrees (`1`–`7`)

A single integer from 1 to 7 selects the note at that position in the current
scale (1-indexed). The root note for the chord is `ScaleNotes[degree-1]`.

```
"1 4 5 1"   →  I  IV  V  I  (classic cadence)
"1 6 4 5"   →  I  vi  IV  V (pop progression)
"2 5 1"     →  ii  V  I    (jazz cadence)
```

Quality is inferred from the scale unless overridden with `:quality`.

### Note names (`C`, `F#`, `Bb`, etc.)

A note name without an octave number. The octave is inferred from the `-root`
flag — the chord root is placed in the same octave as the root note, or the
nearest octave above if the named note is below the root pitch class.

```
"C F G C"     →  C  F  G  C  (major scale I IV V I from C)
"A C E G"     →  A  C  E  G  (ascending by thirds)
```

Quality is inferred from the scale unless overridden with `:quality`.

### Explicit quality override (`:quality`)

Any token can include a `:quality` suffix to override the inferred chord quality.
The quality must be a key in `ChordIntervals` (e.g. `major`, `minor`, `maj7`,
`dom7`, `min7`, `dim`, `aug`, `sus4`, etc.).

```
"1 4:maj7 5:dom7 1"
"C:maj7 F:min G:dom7 C"
"2:min7 5:dom7 1:maj7"   (ii-V-I in jazz voicing)
```

If the quality suffix is omitted, quality is inferred diatonically from the
scale (same logic as the existing `generateProgression` function).

---

## Chord Rate

`-chordrate` determines how many steps each chord in the progression occupies
before advancing to the next chord. After the last chord, the progression
cycles back to the first.

Step counts per chord by quantize and chordrate:

| Quantize | `beat` | `bar` |
|----------|--------|-------|
| `quarter` | 1 step | 4 steps |
| `eighth` | 2 steps | 8 steps |
| `sixteenth` | 4 steps | 16 steps |

Formula:
```
stepsPerBeat = 1                          (quarter = 1 beat per step)
stepsPerBeat = 2                          (eighth  = 2 steps per beat)
stepsPerBeat = 4                          (sixteenth = 4 steps per beat)

chordrate=beat → stepsPerChord = stepsPerBeat
chordrate=bar  → stepsPerChord = stepsPerBeat * 4  (4/4 time assumed)
```

---

## Behaviour Per Mode

### `melody` with `-prog`

The progression defines a harmonic map over the sequence. At each step, the
active chord is determined by which progression slot the step falls in.

The note pool for that step is filtered to only notes that belong to the
active chord's tones (root, third, fifth, seventh etc). This makes the melody
follow the harmonic rhythm of the progression — notes will outline the chord
changes rather than drawing freely from the full scale.

If a step's filtered note pool is empty (e.g. the chord has no tones in the
current octave range), it falls back to the full scale pool for that step.

```
-prog "1 4 5 1" -chordrate bar -mode melody
→ bar 1: melody uses notes from chord I
→ bar 2: melody uses notes from chord IV
→ bar 3: melody uses notes from chord V
→ bar 4: melody uses notes from chord I
→ repeats if -length exceeds 4 bars
```

### `chords` with `-prog`

Each chord slot plays the specified chord for `stepsPerChord` steps, then
advances. The `-chordstyle` flag still controls note duration within each step.

```
-prog "1 4 5 1" -chordrate bar -mode chords
→ 8 steps of chord I (one bar at eighth quantize)
→ 8 steps of chord IV
→ 8 steps of chord V
→ 8 steps of chord I
→ repeats
```

### `progression` with `-prog`

Same as `chords` but with inversions applied based on complexity and the
existing diatonic voice-leading logic. The `-prog` flag controls which chords
appear and in what order; the `progression` mode controls how they are voiced.

---

## Implementation Plan

### 1. `theory/progression.go` (new file)

**Types:**

```go
// ProgChord represents one parsed chord in a user-specified progression.
type ProgChord struct {
    Root    int     // MIDI note number of the chord root
    Quality string  // chord quality key from ChordIntervals
}
```

**Functions:**

```go
// ParseProgression parses a -prog string into a []ProgChord.
// rootNote and scaleName are used for degree/name resolution and quality inference.
// octave is used when resolving bare note names without an octave number.
func ParseProgression(prog string, rootNote int, scaleName string) ([]ProgChord, error)

// parseDegree resolves a scale degree token ("1"–"7") to a ProgChord.
func parseDegree(token string, degree int, scaleNotes []int, scaleSet map[int]bool) (ProgChord, error)

// parseNoteName resolves a note name token ("C", "F#", "Bb") to a ProgChord.
// Infers octave from rootNote.
func parseNoteName(token string, rootNote int, scaleSet map[int]bool) (ProgChord, error)

// inferQuality determines major or minor quality from whether the major third
// above the root exists in the scale. Upgrades to maj7/min7 at medium/complex
// complexity. Same logic as generateProgression.
func inferQuality(root int, scaleSet map[int]bool, complexity string) string

// StepsPerChord returns the number of generator steps each chord occupies.
func StepsPerChord(chordRate string, quantize string) (int, error)
```

### 2. `GeneratorConfig` additions (`theory/generator.go`)

```go
Progression []ProgChord  // nil = random (existing behaviour preserved)
ChordRate   string       // "beat" | "bar" — default "bar"
```

### 3. Generator changes (`theory/generator.go`)

- `generateMelody`: when `cfg.Progression != nil`, build a per-step chord
  index from `StepsPerChord`, filter the note pool to chord tones at each step.
- `generateChords`: when `cfg.Progression != nil`, cycle through the progression
  using the chord index instead of random root selection.
- `generateProgression`: when `cfg.Progression != nil`, use the parsed chords
  instead of diatonic random selection, still applying inversions.
- `validateConfig`: validate `ChordRate` when `Progression` is set.

### 4. `main.go` additions

```go
prog      := flag.String("prog", "", "chord progression e.g. \"1 4 5 1\" or \"C F G C\"")
chordRate := flag.String("chordrate", "bar", "chord duration: beat | bar")
```

Parse `-prog` via `theory.ParseProgression` after flag parsing. If the result
is non-nil, set `cfg.Progression`. Pass `*chordRate` as `cfg.ChordRate`.

### 5. `theory/progression_test.go` (new file)

Key test cases:
- Degree parsing: `"1 4 5 1"` produces correct roots from C major scale
- Note name parsing: `"C F G C"` resolves to correct MIDI note numbers
- Quality override: `"1 4:maj7 5:dom7 1"` preserves explicit qualities
- Quality inference: degree on a major scale degree gets `major`, minor degree gets `minor`
- Mixed formats rejected: `"1 F 5"` mixing degrees and notes returns an error
- Invalid degree: `"8"` or `"0"` returns an error
- Invalid quality: `"1:superchord"` returns an error
- `StepsPerChord`: all quantize × chordrate combinations produce correct counts
- Cycling: progression repeats correctly when length exceeds chord count
- Empty prog string returns nil, not an error

---

## Examples

```sh
# Classic I-IV-V-I, one bar each, chords mode
./midi-gen -mode chords -scale major -root C3 -prog "1 4 5 1" -chordrate bar -chordstyle long

# ii-V-I jazz cadence with explicit 7th chords
./midi-gen -mode chords -scale major -root C3 -prog "2:min7 5:dom7 1:maj7" -chordrate bar

# Melody that follows a I-VI-IV-V progression
./midi-gen -mode melody -scale major -root C4 -prog "1 6 4 5" -chordrate bar -length 32

# Same progression, one chord per beat instead of per bar
./midi-gen -mode chords -scale major -root C3 -prog "1 4 5 1" -chordrate beat -chordstyle bouncy

# Note names instead of degrees
./midi-gen -mode progression -scale major -root C3 -prog "C F G C" -chordrate bar

# Mixed with explicit quality on the V chord
./midi-gen -mode chords -scale major -root C3 -prog "C F G:dom7 C" -chordrate bar
```

---

## What Does Not Change

- Omitting `-prog` preserves all existing random behaviour exactly.
- `-chordrate` is ignored when `-prog` is not specified.
- All existing flags, modes, complexity levels, and chord styles continue to
  work as before.
- The `-seed` flag still produces deterministic output when `-prog` is set
  (the RNG is still used for velocity, jitter, and rests).