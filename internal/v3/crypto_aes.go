package v3

import (
	"crypto/aes"
	"crypto/cipher"
)

func newAESCipher(key []byte) (cipher.Block, error) {
	return aes.NewCipher(key)
}
