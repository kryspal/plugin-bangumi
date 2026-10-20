package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/kryspal/plugin-sdk/sdk"
)

const (
	maxResponseBytes = 8 << 20
	requestTimeout   = 20 * time.Second
	// minInterval is the pause between two requests: Bangumi publishes no limit, so the plugin goes one request at a time, a few a second.
	minInterval       = 300 * time.Millisecond
	defaultRetryAfter = time.Minute
	// homepage goes in the User-Agent: Bangumi asks open source clients to name their project, and bans the default agents of HTTP libraries.
	homepage = "https://github.com/kryspal/plugin-bangumi"
)

var errNotFound = errors.New("bangumi: not found")

// userAgent follows Bangumi's guideline: developer, application, version and the project page for an open source client.
func userAgent() string { return "kryspal/plugin-bangumi/" + version() + " (" + homepage + ")" }

// api is the Bangumi v0 client: one request at a time, spaced by interval.
type api struct {
	http     *http.Client
	interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

func newAPI() *api {
	return &api{http: &http.Client{Timeout: requestTimeout}, interval: minInterval}
}

// do sends one request and decodes the JSON answer into out. A 429 returns sdk.RateLimited with Retry-After; 5xx and connection errors
// return sdk.Unavailable; 404 returns errNotFound (so does 400 for a GET, which Bangumi answers for an ID that is not a valid number).
func (a *api) do(ctx context.Context, method, rawURL string, body any, out any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if d := time.Until(a.last.Add(a.interval)); d > 0 {
		t := time.NewTimer(d)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		}
	}
	a.last = time.Now()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return sdk.Unavailable("bangumi: bad request")
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return sdk.Unavailable("bangumi: bad request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return sdk.Unavailable("bangumi: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return sdk.Unavailable("bangumi: " + err.Error())
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		d := defaultRetryAfter
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			d = time.Duration(s) * time.Second
		}
		return sdk.RateLimited(d, "bangumi: rate limit reached")
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusBadRequest && method == http.MethodGet:
		return errNotFound
	case resp.StatusCode >= 500:
		return sdk.Unavailable(fmt.Sprintf("bangumi: %s", resp.Status))
	case resp.StatusCode != http.StatusOK:
		return sdk.Unavailable(fmt.Sprintf("bangumi: %s", resp.Status))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return sdk.Unavailable("bangumi: bad response")
		}
	}
	return nil
}

func (a *api) get(ctx context.Context, base, path string, q url.Values, out any) error {
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return a.do(ctx, http.MethodGet, u, nil, out)
}
