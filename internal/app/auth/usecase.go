package auth

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"

	"invise-backend/internal/bootstrap/config"
	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/jwt"
	"invise-backend/pkg/mail"
	"invise-backend/pkg/password"
	"invise-backend/pkg/ulid"
)

type AuthUsecaseI interface {
	Register(ctx context.Context, req RegisterRequest) error
	Verify(ctx context.Context, req VerifyRequest) error
	Login(ctx context.Context, req LoginRequest) (*TokenResponse, error)
}

type authUsecase struct {
	userRepo UserRepositoryI
	jwt      jwt.JwtI
	password password.BcryptI
	ulid     ulid.GeneratorI
	mailer   mail.MailerI
	rdb      *redis.Client
	otpCfg   config.OTPConfig
}

func NewAuthUsecase(
	userRepo UserRepositoryI,
	jwt jwt.JwtI,
	password password.BcryptI,
	ulid ulid.GeneratorI,
	mailer mail.MailerI,
	rdb *redis.Client,
	otpCfg config.OTPConfig,
) AuthUsecaseI {
	return &authUsecase{
		userRepo: userRepo,
		jwt:      jwt,
		password: password,
		ulid:     ulid,
		mailer:   mailer,
		rdb:      rdb,
		otpCfg:   otpCfg,
	}
}

func (u *authUsecase) Register(ctx context.Context, req RegisterRequest) error {
	existing, err := u.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return pkgerr.InternalServerError("REGISTRATION_FAILED", "could not check existing user")
	}
	if existing != nil {
		return ErrEmailAlreadyExists
	}

	hash, err := u.password.Hash(req.Password)
	if err != nil {
		return pkgerr.InternalServerError("HASHING_FAILED", "could not hash password")
	}

	id, err := u.ulid.Generate()
	if err != nil {
		return pkgerr.InternalServerError("ID_GENERATION_FAILED", "could not generate user ID")
	}

	user := &User{
		ID:           id,
		Email:        req.Email,
		PasswordHash: hash,
		Verified:     false,
	}
	if err := u.userRepo.Create(ctx, user); err != nil {
		return pkgerr.InternalServerError("REGISTRATION_FAILED", "could not create user")
	}

	otp := fmt.Sprintf("%06d", rand.Intn(1000000))
	key := fmt.Sprintf("otp:%s", req.Email)
	ttl := time.Duration(u.otpCfg.TTLMinutes) * time.Minute
	if err := u.rdb.Set(ctx, key, otp, ttl).Err(); err != nil {
		return pkgerr.InternalServerError("OTP_STORAGE_FAILED", "could not store OTP")
	}

	if err := u.mailer.SendOTP(req.Email, otp); err != nil {
		fmt.Println(err.Error())
		return pkgerr.InternalServerError("EMAIL_FAILED", "could not send verification email")
	}

	return nil
}

func (u *authUsecase) Verify(ctx context.Context, req VerifyRequest) error {
	user, err := u.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return pkgerr.InternalServerError("VERIFICATION_FAILED", "could not look up user")
	}
	if user == nil {
		return ErrUserNotFound
	}
	if user.Verified {
		return nil
	}

	key := fmt.Sprintf("otp:%s", req.Email)
	stored, err := u.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return ErrInvalidOTP
	}
	if err != nil {
		return pkgerr.InternalServerError("OTP_LOOKUP_FAILED", "could not retrieve OTP")
	}
	if stored != req.OTP {
		return ErrInvalidOTP
	}

	if err := u.userRepo.UpdateVerified(ctx, user.ID, true); err != nil {
		return pkgerr.InternalServerError("VERIFICATION_FAILED", "could not verify account")
	}
	u.rdb.Del(ctx, key)

	return nil
}

func (u *authUsecase) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := u.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, pkgerr.InternalServerError("LOGIN_FAILED", "could not look up user")
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if !user.Verified {
		return nil, ErrAccountNotVerified
	}

	if err := u.password.Compare(user.PasswordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := u.jwt.Generate(user.ID, user.Email)
	if err != nil {
		return nil, pkgerr.InternalServerError("TOKEN_GENERATION_FAILED", "could not generate token")
	}

	return &TokenResponse{AccessToken: token}, nil
}
