package synth

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"midi-gen/midi"
	"time"

	"github.com/ebitengine/oto/v3"
)

// outputSampleRate is the PCM sample rate used for audio output.
// Must match SampleRate in sine.go — they are kept as separate constants
// so that output.go and sine.go can be compiled independently.
// Both must be 44100 Hz to match oto's CoreAudio configuration on macOS.
const outputSampleRate = SampleRate

// outputChannels is the number of audio output channels.
// oto on macOS defaults to stereo (2 channels). The Scheduler renders mono
// PCM which RenderStereo expands to stereo before passing to oto.
const outputChannels = 2

// outputBitDepth is the number of bits per sample per channel.
// We use 16-bit signed integer PCM (oto.FormatSignedInt16LE) because it is
// universally supported and provides sufficient dynamic range (96 dB) for
// synthesized audio at the velocities and masterGain values we use.
//
// The float32 samples from Scheduler.Render are converted to int16 by
// FloatToInt16 before being passed to oto. This conversion is lossless
// within the float32 precision at the amplitude ranges we produce.
const outputBitDepth = 16

// otoBytesPerSample is the number of bytes consumed by oto per sample frame.
// With 2 channels at 2 bytes each: 4 bytes per frame.
// oto reads raw bytes from an io.Reader — we must pack float32 → int16 → []byte.
const otoBytesPerSample = outputChannels * (outputBitDepth / 8) // = 4

// PlayOptions configures audio playback behaviour.
// Passed to Play() by main.go alongside the MIDI track.
//
// Memory layout:
//
//	Complexity   string    controls ADSR envelope shape (see ADSRForComplexity)
//	MasterGain   float64   global amplitude scale before output
//	             Prevents clipping when many voices are summed.
//	             Typical: 0.3 for up to ~10 simultaneous voices.
//	ReverbDryMix float32   gain of the dry (direct) signal in the reverb mix
//	ReverbWetMix float32   gain of the wet (reverberant) signal in the reverb mix
type PlayOptions struct {
	Complexity   string
	MasterGain   float64
	ReverbDryMix float32
	ReverbWetMix float32
}

// DefaultPlayOptions returns PlayOptions suitable for most generation configs.
func DefaultPlayOptions() PlayOptions {
	return PlayOptions{
		Complexity:   "medium",
		MasterGain:   0.3,
		ReverbDryMix: 0.7,
		ReverbWetMix: 0.3,
	}
}

// FloatToInt16 converts a float32 PCM sample in [-1.0, 1.0] to a signed
// 16-bit integer in [-32768, 32767].
//
// Conversion formula:
//
//	int16 = clamp(sample * 32767, -32768, 32767)
//
// Clamping is necessary because float32 arithmetic can produce values
// slightly outside [-1.0, 1.0] due to envelope overshoots or summing
// multiple voices without sufficient masterGain headroom.
//
// A sample of exactly 1.0 maps to 32767 (not 32768) because int16 max
// is 32767. This is standard PCM convention — the positive and negative
// ranges are asymmetric by one value.
//
// This function is exported so it can be tested independently without
// requiring an audio device or oto context.
func FloatToInt16(sample float32) int16 {
	// Scale to int16 range using asymmetric scaling.
	// Positive samples scale against 32767 (int16 max).
	// Negative samples scale against 32768 (int16 min magnitude) so that
	// -1.0 maps to exactly -32768 rather than -32767.
	// This matches the two's complement asymmetry of int16: [-32768, 32767].
	var scaled float64
	if sample >= 0 {
		scaled = float64(sample) * 32767.0
	} else {
		scaled = float64(sample) * 32768.0
	}

	// Clamp to valid int16 range
	if scaled > 32767.0 {
		scaled = 32767.0
	} else if scaled < -32768.0 {
		scaled = -32768.0
	}

	// math.Round ensures -32768.0 rounds to -32768 rather than truncating
	// toward zero to -32767, which is what int16(-32768.0) would do in Go.
	return int16(math.Round(scaled))
}

