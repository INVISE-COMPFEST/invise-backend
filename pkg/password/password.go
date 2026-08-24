package password

import "golang.org/x/crypto/bcrypt"

type BcryptI interface {
	Hash(password string) (string, error)
	Compare(hashed, password string) error
}

type bcryptPassword struct{}

func New() BcryptI {
	return &bcryptPassword{}
}

func (b *bcryptPassword) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (b *bcryptPassword) Compare(hashed, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
}
