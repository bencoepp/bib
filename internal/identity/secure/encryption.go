package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

type EncryptedBlob struct {
	Version int `json:"version"`
	KDF     struct {
		Name    string `json:"name"`
		Salt    string `json:"salt"`
		Time    uint32 `json:"time"`
		Memory  uint32 `json:"memory"`
		Threads uint8  `json:"threads"`
		KeyLen  uint32 `json:"keyLen"`
	} `json:"kdf"`
	AEAD struct {
		Cipher     string `json:"cipher"`
		Nonce      string `json:"nonce"`
		Ciphertext string `json:"ciphertext"`
	} `json:"aead"`
}

func deriveCombinedKey(passphrase, secondFactor, salt []byte, t, m uint32, p uint8, dkLen uint32) ([]byte, []byte) {
	kdfOut := argon2.IDKey(passphrase, salt, t, m, p, dkLen)
	h := hkdf.New(sha256.New, kdfOut, secondFactor, []byte("bibd identity encryption"))
	final := make([]byte, 32)
	_, err := io.ReadFull(h, final)
	if err != nil {
		return nil, nil
	}
	return kdfOut, final
}

func EncryptPrivateKey(passphrase, secondFactor, plaintext []byte) (*EncryptedBlob, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	t, m, p := uint32(3), uint32(64*1024), uint8(4)
	dkLen := uint32(32)
	_, key := deriveCombinedKey(passphrase, secondFactor, salt, t, m, p, dkLen)

	ahead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ct := ahead.Seal(nil, nonce, plaintext, nil)

	blob := &EncryptedBlob{Version: 1}
	blob.KDF.Name = "argon2id+hkdf"
	blob.KDF.Salt = base64.StdEncoding.EncodeToString(salt)
	blob.KDF.Time = t
	blob.KDF.Memory = m
	blob.KDF.Threads = p
	blob.KDF.KeyLen = dkLen
	blob.AEAD.Cipher = "xchacha20-poly1305"
	blob.AEAD.Nonce = base64.StdEncoding.EncodeToString(nonce)
	blob.AEAD.Ciphertext = base64.StdEncoding.EncodeToString(ct)
	return blob, nil
}

// DecryptPrivateKey recovers plaintext private key bytes.
func DecryptPrivateKey(passphrase, secondFactor []byte, blob *EncryptedBlob) ([]byte, error) {
	if blob.AEAD.Cipher != "xchacha20-poly1305" || blob.KDF.Name != "argon2id+hkdf" {
		return nil, errors.New("unsupported blob parameters")
	}
	salt, err := base64.StdEncoding.DecodeString(blob.KDF.Salt)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(blob.AEAD.Nonce)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(blob.AEAD.Ciphertext)
	if err != nil {
		return nil, err
	}

	_, key := deriveCombinedKey(passphrase, secondFactor, salt, blob.KDF.Time, blob.KDF.Memory, blob.KDF.Threads, blob.KDF.KeyLen)
	ahead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	pt, err := ahead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}

func MarshalBlob(b *EncryptedBlob) ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

func UnmarshalBlob(data []byte) (*EncryptedBlob, error) {
	var b EncryptedBlob
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
