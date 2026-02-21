package crypto

// blockCipher is the interface for ModulusCryptor block ciphers.
type blockCipher interface {
	BlockSize() int
	DecryptBlock(src, dst []byte)
}

// modulusKey1 is the hard-coded 32-byte key for stage 1 decryption.
var modulusKey1 = []byte("webzen#@!01webzen#@!01webzen#@!0")

// initCipher creates a block cipher by algorithm index (0-7).
func initCipher(algorithm int, key []byte) blockCipher {
	switch algorithm & 7 {
	case 0:
		return newTEA(key)
	case 1:
		return newThreeWay(key)
	case 2:
		return newCAST5(key)
	case 3:
		return newRC5(key)
	case 4:
		return newRC6(key)
	case 5:
		return newMARS(key)
	case 6:
		return newIDEA(key)
	case 7:
		return newGOST(key)
	default:
		return newTEA(key) // unreachable
	}
}

// decryptBlocks decrypts data in-place using the given cipher's block size.
func decryptBlocks(cipher blockCipher, data []byte, size int) {
	bs := cipher.BlockSize()
	tmp := make([]byte, bs)
	for i := 0; i+bs <= size; i += bs {
		cipher.DecryptBlock(data[i:i+bs], tmp)
		copy(data[i:i+bs], tmp)
	}
}

// DecryptModulus decrypts BMD v14 data using the ModulusCryptor algorithm.
// Input: raw encrypted data (after 8-byte BMD header: "BMD\x0E" + uint32 size).
// The first 2 bytes select cipher algorithms, bytes 2-33 contain the embedded key
// (recovered after stage 1 partial decryption), and bytes 34+ contain the payload.
func DecryptModulus(data []byte) []byte {
	if len(data) < 34 {
		return data
	}

	// Clone to avoid mutating input
	buf := make([]byte, len(data))
	copy(buf, data)

	algorithm1 := int(buf[1]) // stage 1 cipher selector
	algorithm2 := int(buf[0]) // stage 2 cipher selector
	size := len(buf)
	dataSize := size - 34

	// Stage 1: partially decrypt to recover key_2
	cipher1 := initCipher(algorithm1, modulusKey1)
	blockSize := 1024 - (1024 % cipher1.BlockSize())

	if dataSize > 4*blockSize {
		// Decrypt middle block
		index := 2 + (dataSize >> 1)
		decryptBlocks(cipher1, buf[index:index+blockSize], blockSize)
	}

	if dataSize > blockSize {
		// Decrypt end block
		index := size - blockSize
		decryptBlocks(cipher1, buf[index:index+blockSize], blockSize)

		// Decrypt start block
		index = 2
		decryptBlocks(cipher1, buf[index:index+blockSize], blockSize)
	}

	// Extract key_2 from bytes [2:34]
	key2 := make([]byte, 32)
	copy(key2, buf[2:34])

	// Stage 2: decrypt actual data using key_2
	cipher2 := initCipher(algorithm2, key2)
	decryptSize := dataSize - (dataSize % cipher2.BlockSize())

	if decryptSize > 0 {
		decryptBlocks(cipher2, buf[34:34+decryptSize], decryptSize)
	}

	// Return data without 34-byte crypto header
	return buf[34:]
}
