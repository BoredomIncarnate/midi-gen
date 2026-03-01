package main

import (
	"flag"
	"fmt"
	"os"

	"midi-gen/midi"
	"midi-gen/theory"
)

func main() {
	// --- CLI flags ---
	//
	// All flags map 1:1 to fields in theory.GeneratorConfig.
	// Defaults are chosen to produce a pleasant result with no flags specified.
	mode := flag.String("mode", "melody",
		"generation mode: melody | chords | progression")

	scale := flag.String("scale", "major",
		"scale to draw notes from: major | minor | pentatonic | blues | dorian | phrygian | lydian | mixolydian | harmonicminor | wholetone | diminished")

	root := flag.String("root", "C4",
		"root note of the scale/chord, e.g. C4, F#3, Bb2")

	octaves := flag.Int("octaves", 2,
		"number of octaves to span for the note pool (min 1)")

	length := flag.Int("length", 16,
		"number of notes (melody) or chords (chords/progression) to generate")

	bpm := flag.Int("bpm", 120,
		"tempo in beats per minute (1–300)")

	complexity := flag.String("complexity", "medium",
		"complexity level: simple | medium | complex")

	quantize := flag.String("quantize", "eighth",
		"rhythmic grid: quarter | eighth | sixteenth")

	chordStyle := flag.String("chordstyle", "long",
		"chord duration style (chords/progression modes only): long | bouncy | overlap")

	minVel := flag.Int("minvel", 60,
		"minimum MIDI velocity for generated notes (0–127)")

	maxVel := flag.Int("maxvel", 100,
		"maximum MIDI velocity for generated notes (0–127)")

	ticksPerQN := flag.Int("ticks", 480,
		"MIDI timing resolution: ticks per quarter note (96 | 480 | 960)")

	out := flag.String("out", "output.mid",
		"output .mid file path")

	seed := flag.Int64("seed", 0,
		"random seed for reproducible output (0 = random each run)")

	// play flag is declared here for future Phase 4 use.
	// Parsed but not yet acted on — the synth package is not yet wired up.
	play := flag.Bool("play", false,
		"[Phase 4] play back via sine synth after generating (not yet implemented)")

	flag.Usage = usage
	flag.Parse()

	// --- Resolve root note string → MIDI note number ---
	//
	// The CLI accepts human-readable notation ("C4", "F#3", "Bb2").
	// theory.NoteNumber converts this to the integer the generator needs.
	rootNote, err := theory.NoteNumber(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid root note %q: %v\n", *root, err)
		os.Exit(1)
	}

	// --- Build GeneratorConfig ---
	cfg := theory.GeneratorConfig{
		Scale:      *scale,
		RootNote:   rootNote,
		Octaves:    *octaves,
		Length:     *length,
		MinVel:     *minVel,
		MaxVel:     *maxVel,
		Complexity: *complexity,
		Mode:       *mode,
		BPM:        *bpm,
		TicksPerQN: *ticksPerQN,
		Quantize:   *quantize,
		ChordStyle: *chordStyle,
		Seed:       *seed,
	}

	// --- Generate ---
	fmt.Printf("generating: mode=%s scale=%s root=%s bpm=%d complexity=%s length=%d\n",
		*mode, *scale, *root, *bpm, *complexity, *length)

	track, err := theory.Generate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generation failed: %v\n", err)
		os.Exit(1)
	}

	// --- Build MIDI file ---
	//
	// We use Format 0 (single track) for maximum compatibility.
	// Every DAW and hardware device supports Format 0.
	// Format 1 (multi-track) would be needed if we ever separate melody,
	// bass, and drums onto individual tracks.
	f := &midi.File{
		Format:     0,
		TicksPerQN: uint16(*ticksPerQN),
		Tracks:     []midi.Track{track},
	}

	// --- Write file ---
	if err := midi.WriteFile(*out, f); err != nil {
		fmt.Fprintf(os.Stderr, "error: could not write %s: %v\n", *out, err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s\n", *out)

	// --- Phase 4: playback ---
	// synth.Play() will be wired here once synth package is implemented.
	if *play {
		fmt.Fprintln(os.Stderr, "note: -play is not yet implemented (Phase 4)")
	}
}

// usage prints a friendly help message when -h or --help is passed.
func usage() {
	fmt.Fprintf(os.Stderr, `midi-gen — random MIDI melody and chord generator

USAGE:
  midi-gen [flags]

FLAGS:
  -mode       string   melody | chords | progression  (default: melody)
  -scale      string   major | minor | pentatonic | blues | dorian |
                       phrygian | lydian | mixolydian | harmonicminor |
                       wholetone | diminished         (default: major)
  -root       string   root note, e.g. C4 F#3 Bb2    (default: C4)
  -octaves    int      note pool range in octaves     (default: 2)
  -length     int      notes or chords to generate   (default: 16)
  -bpm        int      tempo in BPM                  (default: 120)
  -complexity string   simple | medium | complex      (default: medium)
  -quantize   string   quarter | eighth | sixteenth   (default: eighth)
  -minvel     int      minimum velocity 0–127         (default: 60)
  -maxvel     int      maximum velocity 0–127         (default: 100)
  -ticks      int      ticks per quarter note         (default: 480)
  -out        string   output file path               (default: output.mid)
  -chordstyle string   long | bouncy | overlap         (default: long)
  -seed       int64    random seed (0 = random)       (default: 0)
  -play                play via sine synth [Phase 4]  (default: false)

EXAMPLES:
  midi-gen -mode melody -scale minor -root A3 -length 32 -out melody.mid
  midi-gen -mode progression -scale dorian -root D4 -bpm 95 -complexity complex
  midi-gen -mode chords -scale major -root G3 -seed 42 -out repro.mid

`)
}
