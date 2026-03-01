package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	mode := flag.String("mode", "melody", "generation mode: melody | chords | progression")
	scale := flag.String("scale", "major", "scale: major | minor | pentatonic | blues | dorian")
	root := flag.String("root", "C4", "root note, e.g. C4, A3, F#4")
	length := flag.Int("length", 16, "number of notes or chords to generate")
	bpm := flag.Int("bpm", 120, "tempo in beats per minute")
	complexity := flag.String("complexity", "medium", "complexity: simple | medium | complex")
	quantize := flag.String("quantize", "eighth", "quantize: quarter | eighth | sixteenth")
	out := flag.String("out", "output.mid", "output .mid file path")
	seed := flag.Int64("seed", 0, "random seed (0 = random)")
	play := flag.Bool("play", false, "play back via sine synth instead of (or in addition to) writing file")

	flag.Parse()

	fmt.Printf("midi-gen\n")
	fmt.Printf("  mode:       %s\n", *mode)
	fmt.Printf("  scale:      %s\n", *scale)
	fmt.Printf("  root:       %s\n", *root)
	fmt.Printf("  length:     %d\n", *length)
	fmt.Printf("  bpm:        %d\n", *bpm)
	fmt.Printf("  complexity: %s\n", *complexity)
	fmt.Printf("  quantize:   %s\n", *quantize)
	fmt.Printf("  out:        %s\n", *out)
	fmt.Printf("  seed:       %d\n", *seed)
	fmt.Printf("  play:       %v\n", *play)

	// TODO Phase 2: build GeneratorConfig and call theory.Generate()
	// TODO Phase 1: call midi.WriteFile()
	// TODO Phase 4: call synth.Play() if -play flag set

	fmt.Fprintln(os.Stderr, "not yet implemented")
	os.Exit(1)
}
