package sinanews_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
