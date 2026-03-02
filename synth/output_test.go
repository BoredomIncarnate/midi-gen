package synth

import (
	"io"
	"math"
	"testing"
)

// The oto-dependent Play() function requires audio hardware and cannot be
// tested in a unit test environment. All other exported functions in output.go
// (FloatToInt16, StereoFloatToBytes, normaliseGain) are pure functions that
// operate only on memory and are fully testable here.

// -----------------------------------------------------------------------------
// FloatToInt16 tests
// -----------------------------------------------------------------------------

// TestFloatToInt16_Zero verifies that a float32 value of 0.0 maps to int16 0.
// This is the silence value — it must be exact to prevent DC offset.
func TestFloatToInt16_Zero(t *testing.T) {
	got := FloatToInt16(0.0)
	if got != 0 {
		t.Errorf("FloatToInt16(0.0): expected 0, got %d", got)
	}
}

// TestFloatToInt16_PositiveFull verifies that 1.0 maps to int16 max (32767).
// int16 positive maximum is 32767 (not 32768) due to two's complement asymmetry.
func TestFloatToInt16_PositiveFull(t *testing.T) {
	got := FloatToInt16(1.0)
	if got != 32767 {
		t.Errorf("FloatToInt16(1.0): expected 32767, got %d", got)
	}
}

// TestFloatToInt16_NegativeFull verifies that -1.0 maps to int16 min (-32768).
// The negative range of int16 extends one further than positive (-32768 vs +32767).
func TestFloatToInt16_NegativeFull(t *testing.T) {
	got := FloatToInt16(-1.0)
	if got != -32768 {
		t.Errorf("FloatToInt16(-1.0): expected -32768, got %d", got)
	}
}

// TestFloatToInt16_Midpoint verifies that 0.5 maps to approximately 16383
// (half of 32767), confirming linear scaling in the positive range.
func TestFloatToInt16_Midpoint(t *testing.T) {
	got := FloatToInt16(0.5)
	expected := int16(16383) // 0.5 * 32767 = 16383.5, truncated to 16383
	// Allow ±1 for integer truncation
	diff := int(got) - int(expected)
	if diff > 1 || diff < -1 {
		t.Errorf("FloatToInt16(0.5): expected ~%d, got %d", expected, got)
	}
}

// TestFloatToInt16_NegativeMidpoint verifies that -0.5 maps to approximately
// -16383, confirming linear scaling in the negative range.
func TestFloatToInt16_NegativeMidpoint(t *testing.T) {
	got := FloatToInt16(-0.5)
	expected := int16(-16384) // -0.5 * 32768 = -16384 (asymmetric negative scaling)
	diff := int(got) - int(expected)
	if diff > 1 || diff < -1 {
		t.Errorf("FloatToInt16(-0.5): expected ~%d, got %d", expected, got)
	}
}

// TestFloatToInt16_ClampAboveOne verifies that values above 1.0 are clamped
// to int16 max (32767) rather than wrapping or overflowing.
// Values > 1.0 can appear when many voices are summed with insufficient masterGain.
func TestFloatToInt16_ClampAboveOne(t *testing.T) {
	values := []float32{1.001, 1.5, 2.0, 100.0}
	for _, v := range values {
		got := FloatToInt16(v)
		if got != 32767 {
			t.Errorf("FloatToInt16(%f): expected clamped value 32767, got %d", v, got)
		}
	}
}

// TestFloatToInt16_ClampBelowNegativeOne verifies that values below -1.0 are
// clamped to int16 min (-32768).
func TestFloatToInt16_ClampBelowNegativeOne(t *testing.T) {
	values := []float32{-1.001, -1.5, -2.0, -100.0}
	for _, v := range values {
		got := FloatToInt16(v)
		if got != -32768 {
			t.Errorf("FloatToInt16(%f): expected clamped value -32768, got %d", v, got)
		}
	}
}

// TestFloatToInt16_Monotonic verifies that FloatToInt16 is monotonically
// non-decreasing — a larger float32 input always produces a >= int16 output.
// A non-monotonic conversion would introduce distortion at amplitude boundaries.
func TestFloatToInt16_Monotonic(t *testing.T) {
	values := []float32{-1.0, -0.75, -0.5, -0.25, 0.0, 0.25, 0.5, 0.75, 1.0}
	for i := 1; i < len(values); i++ {
		prev := FloatToInt16(values[i-1])
		curr := FloatToInt16(values[i])
		if curr < prev {
			t.Errorf("not monotonic at index %d: FloatToInt16(%f)=%d > FloatToInt16(%f)=%d",
				i, values[i-1], prev, values[i], curr)
		}
	}
}

