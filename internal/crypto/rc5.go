package crypto

import (
	"encoding/binary"
	"math/bits"
)

// rc5Cipher implements RC5 (w=32, r=16) decryption.
// 8-byte block, 16-byte key.
type rc5Cipher struct {
	s []uint32
}

func newRC5(key []byte) blockCipher {
	c := &rc5Cipher{}
	c.expandKey(key[:16])
	return c
}

func (c *rc5Cipher) BlockSize() int { return 8 }

func (c *rc5Cipher) DecryptBlock(src, dst []byte) {
	const r = 16
	s := c.s

	a := binary.LittleEndian.Uint32(src[0:])
	b := binary.LittleEndian.Uint32(src[4:])

	for i := r; i >= 1; i-- {
		b = bits.RotateLeft32(b-s[2*i+1], -int(a&31)) ^ a
		a = bits.RotateLeft32(a-s[2*i], -int(b&31)) ^ b
	}

	b -= s[1]
	a -= s[0]

	binary.LittleEndian.PutUint32(dst[0:], a)
	binary.LittleEndian.PutUint32(dst[4:], b)
}

func (c *rc5Cipher) expandKey(key []byte) {
	const (
		r    = 16
		p32  = uint32(0xB7E15163)
		q32  = uint32(0x9E3779B9)
	)

	keyLen := len(key)
	cw := keyLen / 4
	if cw < 1 {
		cw = 1
	}
	L := make([]uint32, cw)
	for i := keyLen - 1; i >= 0; i-- {
		L[i/4] = (L[i/4] << 8) + uint32(key[i])
	}

	sLen := 2 * (r + 1)
	c.s = make([]uint32, sLen)
	c.s[0] = p32
	for i := 1; i < sLen; i++ {
		c.s[i] = c.s[i-1] + q32
	}

	var a, b uint32
	ii, jj := 0, 0
	v := 3 * sLen
	if 3*cw > v {
		v = 3 * cw
	}
	for s := 0; s < v; s++ {
		a = bits.RotateLeft32(c.s[ii]+a+b, 3)
		c.s[ii] = a
		b = bits.RotateLeft32(L[jj]+a+b, int((a+b)&31))
		L[jj] = b
		ii = (ii + 1) % sLen
		jj = (jj + 1) % cw
	}
}
