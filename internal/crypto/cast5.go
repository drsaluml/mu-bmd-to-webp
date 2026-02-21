package crypto

import (
	"encoding/binary"
	"math/bits"
)

// cast5Cipher implements CAST5 (CAST-128) decryption per RFC 2144.
// 8-byte block, 16-byte key, 16 rounds.
type cast5Cipher struct {
	kr [17]int32  // rotate subkeys, 1-based
	km [17]uint32 // masking subkeys, 1-based
	rounds int
}

func newCAST5(key []byte) blockCipher {
	c := &cast5Cipher{}
	c.setKey(key[:16])
	return c
}

func (c *cast5Cipher) BlockSize() int { return 8 }

func (c *cast5Cipher) DecryptBlock(src, dst []byte) {
	li := binary.BigEndian.Uint32(src[0:])
	ri := binary.BigEndian.Uint32(src[4:])

	for i := c.rounds; i > 0; i-- {
		lp := li
		rp := ri
		li = rp

		switch i {
		case 1, 4, 7, 10, 13, 16:
			ri = lp ^ cast5F1(rp, c.km[i], c.kr[i])
		case 2, 5, 8, 11, 14:
			ri = lp ^ cast5F2(rp, c.km[i], c.kr[i])
		case 3, 6, 9, 12, 15:
			ri = lp ^ cast5F3(rp, c.km[i], c.kr[i])
		}
	}

	binary.BigEndian.PutUint32(dst[0:], ri)
	binary.BigEndian.PutUint32(dst[4:], li)
}

func cast5F1(d, kmi uint32, kri int32) uint32 {
	i := bits.RotateLeft32(kmi+d, int(kri))
	return ((cast5S1[(i>>24)&0xFF] ^ cast5S2[(i>>16)&0xFF]) - cast5S3[(i>>8)&0xFF]) + cast5S4[i&0xFF]
}

func cast5F2(d, kmi uint32, kri int32) uint32 {
	i := bits.RotateLeft32(kmi^d, int(kri))
	return ((cast5S1[(i>>24)&0xFF] - cast5S2[(i>>16)&0xFF]) + cast5S3[(i>>8)&0xFF]) ^ cast5S4[i&0xFF]
}

func cast5F3(d, kmi uint32, kri int32) uint32 {
	i := bits.RotateLeft32(kmi-d, int(kri))
	return ((cast5S1[(i>>24)&0xFF] + cast5S2[(i>>16)&0xFF]) ^ cast5S3[(i>>8)&0xFF]) - cast5S4[i&0xFF]
}

func cast5IntsTo32(b []int, i int) uint32 {
	return uint32((b[i]&0xFF)<<24 | (b[i+1]&0xFF)<<16 | (b[i+2]&0xFF)<<8 | (b[i+3] & 0xFF))
}

func cast5Bits32ToInts(v uint32, b []int, off int) {
	b[off+3] = int(v & 0xFF)
	b[off+2] = int((v >> 8) & 0xFF)
	b[off+1] = int((v >> 16) & 0xFF)
	b[off] = int((v >> 24) & 0xFF)
}

