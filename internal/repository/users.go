package repository

import (
	"database/sql"
	"fmt"

	"github.com/MBFG9000/golang-practice-9/internal/database"
	"github.com/MBFG9000/golang-practice-9/internal/entity"
	"github.com/google/uuid"
)

type UserRepository struct {
	conn *database.Dialect
}

func NewUserRepository(conn *database.Dialect) *UserRepository {
	return &UserRepository{
		conn: conn,
	}
}

func (r *UserRepository) CreateUser(user *entity.User) error {
	query := `
		INSERT INTO users (
			username,
			email,
			password,
			role,
			verified
		) VALUES (
			$1,
			$2,
			$3,
			$4,
			$5
		)
		RETURNING id::text
	`

	var rawID string
	if err := r.conn.DB.QueryRow(query, user.Username, user.Email, user.Password, user.Role, user.Verified).Scan(&rawID); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return fmt.Errorf("parse created user id: %w", err)
	}
	user.ID = id

	return nil
}

func (r *UserRepository) GetUserByUsername(username string) (*entity.User, error) {
	query := `
		SELECT id, username, email, password, role, verified
		FROM users
		WHERE username = $1
	`

	var user entity.User
	if err := r.conn.DB.Get(&user, query, username); err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) GetUserByID(id uuid.UUID) (*entity.User, error) {
	query := `
		SELECT id, username, email, password, role, verified
		FROM users
		WHERE id = $1
	`

	var user entity.User
	if err := r.conn.DB.Get(&user, query, id); err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) UpdateUserRole(id uuid.UUID, role string) error {
	query := `
		UPDATE users
		SET role = $1
		WHERE id = $2
	`

	res, err := r.conn.DB.Exec(query, role, id)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user role rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("update user role: %w", sql.ErrNoRows)
	}

	return nil
}
