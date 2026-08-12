// Package service provides business logic layer.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	ValidateCredentials(ctx context.Context, email, password string) (*model.User, error)
	Update(ctx context.Context, id int64, req *dto.UpdateUserRequest) (*dto.UserResponse, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*model.User, error)
	ListWithCount(ctx context.Context, limit, offset int) ([]*model.User, int, error)
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
	// 检查邮箱是否已存在
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, errcode.UserAlreadyExists
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check user email: %w", err)
	}

	// 加密密码
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

// GetByID retrieves a user by ID using cache-aside: the cache is checked
// first, and on a miss the user is loaded from the repository and written
// back to the cache.
func (s *userService) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if cached, err := s.cache.GetUserByID(ctx, id); err == nil {
		return cached, nil
	}
	// On cache miss or transient cache error, fall through to the repository.

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	_ = s.cache.SetUserByID(ctx, id, user)
	return user, nil
}

// GetByEmail retrieves a user by email using cache-aside.
func (s *userService) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if cached, err := s.cache.GetUserByEmail(ctx, email); err == nil {
		return cached, nil
	}
	// On cache miss or transient cache error, fall through to the repository.

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	_ = s.cache.SetUserByEmail(ctx, email, user)
	return user, nil
}

// ValidateCredentials validates user credentials (email and password).
// It reuses the cached GetByEmail path for the user lookup.
func (s *userService) ValidateCredentials(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.GetByEmail(ctx, email)
	if err != nil {
		return nil, errcode.AuthFailed
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, errcode.UserDisabled
	}

	// 验证密码
	if err := crypto.CheckPassword(password, user.Password); err != nil {
		return nil, errcode.AuthFailed
	}

	return user, nil
}

// Update updates an existing user.
func (s *userService) Update(ctx context.Context, id int64, req *dto.UpdateUserRequest) (*dto.UserResponse, error) {
	// 检查用户是否存在
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
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.Wrap(errcode.UserNotFound, err)
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// 失效双键缓存，防止脏读
	s.invalidateUserCache(ctx, id, user.Email)
	return toUserResponse(user), nil
}

// Delete deletes a user by ID.
func (s *userService) Delete(ctx context.Context, id int64) error {
	// 检查用户是否存在（同时获取 email 用于缓存失效）
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

	// 失效双键缓存
	s.invalidateUserCache(ctx, id, user.Email)
	return nil
}

// invalidateUserCache removes both the by-id and by-email cache entries.
// Cache errors are ignored: a failed invalidation is not fatal; the entry
// will simply expire according to its TTL.
func (s *userService) invalidateUserCache(ctx context.Context, id int64, email string) {
	_ = s.cache.DelUserByID(ctx, id)
	if email != "" {
		_ = s.cache.DelUserByEmail(ctx, email)
	}
}

// List retrieves a list of users with pagination.
func (s *userService) List(ctx context.Context, limit, offset int) ([]*model.User, error) {
	users, err := s.userRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}

// ListWithCount retrieves a list of users with pagination and returns total count.
func (s *userService) ListWithCount(ctx context.Context, limit, offset int) ([]*model.User, int, error) {
	users, err := s.userRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	return users, total, nil
}

func toUserResponse(user *model.User) *dto.UserResponse {
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
