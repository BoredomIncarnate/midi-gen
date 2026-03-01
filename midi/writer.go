package midi

import (
	"encoding/binary"
	"os"
)

// encodeVarLen encodes a uint32 as a MIDI variable-length quantity.
func encodeVarLen(n uint32) []byte {
	// TODO: implement 7-bit-per-byte encoding, MSB=1 means more bytes follow
	_ = n
	return nil
}

// Serialize encodes the File into raw MIDI bytes.
func (f *File) Serialize() []byte {
	// TODO:
	// 1. Write MThd header (magic, length=6, format, ntracks, ticks)
	// 2. For each track, serialize events, wrap in MTrk chunk
	_ = binary.BigEndian // will be used for uint16/uint32 writes
	return nil
}

// WriteFile serializes f and writes it to the given path.
func WriteFile(path string, f *File) error {
	data := f.Serialize()
	return os.WriteFile(path, data, 0644)
}
