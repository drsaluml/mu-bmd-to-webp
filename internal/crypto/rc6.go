package crypto

import (
	"encoding/binary"
	"math/bits"
)

// rc6Cipher implements RC6 (w=32, r=20) decryption.
// 16-byte block, 16-byte key.
type rc6Cipher struct {
	s []uint32
}

func newRC6(key []byte) blockCipher {
	c := &rc6Cipher{}
	c.expandKey(key[:16])
	return c
}

func (c *rc6Cipher) BlockSize() int { return 16 }

func (c *rc6Cipher) DecryptBlock(src, dst []byte) {
	const r = 20
	s := c.s

	a := binary.LittleEndian.Uint32(src[0:])
	b := binary.LittleEndian.Uint32(src[4:])
	cc := binary.LittleEndian.Uint32(src[8:])
	d := binary.LittleEndian.Uint32(src[12:])

	cc -= s[2*r+3]
	a -= s[2*r+2]

	for i := r; i >= 1; i-- {
		// Rotate ABCD right: (A,B,C,D) = (D,A,B,C)
		a, b, cc, d = d, a, b, cc

		u := bits.RotateLeft32(uint32(uint64(d)*uint64(2*d+1)), 5)
		t := bits.RotateLeft32(uint32(uint64(b)*uint64(2*b+1)), 5)
		cc = bits.RotateLeft32(cc-s[2*i+1], -int(t&31)) ^ u
		a = bits.RotateLeft32(a-s[2*i], -int(u&31)) ^ t
	}

	d -= s[1]
	b -= s[0]

	binary.LittleEndian.PutUint32(dst[0:], a)
	binary.LittleEndian.PutUint32(dst[4:], b)
	binary.LittleEndian.PutUint32(dst[8:], cc)
	binary.LittleEndian.PutUint32(dst[12:], d)
}

func (c *rc6Cipher) expandKey(key []byte) {
	const (
		r   = 20
		p32 = uint32(0xB7E15163)
		q32 = uint32(0x9E3779B9)
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

	sLen := 2*r + 4
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
