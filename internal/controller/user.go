package controller

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/MBFG9000/golang-practice-9/internal/entity"
	"github.com/MBFG9000/golang-practice-9/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Controller struct {
	userService *service.UserService
}

func NewController(userService *service.UserService) *Controller {
	return &Controller{userService: userService}
}

func (con *Controller) RegisterUser(c *gin.Context) {
	var createUserDTO entity.CreateUserDTO
	if err := c.ShouldBindJSON(&createUserDTO); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := con.userService.RegisterUser(createUserDTO)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "User registered successfully",
		"session_id": uuid.New().String(),
		"user":       entity.ToUserResponseDTO(user),
	})
}

func (con *Controller) Login(c *gin.Context) {
	var loginDTO entity.LoginDTO
	if err := c.ShouldBindJSON(&loginDTO); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := con.userService.Login(loginDTO)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (con *Controller) GetMe(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := con.userService.GetMe(userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, sql.ErrNoRows):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, entity.ToUserResponseDTO(user))
}

func (con *Controller) PromoteUser(c *gin.Context) {
	id := c.Param("id")

	user, err := con.userService.PromoteUser(id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, sql.ErrNoRows):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user promoted to admin",
		"user":    entity.ToUserResponseDTO(user),
	})
}