// -----------------------------------------------------------------------------
// StereoFloatToBytes tests
// -----------------------------------------------------------------------------

// TestStereoFloatToBytes_Length verifies the output byte slice is exactly
// 2 bytes per float32 sample (one int16 per sample).
func TestStereoFloatToBytes_Length(t *testing.T) {
	stereo := []float32{0.1, -0.1, 0.5, -0.5}
	out := StereoFloatToBytes(stereo)
	expected := len(stereo) * 2
	if len(out) != expected {
		t.Errorf("expected %d bytes, got %d", expected, len(out))
	}
}

// TestStereoFloatToBytes_Empty verifies that an empty input produces an empty
// byte slice without panicking.
func TestStereoFloatToBytes_Empty(t *testing.T) {
	out := StereoFloatToBytes([]float32{})
	if len(out) != 0 {
		t.Errorf("expected empty output, got %d bytes", len(out))
	}
}

// TestStereoFloatToBytes_LittleEndian verifies the byte order of a known value.
// float32(1.0) → int16(32767) → little-endian bytes [0xFF, 0x7F]
//
// Little-endian means the least significant byte comes first:
//
//	32767 = 0x7FFF → bytes [0xFF, 0x7F]
func TestStereoFloatToBytes_LittleEndian(t *testing.T) {
	out := StereoFloatToBytes([]float32{1.0})
	if len(out) < 2 {
		t.Fatal("output too short")
	}
	// 32767 = 0x7FFF in little-endian = [0xFF, 0x7F]
	if out[0] != 0xFF || out[1] != 0x7F {
		t.Errorf("little-endian encoding: expected [0xFF, 0x7F], got [0x%02X, 0x%02X]",
			out[0], out[1])
	}
}

// TestStereoFloatToBytes_ZeroSilence verifies that a silent sample (0.0)
// encodes to two zero bytes [0x00, 0x00].
func TestStereoFloatToBytes_ZeroSilence(t *testing.T) {
	out := StereoFloatToBytes([]float32{0.0})
	if out[0] != 0x00 || out[1] != 0x00 {
		t.Errorf("silence should encode to [0x00, 0x00], got [0x%02X, 0x%02X]",
			out[0], out[1])
	}
}

// TestStereoFloatToBytes_RoundTrip verifies that the byte encoding can be
// decoded back to the original int16 value. This confirms the encoding is
// correct and lossless at the int16 precision level.
func TestStereoFloatToBytes_RoundTrip(t *testing.T) {
	inputs := []float32{0.0, 0.5, -0.5, 1.0, -1.0, 0.123}
	out := StereoFloatToBytes(inputs)

	for i, f := range inputs {
		// Decode the two bytes back to int16 (little-endian)
		lo := uint16(out[i*2])
		hi := uint16(out[i*2+1])
		decoded := int16(lo | (hi << 8))

		// Re-encode the original float to compare
		expected := FloatToInt16(f)

		if decoded != expected {
			t.Errorf("sample %d (float=%f): expected int16=%d, decoded=%d",
				i, f, expected, decoded)
		}
	}
}

// -----------------------------------------------------------------------------
// normaliseGain tests
// -----------------------------------------------------------------------------

// TestNormaliseGain_PeakReachesTarget verifies that after normalisation,
// the maximum absolute sample value equals the target peak.
func TestNormaliseGain_PeakReachesTarget(t *testing.T) {
	buf := []float32{0.1, -0.3, 0.2, 0.5, -0.4}
	normaliseGain(buf, 0.9)

	peak := float32(0)
	for _, s := range buf {
		abs := float32(math.Abs(float64(s)))
		if abs > peak {
			peak = abs
		}
	}

	if math.Abs(float64(peak)-0.9) > 0.001 {
		t.Errorf("expected peak=0.9 after normalisation, got %.4f", peak)
	}
}

// TestNormaliseGain_RelativeDynamicsPreserved verifies that normalisation
// scales all samples uniformly — the ratio between any two samples is unchanged.
func TestNormaliseGain_RelativeDynamicsPreserved(t *testing.T) {
	buf := []float32{0.2, 0.4, 0.6}
	original := make([]float32, len(buf))
	copy(original, buf)

	normaliseGain(buf, 0.9)

	// Ratio of first two samples should be preserved
	origRatio := float64(original[0]) / float64(original[1])
	newRatio := float64(buf[0]) / float64(buf[1])

	if math.Abs(origRatio-newRatio) > 0.0001 {
		t.Errorf("relative dynamics changed: original ratio %.4f, new ratio %.4f",
			origRatio, newRatio)
	}
}

