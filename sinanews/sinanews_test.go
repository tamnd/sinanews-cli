package sinanews_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tamnd/sinanews-cli/sinanews"
)

func TestNews(t *testing.T) {
	payload := map[string]any{
		"result": map[string]any{
			"status": 1,
			"total":  1,
			"data": []map[string]any{
				{
					"docid":      "coooaabc001",
					"title":      "Test Sina Article",
					"url":        "https://news.sina.com.cn/article/1",
					"intro":      "Test intro text",
					"ctime":      int64(1700000000),
					"media_name": "Test Media",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	cfg := sinanews.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := sinanews.NewClient(cfg)
	articles, err := c.News(context.Background(), "all", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
	if articles[0].Title != "Test Sina Article" {
		t.Errorf("title = %q, want %q", articles[0].Title, "Test Sina Article")
	}
}

func TestNewsUnknownChannel(t *testing.T) {
	cfg := sinanews.DefaultConfig()
	cfg.Rate = 0
	c := sinanews.NewClient(cfg)
	_, err := c.News(context.Background(), "bogus", 1, 5)
	if err == nil {
		t.Fatal("expected error for unknown channel, got nil")
	}
}

func TestChannels(t *testing.T) {
	ch := sinanews.Channels()
	if len(ch) == 0 {
		t.Fatal("Channels() returned empty list")
	}
	for _, c := range ch {
		if c.Name == "" {
			t.Error("channel with empty name")
		}
		if c.LID == 0 {
			t.Errorf("channel %q has zero LID", c.Name)
		}
	}
}

func TestChannelsHasNew(t *testing.T) {
	ch := sinanews.Channels()
	// expect at least finance, sports, tech, entertainment, auto
	want := map[string]bool{
		"finance":       false,
		"sports":        false,
		"tech":          false,
		"entertainment": false,
		"auto":          false,
	}
	for _, c := range ch {
		want[c.Name] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("channel %q missing from Channels()", name)
		}
	}
}

func TestHot(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"allList": []map[string]any{
				{
					"title":    "Sina Hot Item 1",
					"url":      "https://news.sina.com.cn/hot/1",
					"hotScore": "9999",
					"hotValue": "热",
				},
				{
					"title":    "Sina Hot Item 2",
					"url":      "https://news.sina.com.cn/hot/2",
					"hotScore": "8888",
					"hotValue": "热",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	// Hot() calls an absolute URL (top.sina.cn); we override BaseURL to intercept
	// by replacing the URL in the client through the httptest server URL.
	// Since Hot() uses a hardcoded URL, we use an http.Client with a custom transport.
	cfg := sinanews.DefaultConfig()
	cfg.Rate = 0
	cfg.BaseURL = srv.URL // not used by Hot() directly

	// Use a round-tripper that redirects all requests to our test server.
	c := sinanews.NewClientWithTransport(cfg, redirectTransport(srv.URL))
	items, err := c.Hot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d hot items, want 2", len(items))
	}
	if items[0].Title != "Sina Hot Item 1" {
		t.Errorf("title = %q, want %q", items[0].Title, "Sina Hot Item 1")
	}
	if items[0].Rank != 1 {
		t.Errorf("rank = %d, want 1", items[0].Rank)
	}
	if items[1].Rank != 2 {
		t.Errorf("rank = %d, want 2", items[1].Rank)
	}
}

func TestArticle(t *testing.T) {
	htmlBody := `<!DOCTYPE html><html><head>
<meta property="og:title" content="Test Article Title">
<meta property="article:published_time" content="2024-06-15T14:23:00+08:00">
<meta name="keywords" content="keyword1,keyword2">
<meta name="description" content="Test summary">
</head><body>
<div id="article"><p>Paragraph one.</p><p>Paragraph two.</p></div>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer srv.Close()

	cfg := sinanews.DefaultConfig()
	cfg.Rate = 0
	c := sinanews.NewClientWithTransport(cfg, redirectTransport(srv.URL))
	detail, err := c.Article(context.Background(), srv.URL+"/test-article")
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("Article returned nil detail")
	}
	if detail.Title != "Test Article Title" {
		t.Errorf("title = %q, want %q", detail.Title, "Test Article Title")
	}
	if detail.PublishedAt != "2024-06-15T14:23:00+08:00" {
		t.Errorf("published_at = %q", detail.PublishedAt)
	}
	if len(detail.Keywords) != 2 {
		t.Errorf("keywords = %v, want 2 items", detail.Keywords)
	}
}

func TestSearch(t *testing.T) {
	htmlBody := `<!DOCTYPE html><html><body>
<ul class="blk_search">
<li class="search-result">
<h2><a href="https://news.sina.com.cn/s/1">Result One</a></h2>
<p class="alg">Summary of result one.</p>
<p class="alg-ent"><em>新华社</em> &nbsp; <span>2024-06-15</span></p>
</li>
<li class="search-result">
<h2><a href="https://news.sina.com.cn/s/2">Result Two</a></h2>
<p class="alg">Summary of result two.</p>
<p class="alg-ent"><em>人民日报</em> &nbsp; <span>2024-06-14</span></p>
</li>
</ul>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer srv.Close()

	cfg := sinanews.DefaultConfig()
	cfg.Rate = 0
	c := sinanews.NewClientWithTransport(cfg, redirectTransport(srv.URL))
	results, err := c.Search(context.Background(), "test query", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d search results, want 2", len(results))
	}
	if results[0].Title != "Result One" {
		t.Errorf("title[0] = %q, want %q", results[0].Title, "Result One")
	}
	if results[0].URL != "https://news.sina.com.cn/s/1" {
		t.Errorf("url[0] = %q", results[0].URL)
	}
}

// redirectTransport returns an http.RoundTripper that sends all requests to base.
func redirectTransport(base string) http.RoundTripper {
	return &fixedTransport{base: base}
}

type fixedTransport struct{ base string }

func (ft *fixedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace only scheme+host; keep path+query.
	newReq := req.Clone(req.Context())
	parsed, _ := url.Parse(ft.base)
	newReq.URL.Scheme = parsed.Scheme
	newReq.URL.Host = parsed.Host
	return http.DefaultTransport.RoundTrip(newReq)
}
