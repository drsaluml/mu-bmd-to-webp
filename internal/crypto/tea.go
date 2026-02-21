package crypto

import "encoding/binary"

// teaCipher implements TEA (Tiny Encryption Algorithm) decryption.
// 8-byte block, 16-byte key, 32 rounds. Matches BouncyCastle TeaEngine.
type teaCipher struct {
	k [4]uint32
}

func newTEA(key []byte) blockCipher {
	c := &teaCipher{}
	// Big-endian key loading (matches BouncyCastle Pack.BE_To_UInt32)
	for i := 0; i < 4; i++ {
		c.k[i] = binary.BigEndian.Uint32(key[i*4:])
	}
	return c
}

func (c *teaCipher) BlockSize() int { return 8 }

func (c *teaCipher) DecryptBlock(src, dst []byte) {
	v0 := binary.BigEndian.Uint32(src[0:])
	v1 := binary.BigEndian.Uint32(src[4:])

	k0, k1, k2, k3 := c.k[0], c.k[1], c.k[2], c.k[3]

	const delta = uint32(0x9E3779B9)
	sum := uint32(0xC6EF3720) // delta * 32

	for i := 0; i < 32; i++ {
		v1 -= ((v0 << 4) + k2) ^ (v0 + sum) ^ ((v0 >> 5) + k3)
		v0 -= ((v1 << 4) + k0) ^ (v1 + sum) ^ ((v1 >> 5) + k1)
		sum -= delta
	}

	binary.BigEndian.PutUint32(dst[0:], v0)
	binary.BigEndian.PutUint32(dst[4:], v1)
}
