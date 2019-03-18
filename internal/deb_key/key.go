package deb_key

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/clearsign"
)

type Key struct {
	signingKey *openpgp.Entity
}

func (k *Key) EncodeWithArmor(w io.Writer, body func(w io.Writer) error) error {
	pr, pw := io.Pipe()
	defer pr.Close()

	go func() {
		defer pw.Close()
		pw.CloseWithError(body(pw))
	}()

	return openpgp.ArmoredDetachSign(w, k.signingKey, pr, nil)
}

func (k *Key) Encode(w io.Writer, body func(w io.Writer) error) error {
	wd, err := clearsign.Encode(w, k.signingKey.PrivateKey, nil)
	if err != nil {
		return err
	}
	defer wd.Close()

	return body(wd)
}

func (k *Key) WriteKey(w io.Writer) error {
	wd, err := armor.Encode(w, openpgp.PublicKeyType, nil)
	if err != nil {
		return err
	}
	defer wd.Close()

	return k.signingKey.Serialize(wd)
}

func New(key string) (*Key, error) {
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(key))
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment GPG_KEY: %q", err)
	}
	if len(entityList) != 1 {
		return nil, fmt.Errorf("exactly one entity should be in GPG_KEY. was: %d", len(entityList))
	}

	return &Key{
		signingKey: entityList[0],
	}, nil
}
