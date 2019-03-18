package multi_hash

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

type Hash struct {
	Name        string
	PackageHash string
	Creator     func() hash.Hash
}

var Hashes = []Hash{
	Hash{
		Name:        "MD5Sum",
		PackageHash: "MD5sum",
		Creator: func() hash.Hash {
			return md5.New()
		},
	},
	Hash{
		Name:        "SHA1",
		PackageHash: "SHA1",
		Creator: func() hash.Hash {
			return sha1.New()
		},
	},
	Hash{
		Name:        "SHA256",
		PackageHash: "SHA256",
		Creator: func() hash.Hash {
			return sha256.New()
		},
	},
	Hash{
		Name:        "SHA512",
		PackageHash: "SHA512",
		Creator: func() hash.Hash {
			return sha512.New()
		},
	},
}

type MultiHash struct {
	io.Writer
	hashes map[string]hash.Hash
	buffer bytes.Buffer
}

func New() (m *MultiHash) {
	m = &MultiHash{}
	m.hashes = make(map[string]hash.Hash)

	hashes := []io.Writer{
		&m.buffer,
	}
	for _, hashOpt := range Hashes {
		hash := hashOpt.Creator()
		hashes = append(hashes, hash)
		m.hashes[hashOpt.Name] = hash
	}
	m.Writer = io.MultiWriter(hashes...)
	return m
}

func (m *MultiHash) hash(hashName string) []byte {
	hash := m.hashes[hashName]
	return hash.Sum(nil)
}

func (m *MultiHash) WriteReleaseHash(w io.Writer, hashName string, name string) {
	hash := m.hashes[hashName]
	checksum := hash.Sum(nil)
	fmt.Fprintln(w, "", hex.EncodeToString(checksum), m.buffer.Len(), name)
}

func (m *MultiHash) WritePackageHashes(w io.Writer) {
	for _, hashOpt := range Hashes {
		hash := m.hashes[hashOpt.Name]
		packageHashValue := hex.EncodeToString(hash.Sum(nil))
		fmt.Fprintln(w, hashOpt.PackageHash+":", packageHashValue)
	}
}

func HashMe(body func(w io.Writer) error) (*MultiHash, error) {
	hash := New()
	return hash, body(hash)
}
