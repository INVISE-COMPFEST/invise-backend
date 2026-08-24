package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"invise-backend/internal/app/auth"
	"invise-backend/internal/bootstrap/config"
	pkgerr "invise-backend/pkg/errors"
	"invise-backend/pkg/jwt"
)

type stubUserRepository struct {
	createFn         func(ctx context.Context, user *auth.User) error
	findByEmailFn    func(ctx context.Context, email string) (*auth.User, error)
	updateVerifiedFn func(ctx context.Context, id string, verified bool) error
}

func (s *stubUserRepository) Create(ctx context.Context, user *auth.User) error {
	if s.createFn != nil {
		return s.createFn(ctx, user)
	}
	return nil
}

func (s *stubUserRepository) FindByEmail(ctx context.Context, email string) (*auth.User, error) {
	if s.findByEmailFn != nil {
		return s.findByEmailFn(ctx, email)
	}
	return nil, nil
}

func (s *stubUserRepository) UpdateVerified(ctx context.Context, id string, verified bool) error {
	if s.updateVerifiedFn != nil {
		return s.updateVerifiedFn(ctx, id, verified)
	}
	return nil
}

type stubJwt struct {
	generateFn func(userID, email string) (string, error)
	validateFn func(tokenString string) (*jwt.Claims, error)
}

func (s *stubJwt) Generate(userID, email string) (string, error) {
	if s.generateFn != nil {
		return s.generateFn(userID, email)
	}
	return "token", nil
}

func (s *stubJwt) Validate(tokenString string) (*jwt.Claims, error) {
	if s.validateFn != nil {
		return s.validateFn(tokenString)
	}
	return &jwt.Claims{UserID: "uid", Email: "test@example.com"}, nil
}

type stubPassword struct {
	hashFn    func(password string) (string, error)
	compareFn func(hashed, password string) error
}

func (s *stubPassword) Hash(password string) (string, error) {
	if s.hashFn != nil {
		return s.hashFn(password)
	}
	return "hashed_password", nil
}

func (s *stubPassword) Compare(hashed, password string) error {
	if s.compareFn != nil {
		return s.compareFn(hashed, password)
	}
	return nil
}

type stubULID struct {
	generateFn func() (string, error)
}

func (s *stubULID) Generate() (string, error) {
	if s.generateFn != nil {
		return s.generateFn()
	}
	return "01ARZ3NDEKTSV4RRFFQ69G5FAV", nil
}

type stubMailer struct {
	sendOTPFn func(to, otp string) error
}

func (s *stubMailer) SendOTP(to, otp string) error {
	if s.sendOTPFn != nil {
		return s.sendOTPFn(to, otp)
	}
	return nil
}

func TestNewAuthUsecase(t *testing.T) {
	rdb, _ := redismock.NewClientMock()
	uc := auth.NewAuthUsecase(
		&stubUserRepository{},
		&stubJwt{},
		&stubPassword{},
		&stubULID{},
		&stubMailer{},
		rdb,
		config.OTPConfig{TTLMinutes: 5},
	)
	assert.NotNil(t, uc)
}

