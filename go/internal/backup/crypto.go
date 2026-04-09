// Package backup implements partition backup, restore, and verification for
// Nothing Phone devices, plus AES-256-GCM encryption for backup images.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
	"golang.org/x/crypto/scrypt"
)

// scrypt parameters — byte-compatible with the Python backup.py implementation.
const (
	scryptN    = 1 << 14 // 16384
	scryptR    = 8
	scryptP    = 1
	keyLen     = 32
	nonceLen   = 12
	saltLen    = 32
	saltSuffix = ".salt"
)

// deriveKey derives a 32-byte AES key from password and salt using scrypt with
// the parameters above.
func deriveKey(password string, salt []byte) ([]byte, error) {
	key, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, fmt.Errorf("scrypt key derivation failed: %w", err)
	}
	return key, nil
}

// sealFile is the shared encryption core: generates a fresh nonce, derives a
// key from password+salt, encrypts inputPath with AES-256-GCM, and writes
// nonce || ciphertext to outputPath.
func sealFile(inputPath, outputPath, password string, salt []byte) error {
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nterrors.AdbError("generating nonce: " + err.Error())
	}
	key, err := deriveKey(password, salt)
	if err != nil {
		return nterrors.AdbError(err.Error())
	}
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return nterrors.AdbError("reading input file: " + err.Error())
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nterrors.AdbError("creating AES cipher: " + err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nterrors.AdbError("creating GCM: " + err.Error())
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return os.WriteFile(outputPath, append(nonce, ciphertext...), 0o600)
}

// EncryptFile encrypts inputPath with AES-256-GCM and writes the result to
// outputPath. Key derivation uses scrypt with a fresh 32-byte random salt that
// is saved to <outputPath>.salt.
//
// Output format: 12-byte random nonce || ciphertext+tag (16-byte GCM tag appended
// by AESGCM.Seal).
//
// This is byte-compatible with the Python _encrypt_backup() implementation.
func EncryptFile(inputPath, outputPath, password string) error {
	// Generate random salt.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nterrors.AdbError("generating salt: " + err.Error())
	}

	if err := sealFile(inputPath, outputPath, password, salt); err != nil {
		return err
	}

	// Write salt file.
	saltPath := outputPath + saltSuffix
	if err := os.WriteFile(saltPath, salt, 0o600); err != nil {
		return nterrors.AdbError("writing salt file: " + err.Error())
	}

	return nil
}

// encryptFileWithSalt is the low-level helper used by encryptBackup. It
// encrypts inputPath → outputPath using the provided salt (rather than
// generating a fresh one). Output format: 12-byte nonce || ciphertext.
func encryptFileWithSalt(inputPath, outputPath, password string, salt []byte) error {
	return sealFile(inputPath, outputPath, password, salt)
}

// decryptFileWithSalt is the low-level helper used by decryptBackup. It
// decrypts inputPath → outputPath using the provided salt bytes.
func decryptFileWithSalt(inputPath, outputPath, password string, salt []byte) error {
	key, err := deriveKey(password, salt)
	if err != nil {
		return nterrors.AdbError(err.Error())
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return nterrors.AdbError("reading encrypted file: " + err.Error())
	}
	if len(raw) < nonceLen {
		return nterrors.AdbError(
			fmt.Sprintf("encrypted file too short to contain nonce: %s", inputPath),
		)
	}
	nonce := raw[:nonceLen]
	ciphertext := raw[nonceLen:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nterrors.AdbError("creating AES cipher: " + err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nterrors.AdbError("creating GCM: " + err.Error())
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nterrors.AdbError(
			"decryption failed for " + inputPath + ".\n" +
				"Wrong password, or the file is corrupt.",
		)
	}
	if err := os.WriteFile(outputPath, plaintext, 0o600); err != nil {
		return nterrors.AdbError("writing decrypted file: " + err.Error())
	}
	return nil
}

// DecryptFile decrypts a file produced by EncryptFile. The salt is read from
// <inputPath>.salt; the nonce is the first 12 bytes of the encrypted file.
func DecryptFile(inputPath, outputPath, password string) error {
	// Read salt.
	saltPath := inputPath + saltSuffix
	salt, err := os.ReadFile(saltPath)
	if err != nil {
		return nterrors.AdbError(
			"reading salt file " + saltPath + ": " + err.Error() + "\n" +
				"Cannot decrypt backup without it.",
		)
	}

	// Derive key.
	key, err := deriveKey(password, salt)
	if err != nil {
		return nterrors.AdbError(err.Error())
	}

	// Read encrypted file.
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return nterrors.AdbError("reading encrypted file: " + err.Error())
	}
	if len(raw) < nonceLen {
		return nterrors.AdbError(
			fmt.Sprintf("encrypted file too short to contain nonce: %s", inputPath),
		)
	}

	nonce := raw[:nonceLen]
	ciphertext := raw[nonceLen:]

	// Decrypt with AES-256-GCM.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nterrors.AdbError("creating AES cipher: " + err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nterrors.AdbError("creating GCM: " + err.Error())
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nterrors.AdbError(
			"decryption failed for " + inputPath + ".\n" +
				"Wrong password, or the file is corrupt.",
		)
	}

	if err := os.WriteFile(outputPath, plaintext, 0o600); err != nil {
		return nterrors.AdbError("writing decrypted file: " + err.Error())
	}
	return nil
}
