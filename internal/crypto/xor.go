package crypto

// DecryptXOR decrypts BMD v12 data using chained XOR with the 16-byte key.
// Initial chain value is 0x5E. For each byte:
//
//	out[i] = ((data[i] ^ XORKey[i&15]) - chainKey) & 0xFF
//	chainKey = (data[i] + 0x3D) & 0xFF
func DecryptXOR(data []byte) []byte {
	out := make([]byte, len(data))
	chainKey := byte(0x5E)

	for i, b := range data {
		out[i] = (b ^ XORKey[i&15]) - chainKey
		chainKey = b + 0x3D
	}
	return out
}

// DecryptTRS decrypts ItemTRSData.bmd using simple 3-byte repeating XOR.
//
//	out[i] = data[i] ^ TRSXORKey[i%3]
func DecryptTRS(data []byte) []byte {
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ TRSXORKey[i%3]
	}
	return out
}