func TestAuthUsecase_Register(t *testing.T) {
	ctx := context.Background()
	otpCfg := config.OTPConfig{TTLMinutes: 5}

	t.Run("Success", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.Regexp().ExpectSet(`^otp:test@example\.com$`, `^[0-9]{6}$`, 5*time.Minute).SetVal("OK")

		var createdUser *auth.User
		var sentTo, sentOTP string

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, user *auth.User) error {
				createdUser = user
				return nil
			},
		}
		passSvc := &stubPassword{
			hashFn: func(password string) (string, error) {
				return "hashed_pw", nil
			},
		}
		ulidSvc := &stubULID{
			generateFn: func() (string, error) {
				return "user-id-123", nil
			},
		}
		mailerSvc := &stubMailer{
			sendOTPFn: func(to, otp string) error {
				sentTo = to
				sentOTP = otp
				return nil
			},
		}

		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, passSvc, ulidSvc, mailerSvc, rdb, otpCfg)
		err := uc.Register(ctx, auth.RegisterRequest{
			Email:    "test@example.com",
			Password: "password123",
		})

		assert.NoError(t, err)
		require.NotNil(t, createdUser)
		assert.Equal(t, "user-id-123", createdUser.ID)
		assert.Equal(t, "test@example.com", createdUser.Email)
		assert.Equal(t, "hashed_pw", createdUser.PasswordHash)
		assert.False(t, createdUser.Verified)
		assert.Equal(t, "test@example.com", sentTo)
		assert.Regexp(t, `^[0-9]{6}$`, sentOTP)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("FindByEmail Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, errors.New("db connection failed")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "REGISTRATION_FAILED", appErr.Code)
		assert.Equal(t, "could not check existing user", appErr.Message)
	})

	t.Run("Email Already Exists", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "existing-id", Email: "test@example.com"}, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)
		assert.Equal(t, auth.ErrEmailAlreadyExists, err)
	})

	t.Run("Password Hashing Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
		}
		passSvc := &stubPassword{
			hashFn: func(password string) (string, error) {
				return "", errors.New("bcrypt failure")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, passSvc, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "HASHING_FAILED", appErr.Code)
		assert.Equal(t, "could not hash password", appErr.Message)
	})

	t.Run("ULID Generation Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
		}
		ulidSvc := &stubULID{
			generateFn: func() (string, error) {
				return "", errors.New("entropy source depleted")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, ulidSvc, &stubMailer{}, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "ID_GENERATION_FAILED", appErr.Code)
		assert.Equal(t, "could not generate user ID", appErr.Message)
	})

	t.Run("User Create Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, user *auth.User) error {
				return errors.New("insert failed")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "REGISTRATION_FAILED", appErr.Code)
		assert.Equal(t, "could not create user", appErr.Message)
	})

	t.Run("OTP Storage Error", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.Regexp().ExpectSet(`^otp:test@example\.com$`, `^[0-9]{6}$`, 5*time.Minute).SetErr(errors.New("redis down"))

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, user *auth.User) error {
				return nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "OTP_STORAGE_FAILED", appErr.Code)
		assert.Equal(t, "could not store OTP", appErr.Message)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Mailer SendOTP Error", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.Regexp().ExpectSet(`^otp:test@example\.com$`, `^[0-9]{6}$`, 5*time.Minute).SetVal("OK")

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, user *auth.User) error {
				return nil
			},
		}
		mailerSvc := &stubMailer{
			sendOTPFn: func(to, otp string) error {
				return errors.New("smtp connection failed")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, mailerSvc, rdb, otpCfg)

		err := uc.Register(ctx, auth.RegisterRequest{Email: "test@example.com", Password: "password123"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "EMAIL_FAILED", appErr.Code)
		assert.Equal(t, "could not send verification email", appErr.Message)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAuthUsecase_Verify(t *testing.T) {
	ctx := context.Background()
	otpCfg := config.OTPConfig{TTLMinutes: 5}

	t.Run("Success", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.ExpectGet("otp:test@example.com").SetVal("123456")
		mock.ExpectDel("otp:test@example.com").SetVal(1)

		var updatedID string
		var updatedVerified bool

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "user-123", Email: "test@example.com", Verified: false}, nil
			},
			updateVerifiedFn: func(ctx context.Context, id string, verified bool) error {
				updatedID = id
				updatedVerified = verified
				return nil
			},
		}

		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)
		err := uc.Verify(ctx, auth.VerifyRequest{
			Email: "test@example.com",
			OTP:   "123456",
		})

		assert.NoError(t, err)
		assert.Equal(t, "user-123", updatedID)
		assert.True(t, updatedVerified)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("FindByEmail Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, errors.New("db query failed")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "VERIFICATION_FAILED", appErr.Code)
		assert.Equal(t, "could not look up user", appErr.Message)
	})

	t.Run("User Not Found", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		require.Error(t, err)
		assert.Equal(t, auth.ErrUserNotFound, err)
	})

	t.Run("Already Verified", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "user-123", Email: "test@example.com", Verified: true}, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("OTP Expired / redis.Nil", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.ExpectGet("otp:test@example.com").RedisNil()

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "user-123", Email: "test@example.com", Verified: false}, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		require.Error(t, err)
		assert.Equal(t, auth.ErrInvalidOTP, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("OTP Retrieval Error", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.ExpectGet("otp:test@example.com").SetErr(errors.New("redis timeout"))

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "user-123", Email: "test@example.com", Verified: false}, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "OTP_LOOKUP_FAILED", appErr.Code)
		assert.Equal(t, "could not retrieve OTP", appErr.Message)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("OTP Mismatch", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.ExpectGet("otp:test@example.com").SetVal("654321")

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "user-123", Email: "test@example.com", Verified: false}, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		require.Error(t, err)
		assert.Equal(t, auth.ErrInvalidOTP, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateVerified Error", func(t *testing.T) {
		rdb, mock := redismock.NewClientMock()
		mock.ExpectGet("otp:test@example.com").SetVal("123456")

		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{ID: "user-123", Email: "test@example.com", Verified: false}, nil
			},
			updateVerifiedFn: func(ctx context.Context, id string, verified bool) error {
				return errors.New("db update failed")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		err := uc.Verify(ctx, auth.VerifyRequest{Email: "test@example.com", OTP: "123456"})
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "VERIFICATION_FAILED", appErr.Code)
		assert.Equal(t, "could not verify account", appErr.Message)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAuthUsecase_Login(t *testing.T) {
	ctx := context.Background()
	otpCfg := config.OTPConfig{TTLMinutes: 5}

	t.Run("Success", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{
					ID:           "user-123",
					Email:        "test@example.com",
					PasswordHash: "$2a$10$dummyhash",
					Verified:     true,
				}, nil
			},
		}
		passSvc := &stubPassword{
			compareFn: func(hashed, password string) error {
				assert.Equal(t, "$2a$10$dummyhash", hashed)
				assert.Equal(t, "password123", password)
				return nil
			},
		}
		jwtSvc := &stubJwt{
			generateFn: func(userID, email string) (string, error) {
				assert.Equal(t, "user-123", userID)
				assert.Equal(t, "test@example.com", email)
				return "generated-jwt-token", nil
			},
		}

		uc := auth.NewAuthUsecase(userRepo, jwtSvc, passSvc, &stubULID{}, &stubMailer{}, rdb, otpCfg)
		res, err := uc.Login(ctx, auth.LoginRequest{
			Email:    "test@example.com",
			Password: "password123",
		})

		assert.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "generated-jwt-token", res.AccessToken)
	})

	t.Run("FindByEmail Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, errors.New("db error")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		res, err := uc.Login(ctx, auth.LoginRequest{Email: "test@example.com", Password: "password123"})
		assert.Nil(t, res)
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "LOGIN_FAILED", appErr.Code)
		assert.Equal(t, "could not look up user", appErr.Message)
	})

	t.Run("User Not Found", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return nil, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		res, err := uc.Login(ctx, auth.LoginRequest{Email: "test@example.com", Password: "password123"})
		assert.Nil(t, res)
		require.Error(t, err)
		assert.Equal(t, auth.ErrInvalidCredentials, err)
	})

	t.Run("Account Not Verified", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{
					ID:           "user-123",
					Email:        "test@example.com",
					PasswordHash: "$2a$10$dummyhash",
					Verified:     false,
				}, nil
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		res, err := uc.Login(ctx, auth.LoginRequest{Email: "test@example.com", Password: "password123"})
		assert.Nil(t, res)
		require.Error(t, err)
		assert.Equal(t, auth.ErrAccountNotVerified, err)
	})

	t.Run("Invalid Password", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{
					ID:           "user-123",
					Email:        "test@example.com",
					PasswordHash: "$2a$10$dummyhash",
					Verified:     true,
				}, nil
			},
		}
		passSvc := &stubPassword{
			compareFn: func(hashed, password string) error {
				return errors.New("mismatch")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, &stubJwt{}, passSvc, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		res, err := uc.Login(ctx, auth.LoginRequest{Email: "test@example.com", Password: "wrongpassword"})
		assert.Nil(t, res)
		require.Error(t, err)
		assert.Equal(t, auth.ErrInvalidCredentials, err)
	})

	t.Run("Token Generation Error", func(t *testing.T) {
		rdb, _ := redismock.NewClientMock()
		userRepo := &stubUserRepository{
			findByEmailFn: func(ctx context.Context, email string) (*auth.User, error) {
				return &auth.User{
					ID:           "user-123",
					Email:        "test@example.com",
					PasswordHash: "$2a$10$dummyhash",
					Verified:     true,
				}, nil
			},
		}
		jwtSvc := &stubJwt{
			generateFn: func(userID, email string) (string, error) {
				return "", errors.New("signing error")
			},
		}
		uc := auth.NewAuthUsecase(userRepo, jwtSvc, &stubPassword{}, &stubULID{}, &stubMailer{}, rdb, otpCfg)

		res, err := uc.Login(ctx, auth.LoginRequest{Email: "test@example.com", Password: "password123"})
		assert.Nil(t, res)
		require.Error(t, err)

		var appErr *pkgerr.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "TOKEN_GENERATION_FAILED", appErr.Code)
		assert.Equal(t, "could not generate token", appErr.Message)
	})
}
