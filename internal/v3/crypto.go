package v3

import (
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
)

func HMACDigest(proto AuthProtocol, key, data []byte) ([]byte, error) {
	hf, err := hashFunc(proto)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(hf, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil), nil
}

func VerifyHMAC(proto AuthProtocol, key, data, digest []byte) (bool, error) {
	computed, err := HMACDigest(proto, key, data)
	if err != nil {
		return false, err
	}
	return hmac.Equal(computed, digest), nil
}

func LocalizeKey(proto AuthProtocol, passphrase, engineID []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("empty passphrase")
	}
	hf, err := hashFunc(proto)
	if err != nil {
		return nil, err
	}

	extended := make([]byte, 0, 1048576)
	for len(extended) < 1048576 {
		remaining := 1048576 - len(extended)
		if remaining >= len(passphrase) {
			extended = append(extended, passphrase...)
		} else {
			extended = append(extended, passphrase[:remaining]...)
		}
	}

	h := hf()
	_, _ = h.Write(extended)
	ku := h.Sum(nil)

	h2 := hf()
	_, _ = h2.Write(ku)
	_, _ = h2.Write(engineID)
	_, _ = h2.Write(ku)
	return h2.Sum(nil), nil
}

func EncryptCFB(proto PrivProtocol, key, iv, plaintext []byte) ([]byte, error) {
	block, normalizedKey, err := blockForPriv(proto, key)
	if err != nil {
		return nil, err
	}
	if len(iv) < block.BlockSize() {
		return nil, fmt.Errorf("iv too short: need at least %d bytes", block.BlockSize())
	}
	ciphertext := make([]byte, len(plaintext))
	stream := cipher.NewCFBEncrypter(block, iv[:block.BlockSize()])
	stream.XORKeyStream(ciphertext, plaintext)
	_ = normalizedKey
	return ciphertext, nil
}

func DecryptCFB(proto PrivProtocol, key, iv, ciphertext []byte) ([]byte, error) {
	block, _, err := blockForPriv(proto, key)
	if err != nil {
		return nil, err
	}
	if len(iv) < block.BlockSize() {
		return nil, fmt.Errorf("iv too short: need at least %d bytes", block.BlockSize())
	}
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCFBDecrypter(block, iv[:block.BlockSize()])
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

func hashFunc(proto AuthProtocol) (func() hash.Hash, error) {
	switch proto {
	case AuthMD5:
		return md5.New, nil
	case AuthSHA1:
		return sha1.New, nil
	case AuthSHA224:
		return sha256.New224, nil
	case AuthSHA256:
		return sha256.New, nil
	case AuthSHA384:
		return sha512.New384, nil
	case AuthSHA512:
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("unsupported auth protocol: %s", proto)
	}
}

func blockForPriv(proto PrivProtocol, key []byte) (cipher.Block, []byte, error) {
	switch proto {
	case PrivDES:
		k := normalizeKey(key, 8)
		block, err := des.NewCipher(k)
		return block, k, err
	case Priv3DES:
		k := normalizeKey(key, 24)
		block, err := des.NewTripleDESCipher(k)
		return block, k, err
	case PrivAES128:
		k := normalizeKey(key, 16)
		block, err := aesNewCipher(k)
		return block, k, err
	case PrivAES192:
		k := normalizeKey(key, 24)
		block, err := aesNewCipher(k)
		return block, k, err
	case PrivAES256:
		k := normalizeKey(key, 32)
		block, err := aesNewCipher(k)
		return block, k, err
	default:
		return nil, nil, fmt.Errorf("unsupported priv protocol: %s", proto)
	}
}

func normalizeKey(key []byte, length int) []byte {
	out := make([]byte, length)
	if len(key) == 0 {
		return out
	}
	for i := 0; i < length; i++ {
		out[i] = key[i%len(key)]
	}
	return out
}

func aesNewCipher(key []byte) (cipher.Block, error) {
	// Isolated helper so tests can focus on this package only.
	return newAESCipher(key)
}
