package crypto

import (
	"encoding/binary"
	"math/bits"
)

// threeWayCipher implements 3-Way block cipher decryption.
// 12-byte block, 12-byte key, 11 rounds.
type threeWayCipher struct {
	k [3]uint32
}

func newThreeWay(key []byte) blockCipher {
	c := &threeWayCipher{}
	for i := 0; i < 3; i++ {
		c.k[i] = uint32(key[4*i+3]) |
			uint32(key[4*i+2])<<8 |
			uint32(key[4*i+1])<<16 |
			uint32(key[4*i])<<24
	}
	a0, a1, a2 := c.k[0], c.k[1], c.k[2]
	a0, a1, a2 = twTheta(a0, a1, a2)
	a0, a1, a2 = twMu(a0, a1, a2)
	c.k[0] = twReverseBytes(a0)
	c.k[1] = twReverseBytes(a1)
	c.k[2] = twReverseBytes(a2)
	return c
}

func (c *threeWayCipher) BlockSize() int { return 12 }

func (c *threeWayCipher) DecryptBlock(src, dst []byte) {
	a0 := binary.LittleEndian.Uint32(src[0:])
	a1 := binary.LittleEndian.Uint32(src[4:])
	a2 := binary.LittleEndian.Uint32(src[8:])

	const startD = 0xB1B1
	rc := uint32(startD)

	a0, a1, a2 = twMu(a0, a1, a2)

	for i := 0; i < 11; i++ {
		a0 ^= c.k[0] ^ (rc << 16)
		a1 ^= c.k[1]
		a2 ^= c.k[2] ^ rc
		a0, a1, a2 = twRho(a0, a1, a2)
		rc <<= 1
		if rc&0x10000 != 0 {
			rc ^= 0x11011
		}
		rc &= 0xFFFF
	}

	a0 ^= c.k[0] ^ (rc << 16)
	a1 ^= c.k[1]
	a2 ^= c.k[2] ^ rc
	a0, a1, a2 = twTheta(a0, a1, a2)
	a0, a1, a2 = twMu(a0, a1, a2)

	binary.LittleEndian.PutUint32(dst[0:], a0)
	binary.LittleEndian.PutUint32(dst[4:], a1)
	binary.LittleEndian.PutUint32(dst[8:], a2)
}

func twReverseBytes(x uint32) uint32 {
	return (x&0xFF)<<24 | (x&0xFF00)<<8 | (x&0xFF0000)>>8 | (x&0xFF000000)>>24
}

func twReverseBits(a uint32) uint32 {
	a = ((a & 0xAAAAAAAA) >> 1) | ((a & 0x55555555) << 1)
	a = ((a & 0xCCCCCCCC) >> 2) | ((a & 0x33333333) << 2)
	a = ((a & 0xF0F0F0F0) >> 4) | ((a & 0x0F0F0F0F) << 4)
	return a
}

func twTheta(a0, a1, a2 uint32) (uint32, uint32, uint32) {
	c0 := a0 ^ a1 ^ a2
	c := bits.RotateLeft32(c0, 16) ^ bits.RotateLeft32(c0, 8)
	b0 := (a0 << 24) ^ (a2 >> 8) ^ (a1 << 8) ^ (a0 >> 24)
	b1 := (a1 << 24) ^ (a0 >> 8) ^ (a2 << 8) ^ (a1 >> 24)
	a0 ^= c ^ b0
	a1 ^= c ^ b1
	a2 ^= c ^ ((b0 >> 16) ^ (b1 << 16))
	return a0, a1, a2
}

func twMu(a0, a1, a2 uint32) (uint32, uint32, uint32) {
	a1 = twReverseBits(a1)
	tmp := twReverseBits(a0)
	a0 = twReverseBits(a2)
	a2 = tmp
	return a0, a1, a2
}

func twPiGammaPi(a0, a1, a2 uint32) (uint32, uint32, uint32) {
	b2 := bits.RotateLeft32(a2, 1)
	b0 := bits.RotateLeft32(a0, 22)
	r0 := bits.RotateLeft32(b0^(a1|^b2), 1)
	r2 := bits.RotateLeft32(b2^(b0|^a1), 22)
	r1 := a1 ^ (b2 | ^b0)
	return r0, r1, r2
}

func twRho(a0, a1, a2 uint32) (uint32, uint32, uint32) {
	a0, a1, a2 = twTheta(a0, a1, a2)
	return twPiGammaPi(a0, a1, a2)
}
