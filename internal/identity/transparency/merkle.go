package transparency

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

type MerkleTree struct {
	Leaves [][]byte
}

func NewMerkleTree() *MerkleTree {
	return &MerkleTree{Leaves: [][]byte{}}
}

func (t *MerkleTree) AddLeaf(data []byte) (uint64, []byte) {
	h := sha256.Sum256(data)
	t.Leaves = append(t.Leaves, h[:])
	return uint64(len(t.Leaves) - 1), h[:]
}

func (t *MerkleTree) Root() []byte {
	n := len(t.Leaves)
	if n == 0 {
		empty := sha256.Sum256([]byte{})
		return empty[:]
	}
	level := t.Leaves
	for len(level) > 1 {
		var next [][]byte
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				paired := append(level[i], level[i]...)
				h := sha256.Sum256(paired)
				next = append(next, h[:])
			} else {
				paired := append(level[i], level[i+1]...)
				h := sha256.Sum256(paired)
				next = append(next, h[:])
			}
		}
		level = next
	}
	return level[0]
}

func (t *MerkleTree) InclusionProof(index uint64) ([][]byte, error) {
	if index >= uint64(len(t.Leaves)) {
		return nil, errors.New("index out of range")
	}
	var proof [][]byte
	level := t.Leaves
	pos := int(index)
	for len(level) > 1 {
		var next [][]byte
		for i := 0; i < len(level); i += 2 {
			j := i + 1
			left := level[i]
			var right []byte
			if j < len(level) {
				right = level[j]
			} else {
				right = left
			}
			if pos == i && j < len(level) {
				proof = append(proof, right)
			} else if pos == j {
				proof = append(proof, left)
			} else if pos == i && j >= len(level) {
				proof = append(proof, right)
			}
			paired := append(left, right...)
			h := sha256.Sum256(paired)
			next = append(next, h[:])
		}
		pos /= 2
		level = next
	}
	return proof, nil
}

func VerifyInclusion(leafHash []byte, index uint64, proof [][]byte) []byte {
	h := leafHash
	pos := index
	for _, sibling := range proof {
		var combined []byte
		if pos%2 == 0 {
			combined = append(h, sibling...)
		} else {
			combined = append(sibling, h...)
		}
		x := sha256.Sum256(combined)
		h = x[:]
		pos /= 2
	}
	return h
}

func SignRoot(tree *MerkleTree, logPrivy ed25519.PrivateKey, logPub ed25519.PublicKey) (*SignedLogRoot, error) {
	root := tree.Root()

	// Compute key ID: assign to a variable first, then slice.
	keyHash := sha256.Sum256(logPub) // [32]byte
	keyID := hex.EncodeToString(keyHash[:])

	sl := &SignedLogRoot{
		TreeSize:  uint64(len(tree.Leaves)),
		RootHash:  hex.EncodeToString(root),
		Timestamp: time.Now().UTC(),
		LogKeyID:  keyID,
	}

	// Message layout (stable):
	msg := sl.RootHash + "|" +
		sl.Timestamp.Format(time.RFC3339Nano) + "|" +
		sl.LogKeyID + "|" +
		strconv.FormatUint(sl.TreeSize, 10)

	d := sha256.Sum256([]byte(msg))
	sig := ed25519.Sign(logPrivy, d[:])
	sl.Signature = base64.StdEncoding.EncodeToString(sig)
	return sl, nil
}

func VerifySignedRoot(root *SignedLogRoot, logPub ed25519.PublicKey) error {
	msg := root.RootHash + "|" + root.Timestamp.Format(time.RFC3339Nano) + "|" + root.LogKeyID + "|" + strconv.FormatUint(root.TreeSize, 10)
	d := sha256.Sum256([]byte(msg))
	sigBytes, err := base64.StdEncoding.DecodeString(root.Signature)
	if err != nil {
		return err
	}
	if !ed25519.Verify(logPub, d[:], sigBytes) {
		return errors.New("invalid root signature")
	}
	return nil
}
