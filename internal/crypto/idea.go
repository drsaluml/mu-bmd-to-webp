package crypto

import "encoding/binary"

// ideaCipher implements IDEA (International Data Encryption Algorithm) decryption.
// 8-byte block, 16-byte key, 8 rounds.
type ideaCipher struct {
	decryptKeys [52]uint16
}

func newIDEA(key []byte) blockCipher {
	c := &ideaCipher{}
	encKeys := ideaExpandKey(key[:16])
	c.decryptKeys = ideaInvertKeys(encKeys)
	return c
}

func (c *ideaCipher) BlockSize() int { return 8 }

func (c *ideaCipher) DecryptBlock(src, dst []byte) {
	K := c.decryptKeys

	x0 := binary.BigEndian.Uint16(src[0:])
	x1 := binary.BigEndian.Uint16(src[2:])
	x2 := binary.BigEndian.Uint16(src[4:])
	x3 := binary.BigEndian.Uint16(src[6:])

	p := 0
	for round := 0; round < 8; round++ {
		x0 = ideaMulMod(x0, K[p]); p++
		x1 = ideaAddMod(x1, K[p]); p++
		x2 = ideaAddMod(x2, K[p]); p++
		x3 = ideaMulMod(x3, K[p]); p++

		t0 := x1
		t1 := x2

		x2 ^= x0
		x1 ^= x3

		x2 = ideaMulMod(x2, K[p]); p++
		x1 = ideaAddMod(x1, x2)
		x1 = ideaMulMod(x1, K[p]); p++
		x2 = ideaAddMod(x2, x1)

		x0 ^= x1
		x3 ^= x2
		x1 ^= t1
		x2 ^= t0
	}

	// Output transform
	o0 := ideaMulMod(x0, K[p]); p++
	o1 := ideaAddMod(x2, K[p]); p++
	o2 := ideaAddMod(x1, K[p]); p++
	o3 := ideaMulMod(x3, K[p])

	binary.BigEndian.PutUint16(dst[0:], o0)
	binary.BigEndian.PutUint16(dst[2:], o1)
	binary.BigEndian.PutUint16(dst[4:], o2)
	binary.BigEndian.PutUint16(dst[6:], o3)
}

func ideaExpandKey(key []byte) [52]uint16 {
	var Z [52]uint16
	for i := 0; i < 8; i++ {
		Z[i] = binary.BigEndian.Uint16(key[i*2:])
	}
	for i := 8; i < 52; i++ {
		if i&7 == 6 {
			Z[i] = (Z[i-7]<<9 | Z[i-14]>>7) & 0xFFFF
		} else if i&7 == 7 {
			Z[i] = (Z[i-15]<<9 | Z[i-14]>>7) & 0xFFFF
		} else {
			Z[i] = (Z[i-7]<<9 | Z[i-6]>>7) & 0xFFFF
		}
	}
	return Z
}

func ideaInvertKeys(enc [52]uint16) [52]uint16 {
	var dec [52]uint16
	p := 0
	q := 52

	t1 := ideaMulInverse(enc[p]); p++
	t2 := ideaAddInverse(enc[p]); p++
	t3 := ideaAddInverse(enc[p]); p++
	t4 := ideaMulInverse(enc[p]); p++
	q--; dec[q] = t4
	q--; dec[q] = t3
	q--; dec[q] = t2
	q--; dec[q] = t1

	for r := 1; r < 8; r++ {
		s1 := enc[p]; p++
		s2 := enc[p]; p++
		q--; dec[q] = s2
		q--; dec[q] = s1

		t1 = ideaMulInverse(enc[p]); p++
		t2 = ideaAddInverse(enc[p]); p++
		t3 = ideaAddInverse(enc[p]); p++
		t4 = ideaMulInverse(enc[p]); p++
		q--; dec[q] = t4
		q--; dec[q] = t2 // swapped
		q--; dec[q] = t3 // swapped
		q--; dec[q] = t1
	}

	s1 := enc[p]; p++
	s2 := enc[p]; p++
	q--; dec[q] = s2
	q--; dec[q] = s1

	t1 = ideaMulInverse(enc[p]); p++
	t2 = ideaAddInverse(enc[p]); p++
	t3 = ideaAddInverse(enc[p]); p++
	t4 = ideaMulInverse(enc[p])
	q--; dec[q] = t4
	q--; dec[q] = t3
	q--; dec[q] = t2
	q--; dec[q] = t1

	return dec
}

// ideaMulMod multiplies modulo 65537 (2^16 + 1). In IDEA, 0 represents 2^16.
func ideaMulMod(a, b uint16) uint16 {
	ai, bi := uint32(a), uint32(b)
	if ai == 0 {
		ai = 0x10000
	}
	if bi == 0 {
		bi = 0x10000
	}
	r := (ai * bi) % 0x10001
	if r == 0x10000 {
		return 0
	}
	return uint16(r)
}

func ideaAddMod(a, b uint16) uint16 {
	return a + b // uint16 wraps at 65536 naturally
}

// ideaMulInverse computes multiplicative inverse mod 65537 using extended Euclidean algorithm.
func ideaMulInverse(x uint16) uint16 {
	if x <= 1 {
		return x
	}

	t1 := uint32(0x10001 / uint32(x))
	y := uint32(0x10001 % uint32(x))

	if y == 1 {
		return uint16(0x10001 - t1)
	}

	t0 := uint32(1)
	xv := uint32(x)
	for y != 1 {
		q := xv / y
		xv = xv % y
		t0 = (t0 + q*t1) % 0x10001
		if xv == 1 {
			return uint16(t0)
		}
		q2 := y / xv
		y = y % xv
		t1 = (t1 + q2*t0) % 0x10001
	}

	return uint16(0x10001 - t1)
}

func ideaAddInverse(x uint16) uint16 {
	return uint16(0x10000-uint32(x)) & 0xFFFF
}