func (c *cast5Cipher) setKey(key []byte) {
	c.rounds = 16
	if len(key) < 11 {
		c.rounds = 12
	}

	z := make([]int, 16)
	x := make([]int, 16)
	for i := 0; i < len(key); i++ {
		x[i] = int(key[i] & 0xFF)
	}

	x03 := cast5IntsTo32(x, 0x0)
	x47 := cast5IntsTo32(x, 0x4)
	x8b := cast5IntsTo32(x, 0x8)
	xcf := cast5IntsTo32(x, 0xC)

	z03 := x03 ^ cast5S5[x[0xD]] ^ cast5S6[x[0xF]] ^ cast5S7[x[0xC]] ^ cast5S8[x[0xE]] ^ cast5S7[x[0x8]]
	cast5Bits32ToInts(z03, z, 0x0)
	z47 := x8b ^ cast5S5[z[0x0]] ^ cast5S6[z[0x2]] ^ cast5S7[z[0x1]] ^ cast5S8[z[0x3]] ^ cast5S8[x[0xA]]
	cast5Bits32ToInts(z47, z, 0x4)
	z8b := xcf ^ cast5S5[z[0x7]] ^ cast5S6[z[0x6]] ^ cast5S7[z[0x5]] ^ cast5S8[z[0x4]] ^ cast5S5[x[0x9]]
	cast5Bits32ToInts(z8b, z, 0x8)
	zcf := x47 ^ cast5S5[z[0xA]] ^ cast5S6[z[0x9]] ^ cast5S7[z[0xB]] ^ cast5S8[z[0x8]] ^ cast5S6[x[0xB]]
	cast5Bits32ToInts(zcf, z, 0xC)

	c.km[1] = cast5S5[z[0x8]] ^ cast5S6[z[0x9]] ^ cast5S7[z[0x7]] ^ cast5S8[z[0x6]] ^ cast5S5[z[0x2]]
	c.km[2] = cast5S5[z[0xA]] ^ cast5S6[z[0xB]] ^ cast5S7[z[0x5]] ^ cast5S8[z[0x4]] ^ cast5S6[z[0x6]]
	c.km[3] = cast5S5[z[0xC]] ^ cast5S6[z[0xD]] ^ cast5S7[z[0x3]] ^ cast5S8[z[0x2]] ^ cast5S7[z[0x9]]
	c.km[4] = cast5S5[z[0xE]] ^ cast5S6[z[0xF]] ^ cast5S7[z[0x1]] ^ cast5S8[z[0x0]] ^ cast5S8[z[0xC]]

	z03 = cast5IntsTo32(z, 0x0)
	z47 = cast5IntsTo32(z, 0x4)
	z8b = cast5IntsTo32(z, 0x8)
	zcf = cast5IntsTo32(z, 0xC)

	x03 = z8b ^ cast5S5[z[0x5]] ^ cast5S6[z[0x7]] ^ cast5S7[z[0x4]] ^ cast5S8[z[0x6]] ^ cast5S7[z[0x0]]
	cast5Bits32ToInts(x03, x, 0x0)
	x47 = z03 ^ cast5S5[x[0x0]] ^ cast5S6[x[0x2]] ^ cast5S7[x[0x1]] ^ cast5S8[x[0x3]] ^ cast5S8[z[0x2]]
	cast5Bits32ToInts(x47, x, 0x4)
	x8b = z47 ^ cast5S5[x[0x7]] ^ cast5S6[x[0x6]] ^ cast5S7[x[0x5]] ^ cast5S8[x[0x4]] ^ cast5S5[z[0x1]]
	cast5Bits32ToInts(x8b, x, 0x8)
	xcf = zcf ^ cast5S5[x[0xA]] ^ cast5S6[x[0x9]] ^ cast5S7[x[0xB]] ^ cast5S8[x[0x8]] ^ cast5S6[z[0x3]]
	cast5Bits32ToInts(xcf, x, 0xC)

	c.km[5] = cast5S5[x[0x3]] ^ cast5S6[x[0x2]] ^ cast5S7[x[0xC]] ^ cast5S8[x[0xD]] ^ cast5S5[x[0x8]]
	c.km[6] = cast5S5[x[0x1]] ^ cast5S6[x[0x0]] ^ cast5S7[x[0xE]] ^ cast5S8[x[0xF]] ^ cast5S6[x[0xD]]
	c.km[7] = cast5S5[x[0x7]] ^ cast5S6[x[0x6]] ^ cast5S7[x[0x8]] ^ cast5S8[x[0x9]] ^ cast5S7[x[0x3]]
	c.km[8] = cast5S5[x[0x5]] ^ cast5S6[x[0x4]] ^ cast5S7[x[0xA]] ^ cast5S8[x[0xB]] ^ cast5S8[x[0x7]]

	x03 = cast5IntsTo32(x, 0x0)
	x47 = cast5IntsTo32(x, 0x4)
	x8b = cast5IntsTo32(x, 0x8)
	xcf = cast5IntsTo32(x, 0xC)

	z03 = x03 ^ cast5S5[x[0xD]] ^ cast5S6[x[0xF]] ^ cast5S7[x[0xC]] ^ cast5S8[x[0xE]] ^ cast5S7[x[0x8]]
	cast5Bits32ToInts(z03, z, 0x0)
	z47 = x8b ^ cast5S5[z[0x0]] ^ cast5S6[z[0x2]] ^ cast5S7[z[0x1]] ^ cast5S8[z[0x3]] ^ cast5S8[x[0xA]]
	cast5Bits32ToInts(z47, z, 0x4)
	z8b = xcf ^ cast5S5[z[0x7]] ^ cast5S6[z[0x6]] ^ cast5S7[z[0x5]] ^ cast5S8[z[0x4]] ^ cast5S5[x[0x9]]
	cast5Bits32ToInts(z8b, z, 0x8)
	zcf = x47 ^ cast5S5[z[0xA]] ^ cast5S6[z[0x9]] ^ cast5S7[z[0xB]] ^ cast5S8[z[0x8]] ^ cast5S6[x[0xB]]
	cast5Bits32ToInts(zcf, z, 0xC)

	c.km[9] = cast5S5[z[0x3]] ^ cast5S6[z[0x2]] ^ cast5S7[z[0xC]] ^ cast5S8[z[0xD]] ^ cast5S5[z[0x9]]
	c.km[10] = cast5S5[z[0x1]] ^ cast5S6[z[0x0]] ^ cast5S7[z[0xE]] ^ cast5S8[z[0xF]] ^ cast5S6[z[0xC]]
	c.km[11] = cast5S5[z[0x7]] ^ cast5S6[z[0x6]] ^ cast5S7[z[0x8]] ^ cast5S8[z[0x9]] ^ cast5S7[z[0x2]]
	c.km[12] = cast5S5[z[0x5]] ^ cast5S6[z[0x4]] ^ cast5S7[z[0xA]] ^ cast5S8[z[0xB]] ^ cast5S8[z[0x6]]

	z03 = cast5IntsTo32(z, 0x0)
	z47 = cast5IntsTo32(z, 0x4)
	z8b = cast5IntsTo32(z, 0x8)
	zcf = cast5IntsTo32(z, 0xC)

	x03 = z8b ^ cast5S5[z[0x5]] ^ cast5S6[z[0x7]] ^ cast5S7[z[0x4]] ^ cast5S8[z[0x6]] ^ cast5S7[z[0x0]]
	cast5Bits32ToInts(x03, x, 0x0)
	x47 = z03 ^ cast5S5[x[0x0]] ^ cast5S6[x[0x2]] ^ cast5S7[x[0x1]] ^ cast5S8[x[0x3]] ^ cast5S8[z[0x2]]
	cast5Bits32ToInts(x47, x, 0x4)
	x8b = z47 ^ cast5S5[x[0x7]] ^ cast5S6[x[0x6]] ^ cast5S7[x[0x5]] ^ cast5S8[x[0x4]] ^ cast5S5[z[0x1]]
	cast5Bits32ToInts(x8b, x, 0x8)
	xcf = zcf ^ cast5S5[x[0xA]] ^ cast5S6[x[0x9]] ^ cast5S7[x[0xB]] ^ cast5S8[x[0x8]] ^ cast5S6[z[0x3]]
	cast5Bits32ToInts(xcf, x, 0xC)

	c.km[13] = cast5S5[x[0x8]] ^ cast5S6[x[0x9]] ^ cast5S7[x[0x7]] ^ cast5S8[x[0x6]] ^ cast5S5[x[0x3]]
	c.km[14] = cast5S5[x[0xA]] ^ cast5S6[x[0xB]] ^ cast5S7[x[0x5]] ^ cast5S8[x[0x4]] ^ cast5S6[x[0x7]]
	c.km[15] = cast5S5[x[0xC]] ^ cast5S6[x[0xD]] ^ cast5S7[x[0x3]] ^ cast5S8[x[0x2]] ^ cast5S7[x[0x8]]
	c.km[16] = cast5S5[x[0xE]] ^ cast5S6[x[0xF]] ^ cast5S7[x[0x1]] ^ cast5S8[x[0x0]] ^ cast5S8[x[0xD]]

	// Rotation subkeys
	x03 = cast5IntsTo32(x, 0x0)
	x47 = cast5IntsTo32(x, 0x4)
	x8b = cast5IntsTo32(x, 0x8)
	xcf = cast5IntsTo32(x, 0xC)

	z03 = x03 ^ cast5S5[x[0xD]] ^ cast5S6[x[0xF]] ^ cast5S7[x[0xC]] ^ cast5S8[x[0xE]] ^ cast5S7[x[0x8]]
	cast5Bits32ToInts(z03, z, 0x0)
	z47 = x8b ^ cast5S5[z[0x0]] ^ cast5S6[z[0x2]] ^ cast5S7[z[0x1]] ^ cast5S8[z[0x3]] ^ cast5S8[x[0xA]]
	cast5Bits32ToInts(z47, z, 0x4)
	z8b = xcf ^ cast5S5[z[0x7]] ^ cast5S6[z[0x6]] ^ cast5S7[z[0x5]] ^ cast5S8[z[0x4]] ^ cast5S5[x[0x9]]
	cast5Bits32ToInts(z8b, z, 0x8)
	zcf = x47 ^ cast5S5[z[0xA]] ^ cast5S6[z[0x9]] ^ cast5S7[z[0xB]] ^ cast5S8[z[0x8]] ^ cast5S6[x[0xB]]
	cast5Bits32ToInts(zcf, z, 0xC)

	c.kr[1] = int32((cast5S5[z[0x8]] ^ cast5S6[z[0x9]] ^ cast5S7[z[0x7]] ^ cast5S8[z[0x6]] ^ cast5S5[z[0x2]]) & 0x1F)
	c.kr[2] = int32((cast5S5[z[0xA]] ^ cast5S6[z[0xB]] ^ cast5S7[z[0x5]] ^ cast5S8[z[0x4]] ^ cast5S6[z[0x6]]) & 0x1F)
	c.kr[3] = int32((cast5S5[z[0xC]] ^ cast5S6[z[0xD]] ^ cast5S7[z[0x3]] ^ cast5S8[z[0x2]] ^ cast5S7[z[0x9]]) & 0x1F)
	c.kr[4] = int32((cast5S5[z[0xE]] ^ cast5S6[z[0xF]] ^ cast5S7[z[0x1]] ^ cast5S8[z[0x0]] ^ cast5S8[z[0xC]]) & 0x1F)

	z03 = cast5IntsTo32(z, 0x0)
	z47 = cast5IntsTo32(z, 0x4)
	z8b = cast5IntsTo32(z, 0x8)
	zcf = cast5IntsTo32(z, 0xC)

	x03 = z8b ^ cast5S5[z[0x5]] ^ cast5S6[z[0x7]] ^ cast5S7[z[0x4]] ^ cast5S8[z[0x6]] ^ cast5S7[z[0x0]]
	cast5Bits32ToInts(x03, x, 0x0)
	x47 = z03 ^ cast5S5[x[0x0]] ^ cast5S6[x[0x2]] ^ cast5S7[x[0x1]] ^ cast5S8[x[0x3]] ^ cast5S8[z[0x2]]
	cast5Bits32ToInts(x47, x, 0x4)
	x8b = z47 ^ cast5S5[x[0x7]] ^ cast5S6[x[0x6]] ^ cast5S7[x[0x5]] ^ cast5S8[x[0x4]] ^ cast5S5[z[0x1]]
	cast5Bits32ToInts(x8b, x, 0x8)
	xcf = zcf ^ cast5S5[x[0xA]] ^ cast5S6[x[0x9]] ^ cast5S7[x[0xB]] ^ cast5S8[x[0x8]] ^ cast5S6[z[0x3]]
	cast5Bits32ToInts(xcf, x, 0xC)

	c.kr[5] = int32((cast5S5[x[0x3]] ^ cast5S6[x[0x2]] ^ cast5S7[x[0xC]] ^ cast5S8[x[0xD]] ^ cast5S5[x[0x8]]) & 0x1F)
	c.kr[6] = int32((cast5S5[x[0x1]] ^ cast5S6[x[0x0]] ^ cast5S7[x[0xE]] ^ cast5S8[x[0xF]] ^ cast5S6[x[0xD]]) & 0x1F)
	c.kr[7] = int32((cast5S5[x[0x7]] ^ cast5S6[x[0x6]] ^ cast5S7[x[0x8]] ^ cast5S8[x[0x9]] ^ cast5S7[x[0x3]]) & 0x1F)
	c.kr[8] = int32((cast5S5[x[0x5]] ^ cast5S6[x[0x4]] ^ cast5S7[x[0xA]] ^ cast5S8[x[0xB]] ^ cast5S8[x[0x7]]) & 0x1F)

	x03 = cast5IntsTo32(x, 0x0)
	x47 = cast5IntsTo32(x, 0x4)
	x8b = cast5IntsTo32(x, 0x8)
	xcf = cast5IntsTo32(x, 0xC)

	z03 = x03 ^ cast5S5[x[0xD]] ^ cast5S6[x[0xF]] ^ cast5S7[x[0xC]] ^ cast5S8[x[0xE]] ^ cast5S7[x[0x8]]
	cast5Bits32ToInts(z03, z, 0x0)
	z47 = x8b ^ cast5S5[z[0x0]] ^ cast5S6[z[0x2]] ^ cast5S7[z[0x1]] ^ cast5S8[z[0x3]] ^ cast5S8[x[0xA]]
	cast5Bits32ToInts(z47, z, 0x4)
	z8b = xcf ^ cast5S5[z[0x7]] ^ cast5S6[z[0x6]] ^ cast5S7[z[0x5]] ^ cast5S8[z[0x4]] ^ cast5S5[x[0x9]]
	cast5Bits32ToInts(z8b, z, 0x8)
	zcf = x47 ^ cast5S5[z[0xA]] ^ cast5S6[z[0x9]] ^ cast5S7[z[0xB]] ^ cast5S8[z[0x8]] ^ cast5S6[x[0xB]]
	cast5Bits32ToInts(zcf, z, 0xC)

	c.kr[9] = int32((cast5S5[z[0x3]] ^ cast5S6[z[0x2]] ^ cast5S7[z[0xC]] ^ cast5S8[z[0xD]] ^ cast5S5[z[0x9]]) & 0x1F)
	c.kr[10] = int32((cast5S5[z[0x1]] ^ cast5S6[z[0x0]] ^ cast5S7[z[0xE]] ^ cast5S8[z[0xF]] ^ cast5S6[z[0xC]]) & 0x1F)
	c.kr[11] = int32((cast5S5[z[0x7]] ^ cast5S6[z[0x6]] ^ cast5S7[z[0x8]] ^ cast5S8[z[0x9]] ^ cast5S7[z[0x2]]) & 0x1F)
	c.kr[12] = int32((cast5S5[z[0x5]] ^ cast5S6[z[0x4]] ^ cast5S7[z[0xA]] ^ cast5S8[z[0xB]] ^ cast5S8[z[0x6]]) & 0x1F)

	z03 = cast5IntsTo32(z, 0x0)
	z47 = cast5IntsTo32(z, 0x4)
	z8b = cast5IntsTo32(z, 0x8)
	zcf = cast5IntsTo32(z, 0xC)

	x03 = z8b ^ cast5S5[z[0x5]] ^ cast5S6[z[0x7]] ^ cast5S7[z[0x4]] ^ cast5S8[z[0x6]] ^ cast5S7[z[0x0]]
	cast5Bits32ToInts(x03, x, 0x0)
	x47 = z03 ^ cast5S5[x[0x0]] ^ cast5S6[x[0x2]] ^ cast5S7[x[0x1]] ^ cast5S8[x[0x3]] ^ cast5S8[z[0x2]]
	cast5Bits32ToInts(x47, x, 0x4)
	x8b = z47 ^ cast5S5[x[0x7]] ^ cast5S6[x[0x6]] ^ cast5S7[x[0x5]] ^ cast5S8[x[0x4]] ^ cast5S5[z[0x1]]
	cast5Bits32ToInts(x8b, x, 0x8)
	xcf = zcf ^ cast5S5[x[0xA]] ^ cast5S6[x[0x9]] ^ cast5S7[x[0xB]] ^ cast5S8[x[0x8]] ^ cast5S6[z[0x3]]
	cast5Bits32ToInts(xcf, x, 0xC)

	c.kr[13] = int32((cast5S5[x[0x8]] ^ cast5S6[x[0x9]] ^ cast5S7[x[0x7]] ^ cast5S8[x[0x6]] ^ cast5S5[x[0x3]]) & 0x1F)
	c.kr[14] = int32((cast5S5[x[0xA]] ^ cast5S6[x[0xB]] ^ cast5S7[x[0x5]] ^ cast5S8[x[0x4]] ^ cast5S6[x[0x7]]) & 0x1F)
	c.kr[15] = int32((cast5S5[x[0xC]] ^ cast5S6[x[0xD]] ^ cast5S7[x[0x3]] ^ cast5S8[x[0x2]] ^ cast5S7[x[0x8]]) & 0x1F)
	c.kr[16] = int32((cast5S5[x[0xE]] ^ cast5S6[x[0xF]] ^ cast5S7[x[0x1]] ^ cast5S8[x[0x0]] ^ cast5S8[x[0xD]]) & 0x1F)
}
