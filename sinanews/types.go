package sinanews

import "time"

// Article is the domain record returned by News.
type Article struct {
	Rank   int    `json:"rank"`
	Title  string `json:"title"`
	Source string `json:"source"`
	Intro  string `json:"intro"`
	Date   string `json:"date"`
	URL    string `json:"url"`
}

// ChannelInfo describes an available news channel.
type ChannelInfo struct {
	Name string `json:"name"`
	LID  int    `json:"lid"`
}

// wire types (unexported)

type wireResponse struct {
	Result wireResult `json:"result"`
}

type wireResult struct {
	Status int           `json:"status"`
	Total  int           `json:"total"`
	Data   []wireArticle `json:"data"`
}

type wireArticle struct {
	DocID     string `json:"docid"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Intro     string `json:"intro"`
	CTime     int64  `json:"ctime"`
	MediaName string `json:"media_name"`
	Keywords  string `json:"keywords"`
}

func wireToArticle(w wireArticle, rank int) Article {
	date := ""
	if w.CTime > 0 {
		date = time.Unix(w.CTime, 0).UTC().Format("2006-01-02")
	}
	intro := w.Intro
	rs := []rune(intro)
	if len(rs) > 100 {
		intro = string(rs[:100]) + "..."
	}
	return Article{
		Rank:   rank,
		Title:  w.Title,
		Source: w.MediaName,
		Intro:  intro,
		Date:   date,
		URL:    w.URL,
	}
}
