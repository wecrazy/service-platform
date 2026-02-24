package fun

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// Salt struct to hold the salt string and its position for insertion
type Salt struct {
	Salt     string
	Position int
}

// InsertStringAtPositions inserts the specified salt strings at their respective positions in the original string and returns the modified string.
func InsertStringAtPositions(original string, salts ...Salt) string {
	// Sort positions in ascending order
	// sort.Ints(positions[int])
	sort.Slice(salts, func(i, j int) bool {
		return salts[i].Position < salts[j].Position
	})

	for i, salt := range salts {
		salt.Position = salt.Position + len(salt.Salt)*i

		original = original[:salt.Position] + salt.Salt + original[salt.Position:]
	}

	return original
}

// InsertRandomStringAtPositions inserts random hexadecimal strings of the specified length at the given positions in the original string and returns the modified string.
func InsertRandomStringAtPositions(original string, randomStringLength int, positions ...int) string {
	// Sort positions in ascending order
	sort.Ints(positions)

	for i, position := range positions {
		position = position + randomStringLength*i
		randomString := GenerateRandomHexaString(randomStringLength)

		original = original[:position] + randomString + original[position:]
	}

	return original
}

// RemoveSubstringAtPositions removes substrings of the specified length at the given positions from the original string and returns the modified string.
func RemoveSubstringAtPositions(original string, length int, positions ...int) string {
	// Sort positions in descending order
	sort.Ints(positions)

	// Adjust positions to account for previously inserted strings
	for _, position := range positions {
		original = original[:position] + original[position+length:]
	}
	return original
}

// GenerateSaltedPassword takes a plain-text password, generates random salts, inserts them at specific positions, hashes the result with SHA-256, and returns a combined string of salts and the salted hash.
func GenerateSaltedPassword(password string) string {
	if len(password) == 0 {
		return ""
	}

	saltA := GenerateRandomHexaString(4)
	saltB := GenerateRandomHexaString(4)
	saltC := GenerateRandomHexaString(4)
	saltD := GenerateRandomHexaString(4)

	saltedPassword := InsertStringAtPositions(password,
		Salt{Salt: saltA, Position: 2},
		Salt{Salt: saltB, Position: 5},
		Salt{Salt: saltC, Position: 7},
		Salt{Salt: saltD, Position: 8},
	)

	// Create a new SHA-256 hash
	hash := sha256.New()

	// Write the input data to the hash
	hash.Write([]byte(saltedPassword))

	// Get the finalized hash result as a byte slice
	hashBytes := hash.Sum(nil)

	// Convert the byte slice to a hexadecimal string
	hashedPassword := hex.EncodeToString(hashBytes)

	saltedHashedPassword := InsertRandomStringAtPositions(hashedPassword, 2, 5, 8, 10, 18)

	saltWithSaltedHash := saltA + saltB + saltC + saltD + saltedHashedPassword

	return saltWithSaltedHash

}

// IsPasswordMatchedMd5 checks if the provided plain-text password matches the given MD5 hashed password by hashing the input and comparing it to the stored hash.
func IsPasswordMatchedMd5(password, md5Password string) bool {
	// Compute the MD5 hash of the plain-text password
	hasher := md5.New()
	hasher.Write([]byte(password))
	md5HashedPassword := hex.EncodeToString(hasher.Sum(nil))

	// Compare the computed hash with the stored MD5 password
	return md5HashedPassword == md5Password
}

// IsPasswordMatched checks if the provided plain-text password matches the given salted and hashed password by extracting the salts, reconstructing the salted password, hashing it, and comparing it to the stored hash.
func IsPasswordMatched(password, saltWithSaltedHash string) bool {
	validSaltedHashedPassword := saltWithSaltedHash[16:]
	validHashedPassword := RemoveSubstringAtPositions(validSaltedHashedPassword, 2, 5, 8, 10, 18)

	// how to parse saltWithSaltedHash get the 16 first string and broke them into every 4 char
	saltA := saltWithSaltedHash[:4]
	saltB := saltWithSaltedHash[4:8]
	saltC := saltWithSaltedHash[8:12]
	saltD := saltWithSaltedHash[12:16]

	saltedCheckPassword := InsertStringAtPositions(password,
		Salt{Salt: saltA, Position: 2},
		Salt{Salt: saltB, Position: 5},
		Salt{Salt: saltC, Position: 7},
		Salt{Salt: saltD, Position: 8},
	)

	// Create a new SHA-256 hash
	hash := sha256.New()

	// Write the input data to the hash
	hash.Write([]byte(saltedCheckPassword))

	// Get the finalized hash result as a byte slice
	hashBytes := hash.Sum(nil)

	// Convert the byte slice to a hexadecimal string
	hashedCheckPassword := hex.EncodeToString(hashBytes)

	return hashedCheckPassword == validHashedPassword

}
