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
	"net/url"
	"strings"
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
	"society":       2516,
	"law":           2523,
	"government":    2515,
	"finance":       2017,
	"stock":         2018,
	"sports":        2514,
	"entertainment": 2013,
	"tech":          2039,
	"auto":          2060,
	"house":         2016,
	"military":      2042,
	"gaming":        2076,
}

// channelOrder defines the display order for Channels().
var channelOrder = []string{
	"all", "domestic", "international",
	"society", "law", "government",
	"finance", "stock", "sports",
	"entertainment", "tech", "auto",
	"house", "military", "gaming",
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

// NewClientWithTransport returns a Client that uses the provided RoundTripper.
// Useful in tests to intercept HTTP requests.
func NewClientWithTransport(cfg Config, rt http.RoundTripper) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout, Transport: rt},
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
	out := make([]ChannelInfo, 0, len(channelOrder))
	for _, name := range channelOrder {
		out = append(out, ChannelInfo{Name: name, LID: channelIDs[name]})
	}
	return out
}

// Hot fetches the Sina hot news list.
func (c *Client) Hot(ctx context.Context) ([]HotItem, error) {
	u := "https://top.sina.cn/api/interface/topNews/getTopHot?top_index=0&req_num=50"
	raw, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp wireHotResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode hot response: %w", err)
	}
	items := make([]HotItem, 0, len(resp.Data.AllList))
	for i, w := range resp.Data.AllList {
		items = append(items, HotItem{
			Rank:  i + 1,
			Title: w.Title,
			URL:   w.URL,
			Score: w.HotScore,
			Hot:   w.HotValue,
		})
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

// Article fetches the full detail of a Sina News article by URL or docid.
// If rawInput looks like a URL it is used directly; otherwise it is treated
// as a docid and resolved to news.sina.com.cn.
func (c *Client) Article(ctx context.Context, rawInput string) (*ArticleDetail, error) {
	articleURL := rawInput
	if !strings.HasPrefix(rawInput, "http") {
		articleURL = "https://news.sina.com.cn/c/" + rawInput + ".shtml"
	}
	raw, err := c.get(ctx, articleURL)
	if err != nil {
		return nil, err
	}
	return parseArticleHTML(string(raw), articleURL), nil
}

// Search searches Sina News for the given query and returns results from the
// search.sina.com.cn HTML results page.
func (c *Client) Search(ctx context.Context, query string, page int) ([]SearchResult, error) {
	if page < 1 {
		page = 1
	}
	u := fmt.Sprintf(
		"https://search.sina.com.cn/?q=%s&c=news&range=all&num=20&page=%d&from=index&ie=utf-8",
		url.QueryEscape(query), page,
	)
	raw, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseSearchHTML(string(raw)), nil
}

// parseArticleHTML extracts article fields from raw HTML.
func parseArticleHTML(body, articleURL string) *ArticleDetail {
	d := &ArticleDetail{URL: articleURL}

	// og:title or <h1 class="main-title">
	d.Title = metaContent(body, `property="og:title"`)
	if d.Title == "" {
		d.Title = metaContent(body, `property="og:title"`)
	}
	if d.Title == "" {
		d.Title = innerText(body, `<h1 class="main-title">`)
	}
	if d.Title == "" {
		d.Title = innerText(body, `<h1 `)
	}

	// published time
	d.PublishedAt = metaContent(body, `property="article:published_time"`)
	if d.PublishedAt == "" {
		d.PublishedAt = metaContent(body, `property="article:modified_time"`)
	}

	// source
	d.Source = innerText(body, `class="media-name"`)
	if d.Source == "" {
		d.Source = metaContent(body, `name="mediaid"`)
	}

	// summary
	d.Summary = metaContent(body, `name="description"`)

	// keywords
	kwStr := metaContent(body, `name="keywords"`)
	if kwStr != "" {
		for _, kw := range strings.Split(kwStr, ",") {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				d.Keywords = append(d.Keywords, kw)
			}
		}
	}

	// body text: try <div id="article">, then <div class="article-body">, then <div class="article">
	articleBody := extractBlock(body, `id="article"`)
	if articleBody == "" {
		articleBody = extractBlock(body, `class="article-body"`)
	}
	if articleBody == "" {
		articleBody = extractBlock(body, `class="article"`)
	}
	d.Body = stripTags(articleBody)

	return d
}

