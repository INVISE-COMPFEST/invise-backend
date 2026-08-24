package ulid

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

type GeneratorI interface {
	Generate() (string, error)
}

type ulidGen struct{}

func New() GeneratorI {
	return &ulidGen{}
}

func (u *ulidGen) Generate() (string, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
