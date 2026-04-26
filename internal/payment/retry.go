package payment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

const defaultBaseDelay = 500 * time.Millisecond

type RetryClient struct {
	HTTPClient *http.Client
	MaxRetries int
	BaseDelay  time.Duration
	Backoff    func(attempt int) time.Duration
	Logger     *log.Logger

	rngMu sync.Mutex
	rng   *rand.Rand
}

func IsRetryable(resp *http.Response, err error) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		var netErr net.Error
		if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
			return true
		}
		return false
	}

	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusNotFound:
		return false
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func CalculateBackoff(attempt int) time.Duration {
	return calculateBackoff(attempt, defaultBaseDelay, rand.Int63n)
}

func (c *RetryClient) ExecutePayment(ctx context.Context, req *http.Request) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if req == nil {
		return nil, errors.New("request is required")
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	maxRetries := c.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	logger := c.Logger
	if logger == nil {
		logger = log.Default()
	}

	getBody, err := prepareBody(req)
	if err != nil {
		return nil, err
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		attemptReq, err := cloneRequest(req, ctx, getBody)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(attemptReq)
		if err == nil && resp != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			logger.Printf("Attempt %d: Success!", attempt)
			return resp, nil
		}

		if IsRetryable(resp, err) && attempt < maxRetries {
			closeResponse(resp)

			wait := c.backoff(attempt)
			if err != nil {
				logger.Printf("Attempt %d failed: %v; waiting %s...", attempt, err, wait)
			} else if resp != nil {
				logger.Printf("Attempt %d failed: status %d %s; waiting %s...", attempt, resp.StatusCode, http.StatusText(resp.StatusCode), wait)
			} else {
				logger.Printf("Attempt %d failed: no response; waiting %s...", attempt, wait)
			}

			if wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
			}
			continue
		}

		if err == nil && resp != nil {
			return resp, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		if err == nil {
			return resp, errors.New("request failed")
		}
		return resp, err
	}

	return nil, errors.New("retry attempts exhausted")
}

func (c *RetryClient) backoff(attempt int) time.Duration {
	if c.Backoff != nil {
		return c.Backoff(attempt)
	}

	baseDelay := c.BaseDelay
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}

	c.rngMu.Lock()
	if c.rng == nil {
		c.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	jitter := calculateBackoff(attempt, baseDelay, c.rng.Int63n)
	c.rngMu.Unlock()

	return jitter
}

func calculateBackoff(attempt int, baseDelay time.Duration, jitter func(int64) int64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}

	maxDelay := baseDelay
	for i := 1; i < attempt; i++ {
		if maxDelay > time.Duration(maxInt64/2) {
			maxDelay = time.Duration(maxInt64)
			break
		}
		maxDelay *= 2
	}

	if maxDelay <= 0 {
		return 0
	}

	return time.Duration(jitter(int64(maxDelay)))
}

func prepareBody(req *http.Request) (func() (io.ReadCloser, error), error) {
	if req.GetBody != nil {
		return req.GetBody, nil
	}
	if req.Body == nil {
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if err := req.Body.Close(); err != nil {
		return nil, err
	}

	copied := append([]byte(nil), bodyBytes...)
	return func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(copied)), nil
	}, nil
}

func cloneRequest(req *http.Request, ctx context.Context, getBody func() (io.ReadCloser, error)) (*http.Request, error) {
	clone := req.Clone(ctx)
	if getBody == nil {
		return clone, nil
	}

	body, err := getBody()
	if err != nil {
		return nil, err
	}
	clone.Body = body
	clone.GetBody = getBody
	return clone, nil
}

func closeResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

const maxInt64 = int64(^uint64(0) >> 1)
