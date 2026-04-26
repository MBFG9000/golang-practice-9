package payment

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentResponse struct {
	Status        string `json:"status"`
	Amount        int    `json:"amount"`
	TransactionID string `json:"transaction_id"`
}

func PaymentHandler(c *gin.Context) {
	log.Print("Processing started")
	time.Sleep(2 * time.Second)

	response := PaymentResponse{
		Status:        "paid",
		Amount:        1000,
		TransactionID: "uuid-" + uuid.New().String(),
	}

	c.JSON(http.StatusOK, response)
}