// StereoFloatToBytes converts a stereo float32 PCM buffer into a raw byte
// slice suitable for writing to an oto Player.
//
// Each float32 sample is:
//  1. Converted to int16 via FloatToInt16
//  2. Encoded as little-endian bytes (oto.FormatSignedInt16LE)
//  3. Written as two consecutive bytes into the output slice
//
// The output slice length is exactly len(stereo) * 2 bytes
// (2 bytes per int16 sample).
//
// Little-endian encoding: least significant byte first.
// For example, int16(0x1234) encodes as [0x34, 0x12].
// This matches oto.FormatSignedInt16LE on all platforms.
//
// This function is exported for testing without requiring an audio device.
func StereoFloatToBytes(stereo []float32) []byte {
	// Two bytes per sample (int16 = 2 bytes)
	out := make([]byte, len(stereo)*2)
	for i, s := range stereo {
		// Convert float32 → int16
		v := FloatToInt16(s)
		// Write as little-endian int16 into two consecutive bytes
		// binary.LittleEndian.PutUint16 handles the byte ordering
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}

// Play renders the given MIDI track to PCM, applies reverb, and plays the
// result through the system audio device using oto.
//
// The full rendering pipeline executed by Play:
//
//  1. Build Scheduler from track BPM, TicksPerQN, ADSR, and MasterGain
//  2. Render MIDI events to mono float32 PCM via Scheduler.Render
//  3. Expand mono to stereo via RenderStereo
//  4. Apply Schroeder reverb via Reverb.Process
//  5. Convert float32 stereo to int16 bytes via StereoFloatToBytes
//  6. Initialise oto.Context (CoreAudio on macOS)
//  7. Create an oto.Player and write PCM bytes to it
//  8. Block until playback completes, then close the player
//
// The oto.Context is created fresh on each call. While it is more efficient
// to reuse a context across calls, Play is called at most once per program
// invocation (from main.go after -play is passed), so the overhead of
// context creation is not a concern.
//
// Parameters:
//
//	track      — the generated MIDI track from theory.Generate
//	bpm        — tempo in BPM (must match the track's Tempo meta-event)
//	ticksPerQN — MIDI timing resolution (must match the File.TicksPerQN)
//	opts       — playback configuration (ADSR, gain, reverb mix)
//
// Returns an error if oto context initialisation or player creation fails.
// Audio device unavailability (e.g. no audio hardware) surfaces here.
func Play(track midi.Track, bpm int, ticksPerQN int, opts PlayOptions) error {
	// --- Step 1: resolve ADSR for the requested complexity ---
	adsr := ADSRForComplexity(opts.Complexity)

	// --- Step 2: build scheduler and render mono PCM ---
	sched := NewScheduler(bpm, ticksPerQN, adsr, opts.MasterGain)
	mono := sched.Render(track.Events)
	if len(mono) == 0 {
		// Nothing to play — this is not an error (e.g. all-rest track)
		return nil
	}

	// --- Step 3: expand mono to stereo ---
	// oto expects stereo interleaved PCM. RenderStereo duplicates each
	// mono sample into both L and R channels.
	stereo := RenderStereo(mono)

	// --- Step 4: apply reverb in-place ---
	// Reverb.Process modifies the stereo buffer directly, adding the
	// wet reverberation signal mixed with the dry direct signal.
	reverb := NewReverbWithMix(opts.ReverbDryMix, opts.ReverbWetMix)
	reverb.Process(stereo)

	// --- Step 5: convert float32 stereo to int16 bytes ---
	// oto reads raw bytes from an io.Reader. We convert our float32 samples
	// to little-endian signed 16-bit integers before creating the player.
	pcmBytes := StereoFloatToBytes(stereo)

	// --- Step 6: initialise oto context ---
	// oto.NewContext allocates the CoreAudio session on macOS.
	// The context must remain alive for the duration of playback —
	// it is referenced by the Player and must not be garbage collected.
	//
	// Parameters:
	//   SampleRate:    44100 Hz
	//   ChannelCount:  2 (stereo)
	//   Format:        FormatSignedInt16LE (matches our StereoFloatToBytes output)
	opts2 := &oto.NewContextOptions{
		SampleRate:   outputSampleRate,
		ChannelCount: outputChannels,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, readyChan, err := oto.NewContext(opts2)
	if err != nil {
		return fmt.Errorf("Play: failed to create oto context: %w", err)
	}

	// Wait for the audio context to be ready.
	// On macOS, CoreAudio initialisation is asynchronous — readyChan closes
	// when the context is fully initialised and the audio hardware is reserved.
	// Attempting to play before readyChan closes causes silent failure.
	<-readyChan

	// --- Step 7: create player and write PCM bytes ---
	// oto.Player implements io.Reader — it reads bytes from the slice we provide
	// via a bytes.Reader. We use a simple byte slice reader here.
	player := ctx.NewPlayer(newBytesReader(pcmBytes))

	// Start playback. Play() is non-blocking — it begins feeding bytes to
	// CoreAudio in a background goroutine.
	player.Play()

	// --- Step 8: block until playback is complete ---
	// We poll IsPlaying() at short intervals rather than using a channel or
	// callback because oto's Player interface does not expose a completion event.
	// 10ms polling interval is fine — human perception of playback end
	// is not precise enough to require sub-millisecond resolution here.
	for player.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
	}

	// Close releases the audio hardware reservation and frees CoreAudio resources.
	// Always close — even if the player finished naturally — to prevent resource leaks.
	if err := player.Close(); err != nil && err != io.EOF {
		// io.EOF on close is normal — oto signals clean stream end this way.
		// Any other error indicates a real problem with the audio device.
		return fmt.Errorf("Play: failed to close player: %w", err)
	}

	return nil
}

// bytesReader is a minimal io.Reader over a []byte slice.
// It implements only the Read method needed by oto.Player.
//
// We define this locally rather than using bytes.NewReader to avoid
// importing the bytes package solely for this one use.
//
// Memory layout:
//
//	data []byte   the PCM byte buffer being read
//	pos  int      current read position (advances with each Read call)
type bytesReader struct {
	data []byte
	pos  int
}

// newBytesReader creates a bytesReader over the given byte slice.
func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

// Read implements io.Reader. Copies up to len(p) bytes from the remaining
// data into p, advancing pos. Returns io.EOF when all bytes have been read.
//
// oto calls Read repeatedly in a background goroutine to feed audio hardware.
// When Read returns 0, n and io.EOF, oto stops the player.
func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		// All bytes have been consumed — return the standard io.EOF sentinel.
		// oto checks for io.EOF specifically to detect clean stream completion.
		// A non-standard error here causes oto to treat it as a failure.
		return 0, io.EOF
	}
	// Copy as many bytes as fit in p, up to what remains in data
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// normaliseGain scales a float32 PCM buffer so that the peak absolute
// amplitude equals targetPeak, preventing clipping without changing the
// relative dynamics of the signal.
//
// If the buffer is entirely silent (peak=0), no scaling is applied.
//
// This function is applied before reverb if the rendered PCM has headroom
// issues — for example, when many simultaneous voices are rendered with
// a high masterGain. In normal generation scenarios with masterGain=0.3
// and up to ~8 voices, normalisation is not needed, but it is provided
// for edge cases.
//
// Parameters:
//
//	buf        — float32 buffer to normalise in-place
//	targetPeak — desired maximum absolute amplitude (e.g. 0.9 for -0.9 dB headroom)
func normaliseGain(buf []float32, targetPeak float32) {
	// Find the peak absolute sample value
	peak := float32(0)
	for _, s := range buf {
		abs := float32(math.Abs(float64(s)))
		if abs > peak {
			peak = abs
		}
	}

	// Only skip scaling when the buffer is entirely silent.
	// A peak below targetPeak is still scaled up to targetPeak.
	if peak == 0 {
		return
	}

	// Scale every sample so the peak equals targetPeak
	scale := targetPeak / peak
	for i := range buf {
		buf[i] *= scale
	}
}
