package crypto

import (
	"encoding/binary"
	"math/bits"
)

// leaKeySchedule expands a 32-byte key into 192 uint32 round keys for LEA-256.
func leaKeySchedule(key [32]byte) [192]uint32 {
	var T [8]uint32
	for i := 0; i < 8; i++ {
		T[i] = binary.LittleEndian.Uint32(key[i*4:])
	}

	var rk [192]uint32
	shifts := [6]uint{1, 3, 6, 11, 13, 17}

	for i := uint32(0); i < 32; i++ {
		d := LEAKeyDelta[i&7]
		s := (i * 6) & 7

		for j := uint32(0); j < 6; j++ {
			idx := (s + j) & 7
			T[idx] = bits.RotateLeft32(T[idx]+bits.RotateLeft32(d, int(i+j)), int(shifts[j]))
		}
		for j := uint32(0); j < 6; j++ {
			rk[i*6+j] = T[(s+j)&7]
		}
	}
	return rk
}

// DecryptLEA decrypts data in 16-byte blocks using LEA-256 ECB mode.
// The input length must be a multiple of 16.
func DecryptLEA(data []byte, key [32]byte) []byte {
	rk := leaKeySchedule(key)
	out := make([]byte, len(data))

	for off := 0; off < len(data); off += 16 {
		block := data[off : off+16]
		s0 := binary.LittleEndian.Uint32(block[0:])
		s1 := binary.LittleEndian.Uint32(block[4:])
		s2 := binary.LittleEndian.Uint32(block[8:])
		s3 := binary.LittleEndian.Uint32(block[12:])

		for r := 31; r >= 0; r-- {
			base := r * 6
			rk0 := rk[base]
			rk1 := rk[base+1]
			rk2 := rk[base+2]
			rk3 := rk[base+3]
			rk4 := rk[base+4]
			rk5 := rk[base+5]

			t0 := s3
			t1 := bits.RotateLeft32(s0, -9) - (t0 ^ rk0) ^ rk1
			t2 := bits.RotateLeft32(s1, 5) - (t1 ^ rk2) ^ rk3
			t3 := bits.RotateLeft32(s2, 3) - (t2 ^ rk4) ^ rk5

			s0 = t0
			s1 = t1
			s2 = t2
			s3 = t3
		}

		binary.LittleEndian.PutUint32(out[off+0:], s0)
		binary.LittleEndian.PutUint32(out[off+4:], s1)
		binary.LittleEndian.PutUint32(out[off+8:], s2)
		binary.LittleEndian.PutUint32(out[off+12:], s3)
	}
	return out
}
