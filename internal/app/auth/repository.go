package auth

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Email        string    `gorm:"column:email;uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash"`
	Verified     bool      `gorm:"column:verified;default:false"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }

type UserRepositoryI interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	UpdateVerified(ctx context.Context, id string, verified bool) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepositoryI {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) UpdateVerified(ctx context.Context, id string, verified bool) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("verified", verified).Error
}
