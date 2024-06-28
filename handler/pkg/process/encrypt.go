package process

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"

	"go.uber.org/zap"
)

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func EncryptData(reader io.Reader, key string, l *zap.SugaredLogger) (io.Reader, error) {
	block, err := aes.NewCipher([]byte(createHash(key)))
	if err != nil {
		l.Error("aes failed")
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		l.Error("gcm failed")
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadAtLeast(rand.Reader, nonce, len(nonce)); err != nil {
		l.Error("nonce failed")
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(nonce)

	data, err := io.ReadAll(reader)
	if err != nil {
		l.Error("reader failed")
		return nil, err
	}

	encryptedData := gcm.Seal(nil, nonce, data, nil)
	buf.Write(encryptedData)

	return &buf, nil
}