// parseSearchHTML extracts search results from Sina search HTML.
func parseSearchHTML(body string) []SearchResult {
	var results []SearchResult
	// Find all <li class="search-result"> blocks
	blocks := splitBlocks(body, `<li class="search-result"`)
	for _, blk := range blocks {
		r := SearchResult{}
		// Title + URL from <h2><a href="...">...</a></h2>
		if idx := strings.Index(blk, "<h2>"); idx >= 0 {
			sub := blk[idx:]
			r.URL = attrValue(sub, "href")
			r.Title = innerText(sub, "<a ")
		}
		// Summary from <p class="alg">
		r.Summary = innerText(blk, `class="alg"`)
		// Source + date from <p class="alg-ent">
		entBlk := extractBlock(blk, `class="alg-ent"`)
		r.Source = innerTextTag(entBlk, "<em>")
		r.Date = innerTextTag(entBlk, "<span>")
		if r.Title != "" {
			results = append(results, r)
		}
	}
	return results
}

// --- tiny HTML helpers (no external dependency) ---

// metaContent returns the content="..." value for the first <meta> tag
// containing needle in body.
func metaContent(body, needle string) string {
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	// Find the enclosing tag start
	start := strings.LastIndex(body[:idx], "<meta")
	if start < 0 {
		return ""
	}
	end := strings.Index(body[start:], ">")
	if end < 0 {
		return ""
	}
	tag := body[start : start+end+1]
	return attrValue(tag, "content")
}

// attrValue extracts the value of a named attribute from a tag fragment.
func attrValue(tag, attr string) string {
	needle := attr + "="
	idx := strings.Index(tag, needle)
	if idx < 0 {
		return ""
	}
	rest := tag[idx+len(needle):]
	if len(rest) == 0 {
		return ""
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' {
		// unquoted
		end := strings.IndexAny(rest, " \t\r\n>")
		if end < 0 {
			return rest
		}
		return rest[:end]
	}
	rest = rest[1:]
	end := strings.IndexByte(rest, quote)
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// innerText finds the first element in body that starts with needle and
// returns the text content up to the first closing tag.
func innerText(body, needle string) string {
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	// skip to the end of the opening tag
	start := strings.Index(body[idx:], ">")
	if start < 0 {
		return ""
	}
	content := body[idx+start+1:]
	end := strings.Index(content, "</")
	if end < 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(stripTags(content[:end]))
}

// innerTextTag finds the first element matching exact tag (e.g., "<em>") and
// returns its text content.
func innerTextTag(body, tag string) string {
	return innerText(body, tag)
}

// extractBlock finds the first element with the given attribute (e.g.
// id="article") and returns its inner HTML up to the matching closing tag.
func extractBlock(body, attr string) string {
	idx := strings.Index(body, attr)
	if idx < 0 {
		return ""
	}
	// find opening tag start
	start := strings.LastIndex(body[:idx], "<")
	if start < 0 {
		return ""
	}
	tagEnd := strings.Index(body[start:], ">")
	if tagEnd < 0 {
		return ""
	}
	// determine element name
	tagLine := body[start : start+tagEnd+1]
	elemName := elementName(tagLine)
	inner := body[start+tagEnd+1:]
	closeTag := "</" + elemName + ">"
	// find matching close (simple: first occurrence, good enough for news bodies)
	end := strings.Index(inner, closeTag)
	if end < 0 {
		return inner
	}
	return inner[:end]
}

// elementName extracts the element name from a tag like "<div id=...".
func elementName(tag string) string {
	s := strings.TrimLeft(tag, "< ")
	end := strings.IndexAny(s, " \t\r\n>/")
	if end < 0 {
		return strings.TrimRight(s, ">")
	}
	return s[:end]
}

// splitBlocks splits body on occurrences of needle, returning everything after
// each occurrence up to the next occurrence (or end of string).
func splitBlocks(body, needle string) []string {
	parts := strings.Split(body, needle)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

// stripTags removes HTML tags and decodes a few common entities.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			b.WriteRune(' ')
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&nbsp;", " ")
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", "\"")
	out = strings.ReplaceAll(out, "&#39;", "'")
	// collapse whitespace
	lines := strings.Split(out, "\n")
	var cleaned []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			cleaned = append(cleaned, l)
		}
	}
	return strings.Join(cleaned, "\n")
}
