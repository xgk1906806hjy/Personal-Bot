package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

const (
	defaultSearchEndpoint = "https://html.duckduckgo.com/html/"
	defaultHTTPTimeout    = 20 * time.Second
	defaultMaxSearchHits  = 8
	browserUA             = "Mozilla/5.0 (compatible; PersonalBot/1.0; +https://github.com/xgk1906806hjy/Personal-Bot)"
)

type webHTTP struct {
	client         *http.Client
	maxBytes       int
	searchEndpoint string
	maxHits        int
}

func newWebHTTP(timeout time.Duration, maxBytes int, searchEndpoint string) *webHTTP {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	if maxBytes <= 0 {
		maxBytes = 100_000
	}
	if searchEndpoint == "" {
		searchEndpoint = defaultSearchEndpoint
	}
	return &webHTTP{
		client:         &http.Client{Timeout: timeout},
		maxBytes:       maxBytes,
		searchEndpoint: searchEndpoint,
		maxHits:        defaultMaxSearchHits,
	}
}

type searchHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type webSearchTool struct {
	http *webHTTP
}

func newWebSearchTool(h *webHTTP) *webSearchTool {
	return &webSearchTool{http: h}
}

func (t *webSearchTool) Name() string { return "web_search" }
func (t *webSearchTool) Description() string {
	return "Search the public web (DuckDuckGo HTML, no API key). Returns a short list of titles, URLs, and snippets. Use for facts, news, or docs you do not already know; then fetch_url for a specific page."
}
func (t *webSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Max results to return (default 8, max 10)",
			},
		},
		"required":             []string{"query"},
		"additionalProperties": false,
	}
}

func (t *webSearchTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	limit := t.http.maxHits
	if args.MaxResults > 0 {
		limit = args.MaxResults
	}
	if limit > 10 {
		limit = 10
	}

	form := url.Values{}
	form.Set("q", query)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.http.searchEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", browserUA)

	resp, err := t.http.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.http.maxBytes)+1))
	if err != nil {
		return "", fmt.Errorf("read search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("search HTTP %d: %s", resp.StatusCode, truncateRunes(string(body), 200))
	}
	hits := parseDDGResults(string(body), limit)
	out, err := json.Marshal(map[string]any{
		"query":   query,
		"count":   len(hits),
		"results": hits,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type fetchURLTool struct {
	http *webHTTP
}

func newFetchURLTool(h *webHTTP) *fetchURLTool {
	return &fetchURLTool{http: h}
}

func (t *fetchURLTool) Name() string { return "fetch_url" }
func (t *fetchURLTool) Description() string {
	return "Fetch an http(s) URL and return readable text (HTML stripped to plain text; JSON/plain kept). Use after web_search when you need page content."
}
func (t *fetchURLTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "Absolute http or https URL",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Optional max response bytes to keep (defaults to tool limit)",
			},
		},
		"required":             []string{"url"},
		"additionalProperties": false,
	}
}

func (t *fetchURLTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		URL      string `json:"url"`
		MaxBytes int    `json:"max_bytes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	raw := strings.TrimSpace(args.URL)
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		return "", fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("only http and https URLs are allowed")
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}

	limit := t.http.maxBytes
	if args.MaxBytes > 0 && args.MaxBytes < limit {
		limit = args.MaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json,text/plain,*/*;q=0.8")

	resp, err := t.http.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	truncated := len(body) > limit
	if truncated {
		body = body[:limit]
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateRunes(string(body), 200))
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	ctMain := ct
	if i := strings.Index(ct, ";"); i >= 0 {
		ctMain = strings.TrimSpace(ct[:i])
	}
	text := string(body)
	switch {
	case strings.Contains(ctMain, "html"):
		text = htmlToText(text)
	case ctMain == "" ||
		strings.HasPrefix(ctMain, "text/") ||
		strings.Contains(ctMain, "json") ||
		strings.Contains(ctMain, "xml") ||
		strings.Contains(ctMain, "javascript"):
		// keep as-is
	default:
		return "", fmt.Errorf("unsupported content-type: %s", ctMain)
	}
	text = strings.TrimSpace(collapseWS(text))
	if truncated {
		text += "\n…(truncated)"
	}
	out, err := json.Marshal(map[string]any{
		"url":          resp.Request.URL.String(),
		"status":       resp.StatusCode,
		"content_type": ctMain,
		"text":         text,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseDDGResults(page string, limit int) []searchHit {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	hits := make([]searchHit, 0, limit)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(hits) >= limit {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "result__a") {
			href := attr(n, "href")
			title := strings.TrimSpace(nodeText(n))
			if title == "" || href == "" {
				return
			}
			hit := searchHit{
				Title: title,
				URL:   unwrapDDGLink(href),
			}
			if sn := findNearbySnippet(n); sn != "" {
				hit.Snippet = sn
			}
			hits = append(hits, hit)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if len(hits) >= limit {
				return
			}
		}
	}
	walk(doc)
	return hits
}

func findNearbySnippet(anchor *html.Node) string {
	// Climb to result container then find result__snippet.
	cur := anchor
	for i := 0; i < 6 && cur != nil; i++ {
		cur = cur.Parent
	}
	if cur == nil {
		return ""
	}
	var sn string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if sn != "" {
			return
		}
		if n.Type == html.ElementNode && (hasClass(n, "result__snippet") || hasClass(n, "result__body")) {
			if hasClass(n, "result__snippet") {
				sn = strings.TrimSpace(nodeText(n))
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && hasClass(c, "result__snippet") {
				sn = strings.TrimSpace(nodeText(c))
				return
			}
			walk(c)
		}
	}
	walk(cur)
	return sn
}

func unwrapDDGLink(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if q := u.Query().Get("uddg"); q != "" {
		if decoded, err := url.QueryUnescape(q); err == nil && decoded != "" {
			return decoded
		}
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	return href
}

func htmlToText(page string) string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return page
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "svg", "head":
				return
			case "br", "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6":
				b.WriteByte('\n')
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return b.String()
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return collapseWS(b.String())
}

func collapseWS(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key != "class" {
			continue
		}
		for _, c := range strings.Fields(a.Val) {
			if c == class {
				return true
			}
		}
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func truncateRunes(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
