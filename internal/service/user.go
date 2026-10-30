// Package service provides business logic layer.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ilaziness/app-tpl/internal/cache"
	"github.com/ilaziness/app-tpl/internal/crypto"
	"github.com/ilaziness/app-tpl/internal/dto"
	"github.com/ilaziness/app-tpl/internal/errcode"
	"github.com/ilaziness/app-tpl/internal/model"
	"github.com/ilaziness/app-tpl/internal/repository"
)

// UserService defines the contract for user business logic.
type UserService interface {
	Create(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error)
	GetByID(ctx context.Context, id int64) (*dto.UserResponse, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	ValidateCredentials(ctx context.Context, email, password string) (*model.User, error)
	Update(ctx context.Context, id int64, req *dto.UpdateUserRequest) (*dto.UserResponse, error)
	Delete(ctx context.Context, id int64) error
	ListWithCount(ctx context.Context, limit, offset int64) ([]*dto.UserResponse, int64, error)
}

// userService handles user business logic.
type userService struct {
	userRepo repository.UserRepository
	cache    *cache.Manager
}

// 确保 userService 实现了 UserService 接口
var _ UserService = (*userService)(nil)

// NewUserService creates a new user service. The cache.Manager enables
// cache-aside for read paths; pass a Manager backed by NopCache to disable
// caching (every Get yields ErrCacheMiss, so reads fall back to the repo).
func NewUserService(userRepo repository.UserRepository, cacheMgr *cache.Manager) UserService {
	return &userService{userRepo: userRepo, cache: cacheMgr}
}

// Create creates a new user.
func (s *userService) Create(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error) {
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, errcode.UserAlreadyExists
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check user email: %w", err)
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Email:    req.Email,
		Password: hashedPassword,
		Name:     req.Name,
		Phone:    req.Phone,
		Status:   1,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return toUserResponse(user), nil
}

// GetByID retrieves a user by ID using cache-aside (public fields only; no password).
func (s *userService) GetByID(ctx context.Context, id int64) (*dto.UserResponse, error) {
	if cached, err := s.cache.GetUserByID(ctx, id); err == nil {
		return toUserResponse(cached), nil
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.Wrap(errcode.UserNotFound, err)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	_ = s.cache.SetUserByID(ctx, id, publicUser(user))
	return toUserResponse(user), nil
}

// GetByEmail retrieves a user by email from the repository (includes password hash
// for auth flows). Prefer GetByID for HTTP responses.
func (s *userService) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.Wrap(errcode.UserNotFound, err)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// ValidateCredentials validates user credentials (email and password).
// Loads from the repository so the password hash is available (not from cache).
func (s *userService) ValidateCredentials(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, errcode.AuthFailed
	}

	if user.Status != 1 {
		return nil, errcode.UserDisabled
	}

	if err := crypto.CheckPassword(password, user.Password); err != nil {
		return nil, errcode.AuthFailed
	}

	return user, nil
}

// Update updates an existing user.
func (s *userService) Update(ctx context.Context, id int64, req *dto.UpdateUserRequest) (*dto.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errcode.Wrap(errcode.UserNotFound, err)
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.Wrap(errcode.UserNotFound, err)
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.invalidateUserCache(ctx, id, user.Email)
	return toUserResponse(user), nil
}

// Delete deletes a user by ID.
func (s *userService) Delete(ctx context.Context, id int64) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return errcode.Wrap(errcode.UserNotFound, err)
	}

	if err := s.userRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errcode.Wrap(errcode.UserNotFound, err)
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.invalidateUserCache(ctx, id, user.Email)
	return nil
}

// invalidateUserCache removes both the by-id and by-email cache entries.
func (s *userService) invalidateUserCache(ctx context.Context, id int64, email string) {
	_ = s.cache.DelUserByID(ctx, id)
	if email != "" {
		_ = s.cache.DelUserByEmail(ctx, email)
	}
}

// ListWithCount retrieves a list of users with pagination and returns total count.
func (s *userService) ListWithCount(ctx context.Context, limit, offset int64) ([]*dto.UserResponse, int64, error) {
	users, err := s.userRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	return toUserResponses(users), total, nil
}

// publicUser returns a shallow copy without the password hash for caching.
func publicUser(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	cp := *user
	cp.Password = ""
	return &cp
}

func toUserResponse(user *model.User) *dto.UserResponse {
	if user == nil {
		return nil
	}
	return &dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Phone:     user.Phone,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toUserResponses(users []*model.User) []*dto.UserResponse {
	out := make([]*dto.UserResponse, len(users))
	for i, u := range users {
		out[i] = toUserResponse(u)
	}
	return out
}
