package middlewares

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MBFG9000/golang-practice-9/internal/utils"
	"github.com/gin-gonic/gin"
)

const (
	rateLimitRequests = 5
	rateLimitWindow   = time.Minute
)

type visitorData struct {
	count       int
	windowStart time.Time
}

var (
	visitors   = make(map[string]*visitorData)
	visitorsMu sync.Mutex
)

func RateLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rateLimitKey(c)
		now := time.Now()

		visitorsMu.Lock()
		visitor, exists := visitors[key]
		if !exists || now.Sub(visitor.windowStart) >= rateLimitWindow {
			visitors[key] = &visitorData{
				count:       1,
				windowStart: now,
			}
			visitorsMu.Unlock()
			c.Next()
			return
		}

		if visitor.count >= rateLimitRequests {
			visitorsMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			c.Abort()
			return
		}

		visitor.count++
		visitorsMu.Unlock()

		c.Next()
	}
}

func rateLimitKey(c *gin.Context) string {
	if userID := c.GetString("user_id"); userID != "" {
		return "user:" + userID
	}

	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if claims, err := utils.ParseJWT(token); err == nil && claims.UserID != "" {
			return "user:" + claims.UserID
		}
	}

	return "ip:" + c.ClientIP()
}
