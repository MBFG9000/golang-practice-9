package middlewares

import (
	"bytes"
	"log"
	"net/http"

	"github.com/MBFG9000/golang-practice-9/internal/idempotency"
	"github.com/gin-gonic/gin"
)

type idempotencyWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *idempotencyWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *idempotencyWriter) Body() []byte {
	return w.body.Bytes()
}

func IdempotencyMiddleware(store *idempotency.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency store not configured"})
			c.Abort()
			return
		}

		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header required"})
			c.Abort()
			return
		}

		created, record, err := store.TryStart(c.Request.Context(), key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency storage error"})
			c.Abort()
			return
		}

		if !created {
			if record == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency record missing"})
				c.Abort()
				return
			}

			switch record.Status {
			case idempotency.StatusProcessing:
				c.JSON(http.StatusConflict, gin.H{"error": "request is already processing"})
				c.Abort()
				return
			case idempotency.StatusCompleted:
				c.Data(record.ResponseCode, "application/json", []byte(record.ResponseBody))
				c.Abort()
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid idempotency status"})
				c.Abort()
				return
			}
		}

		writer := &idempotencyWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		if err := store.Complete(c.Request.Context(), key, writer.Status(), writer.Body()); err != nil {
			log.Printf("failed to store idempotency response for %s: %v", key, err)
		}
	}
}
