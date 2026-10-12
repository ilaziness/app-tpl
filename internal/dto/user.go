// Package dto provides data transfer objects.
package dto

import "time"

// CreateUserRequest represents the request to create a user.
type CreateUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone" binding:"omitempty"`
}

// UpdateUserRequest represents the request to update a user.
type UpdateUserRequest struct {
	ID     int64  `uri:"id" binding:"required"`
	Name   string `json:"name" binding:"omitempty"`
	Phone  string `json:"phone" binding:"omitempty"`
	Status *int   `json:"status" binding:"omitempty,min=0,max=10"`
}

// ListUsersRequest represents the request to list users.
type ListUsersRequest struct {
	PaginationRequest
}

// GetUserRequest represents the request to get a user by ID.
type GetUserRequest struct {
	ID int64 `uri:"id" binding:"required"`
}

// DeleteUserRequest represents the request to delete a user by ID.
type DeleteUserRequest struct {
	ID int64 `uri:"id" binding:"required"`
}

// UserResponse represents the user response.
type UserResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
