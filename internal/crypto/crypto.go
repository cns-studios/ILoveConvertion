package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/hkdf"
)

const (
	chunkSize = 64 * 1024

	nonceSize = 12

	tagSize = 16

	hkdfInfo = "fileforge-file-encryption"
)


func DeriveKey(masterKey []byte, jobID string) ([]byte, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}

	r := hkdf.New(sha256.New, masterKey, []byte(jobID), []byte(hkdfInfo))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("HKDF key derivation failed: %w", err)
	}
	return key, nil
}

func EncryptStream(key []byte, src io.Reader, dst io.Writer) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("cipher.NewGCM: %w", err)
	}

	baseNonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(baseNonce); err != nil {
		return fmt.Errorf("nonce generation: %w", err)
	}

	if _, err := dst.Write(baseNonce); err != nil {
		return fmt.Errorf("write nonce: %w", err)
	}

	buf := make([]byte, chunkSize)
	var chunkIdx uint32

	for {
		n, readErr := io.ReadFull(src, buf)

		if n > 0 {
			nonce := chunkNonce(baseNonce, chunkIdx)
			encrypted := gcm.Seal(nil, nonce, buf[:n], nil)

			if _, err := dst.Write(encrypted); err != nil {
				return fmt.Errorf("write chunk %d: %w", chunkIdx, err)
			}
			chunkIdx++
		}

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read chunk %d: %w", chunkIdx, readErr)
		}
	}

	return nil
}

func DecryptStream(key []byte, src io.Reader, dst io.Writer) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("cipher.NewGCM: %w", err)
	}

	baseNonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(src, baseNonce); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("read nonce: %w", err)
	}

	encChunkSize := chunkSize + gcm.Overhead()
	buf := make([]byte, encChunkSize)
	var chunkIdx uint32

	for {
		n, readErr := io.ReadFull(src, buf)

		if n > 0 {
			nonce := chunkNonce(baseNonce, chunkIdx)
			plaintext, decErr := gcm.Open(nil, nonce, buf[:n], nil)
			if decErr != nil {
				return fmt.Errorf("decrypt chunk %d: %w", chunkIdx, decErr)
			}

			if _, err := dst.Write(plaintext); err != nil {
				return fmt.Errorf("write chunk %d: %w", chunkIdx, err)
			}
			chunkIdx++
		}

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read chunk %d: %w", chunkIdx, readErr)
		}
	}

	return nil
}

func EncryptFile(key []byte, srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source %s: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create dest %s: %w", dstPath, err)
	}
	defer func() {
		dst.Close()
		if err != nil {
			os.Remove(dstPath)
		}
	}()

	if err = EncryptStream(key, src, dst); err != nil {
		return err
	}
	return dst.Sync()
}

func DecryptFile(key []byte, srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source %s: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create dest %s: %w", dstPath, err)
	}
	defer func() {
		dst.Close()
		if err != nil {
			os.Remove(dstPath)
		}
	}()

	if err = DecryptStream(key, src, dst); err != nil {
		return err
	}
	return dst.Sync()
}

func chunkNonce(baseNonce []byte, idx uint32) []byte {
	nonce := make([]byte, len(baseNonce))
	copy(nonce, baseNonce)
	nonce[8] ^= byte(idx >> 24)
	nonce[9] ^= byte(idx >> 16)
	nonce[10] ^= byte(idx >> 8)
	nonce[11] ^= byte(idx)
	return nonce
}