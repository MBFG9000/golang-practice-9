package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MBFG9000/golang-practice-9/internal/idempotency"
	"github.com/MBFG9000/golang-practice-9/internal/middlewares"
	"github.com/MBFG9000/golang-practice-9/internal/payment"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	if len(os.Args) < 2 {
		logger.Print("Usage: go run ./cmd/payment-sim [retry|idempotency|all]")
		return
	}

	switch os.Args[1] {
	case "retry":
		runRetryDemo(logger)
	case "idempotency":
		runIdempotencyDemo(logger)
	case "all":
		runRetryDemo(logger)
		runIdempotencyDemo(logger)
	default:
		logger.Printf("Unknown mode %q. Use retry, idempotency, or all.", os.Args[1])
	}
}

func runRetryDemo(logger *log.Logger) {
	logger.Print("Running retry demo")
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	client := &payment.RetryClient{
		HTTPClient: server.Client(),
		MaxRetries: 5,
		BaseDelay:  500 * time.Millisecond,
		Logger:     logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload := []byte(`{"amount": 1000}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewReader(payload))
	if err != nil {
		logger.Printf("Failed to build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}

	resp, err := client.ExecutePayment(ctx, req)
	if err != nil {
		logger.Printf("Retry demo failed: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Printf("Retry demo status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	logger.Printf("Retry demo response: %s", strings.TrimSpace(string(body)))
}

func runIdempotencyDemo(logger *log.Logger) {
	logger.Print("Running idempotency demo")
	db, err := sqlx.Open("sqlite", "file:idempotency.db?mode=memory&cache=shared")
	if err != nil {
		logger.Printf("Failed to open idempotency database: %v", err)
		return
	}
	defer db.Close()

	store := idempotency.NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		logger.Printf("Failed to ensure idempotency schema: %v", err)
		return
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.POST("/payments", middlewares.IdempotencyMiddleware(store), payment.PaymentHandler)
	server := httptest.NewServer(router)
	defer server.Close()

	runDoubleClickAttack(server.URL, logger)
}

func runDoubleClickAttack(baseURL string, logger *log.Logger) {
	const requests = 8
	const key = "payment-1000"

	client := &http.Client{Timeout: 5 * time.Second}
	payload := []byte(`{"amount": 1000}`)

	var wg sync.WaitGroup
	var once sync.Once
	var firstSuccess string

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, baseURL+"/payments", bytes.NewReader(payload))
			if err != nil {
				logger.Printf("Request %d build error: %v", id+1, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", key)

			resp, err := client.Do(req)
			if err != nil {
				logger.Printf("Request %d failed: %v", id+1, err)
				return
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			bodyText := strings.TrimSpace(string(body))

			switch resp.StatusCode {
			case http.StatusConflict:
				logger.Printf("Request %d: 409 Conflict (already processing)", id+1)
			case http.StatusOK:
				once.Do(func() { firstSuccess = bodyText })
				logger.Printf("Request %d: 200 OK %s", id+1, bodyText)
			default:
				logger.Printf("Request %d: status %d %s", id+1, resp.StatusCode, bodyText)
			}
		}(i)
	}

	wg.Wait()

	replayReq, err := http.NewRequest(http.MethodPost, baseURL+"/payments", bytes.NewReader(payload))
	if err != nil {
		logger.Printf("Replay request build error: %v", err)
		return
	}
	replayReq.Header.Set("Content-Type", "application/json")
	replayReq.Header.Set("Idempotency-Key", key)

	replayResp, err := client.Do(replayReq)
	if err != nil {
		logger.Printf("Replay request failed: %v", err)
		return
	}
	replayBody, _ := io.ReadAll(replayResp.Body)
	_ = replayResp.Body.Close()
	replayText := strings.TrimSpace(string(replayBody))
	logger.Printf("Replay request: %d %s", replayResp.StatusCode, replayText)
	if firstSuccess != "" && replayText == firstSuccess {
		logger.Printf("Replay matched stored response")
	}
}
