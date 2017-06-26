package main

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

var supportedHashes = []string{"MD5Sum", "SHA1", "SHA256", "SHA512"}

type multiHash struct {
	io.Writer
	hashes map[string]hash.Hash
	buffer bytes.Buffer
}

func newHash(hashName string) hash.Hash {
	switch hashName {
	case "MD5Sum":
		return md5.New()
	case "SHA1":
		return sha1.New()
	case "SHA256":
		return sha256.New()
	case "SHA512":
		return sha512.New()
	default:
		return nil
	}
}

func newMultiHash() (m *multiHash) {
	m = &multiHash{}
	m.hashes = make(map[string]hash.Hash)

	hashes := []io.Writer{
		&m.buffer,
	}
	for _, hashName := range supportedHashes {
		hash := newHash(hashName)
		hashes = append(hashes, hash)
		m.hashes[hashName] = hash
	}
	m.Writer = io.MultiWriter(hashes...)
	return m
}

func (m *multiHash) hash(hashName string) []byte {
	hash := m.hashes[hashName]
	return hash.Sum(nil)
}

func (m *multiHash) releaseHash(w io.Writer, hashName string, name string) {
	hash := m.hashes[hashName]
	checksum := hash.Sum(nil)
	fmt.Fprintln(w, "", hex.EncodeToString(checksum), m.buffer.Len(), name)
}

func (m *multiHash) packagesHashes() map[string]string {
	hashes := make(map[string]string)
	for _, hashName := range supportedHashes {
		hash := m.hashes[hashName]
		if hashName == "MD5Sum" {
			hashName = "MD5sum"
		}
		hashes[hashName] = hex.EncodeToString(hash.Sum(nil))
	}
	return hashes
}
