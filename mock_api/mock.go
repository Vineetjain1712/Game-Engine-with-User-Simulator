package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"
)

type Options struct {
	N            int           // number of users
	BaseURL      string        // e.g. http://localhost:8080
	MinDelay     time.Duration // min simulated lag
	MaxDelay     time.Duration // max simulated lag
	CorrectRatio float64       // ~probability a user answers correctly
	Seed         int64         // RNG seed; 0 => use time.Now()
}

// Run fires N concurrent simulated user submissions.
func Run(ctx context.Context, opts Options) error {
	if opts.N <= 0 {
		return nil
	}
	if opts.MinDelay <= 0 {
		opts.MinDelay = 10 * time.Millisecond
	}
	if opts.MaxDelay < opts.MinDelay {
		opts.MaxDelay = opts.MinDelay
	}
	if opts.CorrectRatio <= 0 || opts.CorrectRatio > 1 {
		opts.CorrectRatio = 0.2
	}
	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	tr := &http.Transport{
		MaxIdleConns:        4096,
		MaxIdleConnsPerHost: 2048,
		IdleConnTimeout:     30 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: true,
	}
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}

	// Ensure at least one correct answer exists so a winner will always be declared.
	guaranteedWinner := 1 + rng.Intn(opts.N)

	type payload struct {
		UserID  int64 `json:"user_id"`
		Correct bool  `json:"correct"`
	}

	for i := 1; i <= opts.N; i++ {
		i := i
		delay := opts.MinDelay + time.Duration(rng.Int63n(int64(opts.MaxDelay-opts.MinDelay+1)))
		correct := rng.Float64() < opts.CorrectRatio || i == guaranteedWinner

		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}

			body, _ := json.Marshal(payload{UserID: int64(i), Correct: correct})
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, opts.BaseURL+"/submit", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[MOCK] user %d request error: %v", i, err)
				return
			}
			_ = resp.Body.Close()
		}()
	}

	return nil
}
