package service

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MBFG9000/golang-practice-9/internal/entity"
	"github.com/MBFG9000/golang-practice-9/internal/repository"
	"github.com/MBFG9000/golang-practice-9/internal/utils"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidUserID      = errors.New("invalid user id")
)

type UserService struct {
	repo repository.UserRepoInterface
}

func NewUserService(repo repository.UserRepoInterface) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) RegisterUser(dto entity.CreateUserDTO) (*entity.User, error) {
	hashedPassword, err := utils.HashPassword(dto.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &entity.User{
		Username: dto.Username,
		Email:    dto.Email,
		Password: hashedPassword,
		Role:     "user",
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) Login(dto entity.LoginDTO) (string, error) {
	user, err := s.repo.GetUserByUsername(dto.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := utils.ComparePassword(user.Password, dto.Password); err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := utils.GenerateJWT(user.ID.String(), user.Role)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func (s *UserService) GetMe(userID string) (*entity.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalidUserID
	}

	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) PromoteUser(userID string) (*entity.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalidUserID
	}

	if err := s.repo.UpdateUserRole(id, "admin"); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	return user, nil
}
