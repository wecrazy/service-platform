package fun

import (
	"crypto/cipher"
	"crypto/des"
	"fmt"
)

func GenerateUnlockSNCode(code string) string {
	// Get the current date in YYYY-MM-DD format
	if len(code) != 16 {
		return "FALSE INPUT"
	}
	sn := code[:8]
	version := code[8:12]
	random := code[12:]
	data1 := fmt.Sprintf("%s%s%s", version, sn, random)
	data2 := fmt.Sprintf("%s%s%s%s", "2222", version, random, "2222")
	finalData, err := xorStrings(data1, data2)
	fmt.Printf("%X\n", finalData)
	if err != nil {
		fmt.Println(err)
		return "FALSE INPUT"
	}
	key1 := fmt.Sprintf("%s%s%s", version, sn, "1111")
	key2 := fmt.Sprintf("%s%s%s", "1111", random, sn)
	finalKey, err := xorStrings(key1, key2)
	fmt.Printf("%X\n", finalKey)
	if err != nil {
		fmt.Println(err)
		return "FALSE INPUT"
	}
	iv := make([]byte, 8)
	ret, err := encrypt3DESCBCNoPadding([]byte(finalKey), []byte(finalData), iv)
	if err != nil {
		fmt.Println(err)
		return "FALSE INPUT"
	}
	fmt.Printf("%X\n", ret)
	// fmt.Printf("%X\n", encryptedSecondHalf)
	res := fmt.Sprintf("%X", ret[7:10])

	return res
}

func encrypt3DESCBCNoPadding(key, data, iv []byte) ([]byte, error) {
	// Ensure the key is 16 or 24 bytes for 3DES
	if len(key) != 16 && len(key) != 24 {
		return nil, fmt.Errorf("key length must be 16 or 24 bytes")
	}

	// 3DES requires a 24-byte key. If the key is 16 bytes, append first 8 bytes to make 24 bytes.
	if len(key) == 16 {
		key = append(key, key[:8]...)
	}

	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}

	// Ensure the IV length is correct (8 bytes for 3DES)
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("IV length must be %d bytes", block.BlockSize())
	}

	// Ensure the data length is a multiple of the block size (8 bytes for 3DES)
	if len(data)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("data is not a multiple of the block size (8 bytes)")
	}

	// Create the CBC mode encrypter
	mode := cipher.NewCBCEncrypter(block, iv)

	// Create the buffer to hold the encrypted data
	encrypted := make([]byte, len(data))

	// Encrypt the data (no padding applied, data must be block size aligned)
	mode.CryptBlocks(encrypted, data)

	return encrypted, nil
}

func xorStrings(str1, str2 string) (string, error) {
	// Convert the strings to byte slices
	bytes1 := []byte(str1)
	bytes2 := []byte(str2)

	// Ensure both strings are of the same length
	if len(bytes1) != len(bytes2) {
		return "", fmt.Errorf("strings must be of the same length to XOR")
	}

	// XOR the bytes
	xorResult := make([]byte, len(bytes1))
	for i := 0; i < len(bytes1); i++ {
		xorResult[i] = bytes1[i] ^ bytes2[i]
	}

	// Convert the XOR result back to a string
	return string(xorResult), nil
}
