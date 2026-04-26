package controller

import (
	"github.com/MBFG9000/golang-practice-9/internal/middlewares"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, con *Controller) {
	router.Use(middlewares.RateLimiterMiddleware())

	router.POST("/register", con.RegisterUser)
	router.POST("/login", con.Login)

	users := router.Group("/users")
	users.Use(middlewares.JWTAuthMiddleware())
	{
		users.GET("/me", con.GetMe)
		users.PATCH("/promote/:id", middlewares.RoleMiddleware("admin"), con.PromoteUser)
	}
}
