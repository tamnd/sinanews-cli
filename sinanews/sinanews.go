// Package sinanews is the library behind the sinanews command: the HTTP client,
// request shaping, and the typed data models for Sina News (新浪新闻).
//
// The Client fetches articles from the public feed.sina.com.cn roll API.
// No authentication is required. It sets a real User-Agent and Referer,
// paces requests, and retries transient 429/5xx errors with exponential backoff.
package sinanews

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to Sina News.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// referer is hardcoded because Sina's feed API requires the news homepage as referer.
const referer = "https://news.sina.com.cn/"

// channelIDs maps channel names to lid parameter values.
var channelIDs = map[string]int{
	"all":           2509,
	"domestic":      2510,
	"international": 2511,
}

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://feed.sina.com.cn",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the Sina News feed API.
type Client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// get performs a GET with pacing and retry logic.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		b, retry, err := c.do(ctx, url)
		if err == nil {
			return b, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get: %w", lastErr)
}

func (c *Client) do(ctx context.Context, url string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Referer", referer)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// News fetches articles for the given channel. channel must be one of
// "all", "domestic", or "international". page is 1-based.
func (c *Client) News(ctx context.Context, channel string, page, num int) ([]Article, error) {
	lid, ok := channelIDs[channel]
	if !ok {
		return nil, fmt.Errorf("unknown channel %q (choose all, domestic, international)", channel)
	}
	url := fmt.Sprintf(
		"%s/api/roll/get?pageid=153&lid=%d&k=&num=%d&versionNumber=1.2.4&page=%d&encode=utf-8",
		c.cfg.BaseURL, lid, num, page,
	)
	raw, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp wireResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	articles := make([]Article, 0, len(resp.Result.Data))
	rank := (page-1)*num + 1
	for _, w := range resp.Result.Data {
		articles = append(articles, wireToArticle(w, rank))
		rank++
	}
	return articles, nil
}

// Channels returns the list of available channels.
func Channels() []ChannelInfo {
	order := []string{"all", "domestic", "international"}
	out := make([]ChannelInfo, 0, len(order))
	for _, name := range order {
		out = append(out, ChannelInfo{Name: name, LID: channelIDs[name]})
	}
	return out
}
