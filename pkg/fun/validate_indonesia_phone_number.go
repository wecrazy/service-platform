package fun

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// validPhonePre is a map of valid Indonesian phone number prefixes for both mobile and landline numbers. It includes prefixes for major mobile operators (Telkomsel, Indosat IM3, Tri, XL Axiata, AXIS, Smartfren) as well as landline area codes for various cities across Indonesia. This map is used to validate the prefix of a phone number to ensure it conforms to known Indonesian phone number formats.
var validPhonePre = map[string]bool{
	// Telkomsel
	"080": true, "0810": true, "0811": true, "0812": true, "0813": true, "082": true, "0821": true, "0822": true, "0823": true, "0850": true, "0851": true, "0852": true, "0853": true,
	// Indosat IM3
	"0814": true, "0815": true, "0816": true, "0854": true, "0855": true, "0856": true, "0857": true, "0858": true,
	// Tri
	"089": true, "0895": true, "0896": true, "0897": true, "0898": true, "0899": true,
	// XL Axiata XL
	"0817": true, "0818": true, "0819": true, "0859": true, "087": true, "0877": true, "0878": true, "0879": true,
	// XL Axiata AXIS
	"083": true, "0831": true, "0832": true, "0833": true, "0838": true,
	// Smartfren
	"0881": true, "0882": true, "0883": true, "084": true, "0884": true, "0885": true, "0886": true, "0887": true, "0888": true, "0889": true,
	// Unknown
	"086": true,
	// Landline
	"021":  true, // Jakarta
	"022":  true, // Bandung
	"024":  true, // Semarang
	"0251": true, // Bogor
	"0252": true, // Sukabumi
	"0254": true, // Serang
	"0261": true, // Cirebon
	"0271": true, // Solo / Surakarta
	"0274": true, // Yogyakarta
	"0281": true, // Purwokerto
	"031":  true, // Surabaya
	"0341": true, // Malang
	"0351": true, // Madiun
	"0361": true, // Denpasar / Bali
	"0380": true, // Kupang
	"0411": true, // Makassar
	"0431": true, // Manado
	"061":  true, // Medan
	"0621": true, // Pematangsiantar
	"0631": true, // Kisaran
	"0641": true, // Rantauprapat
	"0711": true, // Palembang
	"0721": true, // Bandar Lampung
	"0751": true, // Padang
	"0761": true, // Pekanbaru
	"0778": true, // Batam
	"0911": true, // Balikpapan
	"0541": true, // Samarinda
	"0521": true, // Banjarmasin
	"0951": true, // Sorong
	"0967": true, // Manokwari

}

// SanitizeIndonesiaPhoneNumber sanitizes the phone number and checks its validity.
// If multiple phone numbers are provided (separated by '/', ',', ';', or '|'),
// it will return the first valid one (starting with 8, e.g. 8517320xxxx).
// so after get the result you can add "62" in front of it to make it a complete Indonesian phone number.
func SanitizeIndonesiaPhoneNumber(phone string) (string, error) {
	// Split the input by common delimiters to handle multiple phone numbers
	delimiters := []string{"/", ",", ";", "|", " / ", " , ", " ; ", " | ", "atau", " atau ", "or", " or "}
	phoneNumbers := []string{phone} // Start with the original string

	// Split by each delimiter
	for _, delimiter := range delimiters {
		var newPhoneNumbers []string
		for _, num := range phoneNumbers {
			parts := strings.Split(num, delimiter)
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					newPhoneNumbers = append(newPhoneNumbers, trimmed)
				}
			}
		}
		phoneNumbers = newPhoneNumbers
	}

	// Try to sanitize each phone number and return the first valid one
	for _, phoneNum := range phoneNumbers {
		sanitized, err := sanitizeSingleIndonesiaPhoneNumber(phoneNum)
		if err == nil {
			return sanitized, nil
		}
	}

	// If no valid phone number found, return error with all attempted numbers
	return "", fmt.Errorf("no valid phone number found in: %v", phone)
}

// sanitizeSingleIndonesiaPhoneNumber handles a single phone number sanitization
func sanitizeSingleIndonesiaPhoneNumber(phone string) (string, error) {
	// Remove all unwanted characters and keep only digits & '+' signs
	phone = strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || r == '+' {
			return r // Keep only digits and '+'
		}
		return -1 // Remove everything else
	}, phone)

	// Skip empty strings
	if phone == "" {
		return "", errors.New("empty phone number")
	}

	// Handle the case where phone starts with '62' (should be '0')
	if strings.HasPrefix(phone, "62") {
		phone = "0" + phone[2:] // Replace '62' with '0'
	}

	// If the phone starts with '+62', replace it with '0'
	if strings.HasPrefix(phone, "+62") {
		phone = "0" + phone[3:] // Replace '+62' with '0'
	}

	// If the phone starts with "8" (e.g., "85173207755"), prepend "0"
	if strings.HasPrefix(phone, "8") {
		phone = "0" + phone
	}

	// Ensure the phone number starts with '0'
	if !strings.HasPrefix(phone, "0") {
		return "", fmt.Errorf("invalid phone number: %v must start with '0' or '+62'", phone)
	}

	// Validate the phone number prefix
	if !IsValidIndonesiaPhoneNumber(phone) {
		return "", fmt.Errorf("invalid phone number: %v prefix is not valid try using indonesian number that start with '0' or '+62'", phone)
	}

	// Return the sanitized phone number without the leading '0'
	return phone[1:], nil
}

// IsValidIndonesiaPhoneNumber checks if the phone number has a valid prefix
func IsValidIndonesiaPhoneNumber(phone string) bool {
	// Check if the phone number is long enough and if the prefix is valid
	if len(phone) < 4 {
		return false
	}

	// Check the first 4 digits
	prefix := phone[:4]
	if validPhonePre[prefix] {
		return true
	}

	// Check the first 3 digits
	prefix = phone[:3]
	return validPhonePre[prefix]
}

// SanitizeAllIndonesiaPhoneNumbers returns all valid phone numbers from a string containing multiple numbers
// This is useful if you want to get all valid numbers instead of just the first one
func SanitizeAllIndonesiaPhoneNumbers(phone string) ([]string, error) {
	// Split the input by common delimiters to handle multiple phone numbers
	delimiters := []string{"/", ",", ";", "|", " / ", " , ", " ; ", " | ", "atau", " atau ", "or", " or "}
	phoneNumbers := []string{phone} // Start with the original string

	// Split by each delimiter
	for _, delimiter := range delimiters {
		var newPhoneNumbers []string
		for _, num := range phoneNumbers {
			parts := strings.Split(num, delimiter)
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					newPhoneNumbers = append(newPhoneNumbers, trimmed)
				}
			}
		}
		phoneNumbers = newPhoneNumbers
	}

	// Collect all valid phone numbers
	var validNumbers []string
	for _, phoneNum := range phoneNumbers {
		sanitized, err := sanitizeSingleIndonesiaPhoneNumber(phoneNum)
		if err == nil {
			validNumbers = append(validNumbers, sanitized)
		}
	}

	if len(validNumbers) == 0 {
		return nil, fmt.Errorf("no valid phone number found in: %v", phone)
	}

	return validNumbers, nil
}
