package crypto

import "encoding/binary"

// gostCipher implements GOST 28147-89 block cipher decryption.
// 8-byte block, 32-byte key, 32 rounds.
type gostCipher struct {
	decryptSchedule [32]uint32
}

// S-box: 8 rows of 16 entries (4-bit input -> 4-bit output)
var gostSBox = [8][16]byte{
	{4, 10, 9, 2, 13, 8, 0, 14, 6, 11, 1, 12, 7, 15, 5, 3},
	{14, 11, 4, 12, 6, 13, 15, 10, 2, 3, 8, 1, 0, 7, 5, 9},
	{5, 8, 1, 13, 10, 3, 4, 2, 14, 15, 12, 7, 6, 0, 9, 11},
	{7, 13, 10, 1, 0, 8, 9, 15, 14, 4, 6, 12, 11, 2, 5, 3},
	{6, 12, 7, 1, 5, 15, 13, 8, 4, 10, 9, 14, 0, 3, 11, 2},
	{4, 11, 10, 0, 7, 2, 1, 13, 3, 6, 8, 5, 9, 12, 15, 14},
	{13, 11, 4, 1, 3, 15, 5, 9, 0, 10, 14, 7, 6, 8, 2, 12},
	{1, 15, 13, 0, 5, 7, 10, 4, 9, 2, 3, 14, 6, 11, 8, 12},
}

// Pre-computed lookup tables combining pairs of 4-bit S-boxes into 8-bit tables.
var gostLookup [4][256]uint32

func init() {
	for k := 0; k < 4; k++ {
		sLow := gostSBox[2*k]
		sHigh := gostSBox[2*k+1]
		for i := 0; i < 256; i++ {
			lo := uint32(sLow[i&0x0F])
			hi := uint32(sHigh[(i>>4)&0x0F])
			gostLookup[k][i] = (lo | (hi << 4)) << (8 * uint(k))
		}
	}
}

func gostSubstitute(value uint32) uint32 {
	return gostLookup[0][value&0xFF] |
		gostLookup[1][(value>>8)&0xFF] |
		gostLookup[2][(value>>16)&0xFF] |
		gostLookup[3][(value>>24)&0xFF]
}

func newGOST(key []byte) blockCipher {
	c := &gostCipher{}

	var K [8]uint32
	for i := 0; i < 8; i++ {
		K[i] = binary.LittleEndian.Uint32(key[i*4:])
	}

	// Decryption key schedule:
	// Rounds 1-8:  K[0..7]
	// Rounds 9-32: K[7..0] repeated 3 times
	for i := 0; i < 8; i++ {
		c.decryptSchedule[i] = K[i]
	}
	for rep := 0; rep < 3; rep++ {
		for i := 0; i < 8; i++ {
			c.decryptSchedule[8+rep*8+i] = K[7-i]
		}
	}

	return c
}

func (c *gostCipher) BlockSize() int { return 8 }

func (c *gostCipher) DecryptBlock(src, dst []byte) {
	n1 := binary.LittleEndian.Uint32(src[0:])
	n2 := binary.LittleEndian.Uint32(src[4:])

	ks := c.decryptSchedule

	// Rounds 1..31: swap halves
	for i := 0; i < 31; i++ {
		temp := n1 + ks[i]
		substituted := gostSubstitute(temp)
		rotated := (substituted << 11) | (substituted >> 21)
		newN1 := n2 ^ rotated
		n2 = n1
		n1 = newN1
	}

	// Round 32 (last): no swap
	temp := n1 + ks[31]
	substituted := gostSubstitute(temp)
	rotated := (substituted << 11) | (substituted >> 21)
	n2 ^= rotated

	binary.LittleEndian.PutUint32(dst[0:], n1)
	binary.LittleEndian.PutUint32(dst[4:], n2)
}