// TestNormaliseGain_SilentBufferUnchanged verifies that a buffer of all zeros
// is not modified (no division by zero or NaN introduction).
func TestNormaliseGain_SilentBufferUnchanged(t *testing.T) {
	buf := []float32{0.0, 0.0, 0.0}
	normaliseGain(buf, 0.9)
	for i, s := range buf {
		if s != 0.0 {
			t.Errorf("sample %d: silent buffer was modified to %f", i, s)
		}
	}
}

// TestNormaliseGain_ScalesUpQuietBuffer verifies that a buffer whose peak is
// below targetPeak is scaled up so the peak reaches the target.
// normaliseGain always normalises to targetPeak regardless of direction.
func TestNormaliseGain_ScalesUpQuietBuffer(t *testing.T) {
	buf := []float32{0.1, -0.2, 0.15} // peak = 0.2, target = 0.9
	normaliseGain(buf, 0.9)

	peak := float32(0)
	for _, s := range buf {
		if s < 0 {
			s = -s
		}
		if s > peak {
			peak = s
		}
	}
	if peak < 0.89 || peak > 0.91 {
		t.Errorf("expected peak ~0.9 after scaling up, got %.4f", peak)
	}
}

// TestNormaliseGain_NegativePeakHandled verifies that a buffer whose largest
// absolute value is negative is normalised correctly.
// e.g. buf = {-0.8, 0.3} → peak=0.8, scale = 0.9/0.8 = 1.125 → {-0.9, 0.3375}
func TestNormaliseGain_NegativePeakHandled(t *testing.T) {
	buf := []float32{-0.8, 0.3}
	normaliseGain(buf, 0.9)

	peak := float32(0)
	for _, s := range buf {
		abs := float32(math.Abs(float64(s)))
		if abs > peak {
			peak = abs
		}
	}

	if math.Abs(float64(peak)-0.9) > 0.001 {
		t.Errorf("expected peak=0.9 when largest value is negative, got %.4f", peak)
	}
}

// -----------------------------------------------------------------------------
// bytesReader tests
// -----------------------------------------------------------------------------

// TestBytesReader_ReadsAllBytes verifies that repeated Read calls eventually
// return all bytes from the source slice.
func TestBytesReader_ReadsAllBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	r := newBytesReader(data)

	out := make([]byte, 0, len(data))
	buf := make([]byte, 2)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}

	if len(out) != len(data) {
		t.Errorf("expected to read %d bytes, got %d", len(data), len(out))
	}
	for i := range data {
		if out[i] != data[i] {
			t.Errorf("byte %d: expected 0x%02X, got 0x%02X", i, data[i], out[i])
		}
	}
}

// TestBytesReader_EOFOnExhaustion verifies that Read returns an error (EOF)
// when all bytes have been consumed.
func TestBytesReader_EOFOnExhaustion(t *testing.T) {
	r := newBytesReader([]byte{0x01})
	buf := make([]byte, 10)

	// First read: consumes the one byte
	r.Read(buf)

	// Second read: should return io.EOF
	n, err := r.Read(buf)
	if err != io.EOF {
		t.Errorf("expected io.EOF after exhaustion, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes on EOF, got %d", n)
	}
}

// TestBytesReader_EmptySlice verifies that reading from an empty byte slice
// returns EOF immediately on the first Read call.
func TestBytesReader_EmptySlice(t *testing.T) {
	r := newBytesReader([]byte{})
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != io.EOF {
		t.Errorf("expected io.EOF for empty slice, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes from empty slice, got %d", n)
	}
}

// TestDefaultPlayOptions verifies the default PlayOptions have sensible values.
// masterGain=0.3 and reverbWetMix=0.3 are safe defaults for the typical
// polyphony ranges the generator produces.
func TestDefaultPlayOptions(t *testing.T) {
	opts := DefaultPlayOptions()
	if opts.MasterGain <= 0 || opts.MasterGain > 1.0 {
		t.Errorf("MasterGain %.2f outside (0, 1]", opts.MasterGain)
	}
	if opts.ReverbDryMix <= 0 || opts.ReverbDryMix > 1.0 {
		t.Errorf("ReverbDryMix %.2f outside (0, 1]", opts.ReverbDryMix)
	}
	if opts.ReverbWetMix < 0 || opts.ReverbWetMix > 1.0 {
		t.Errorf("ReverbWetMix %.2f outside [0, 1]", opts.ReverbWetMix)
	}
	if opts.Complexity == "" {
		t.Error("Complexity should not be empty")
	}
}
