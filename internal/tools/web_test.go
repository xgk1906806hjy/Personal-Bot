package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleDDGHTML = `<!DOCTYPE html><html><body>
<div class="results">
  <div class="result">
    <h2 class="result__title">
      <a class="result__a" href="https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fa">Alpha Doc</a>
    </h2>
    <a class="result__snippet">Snippet about alpha.</a>
  </div>
  <div class="result">
    <h2 class="result__title">
      <a class="result__a" href="https://example.com/b">Beta Page</a>
    </h2>
    <a class="result__snippet">Snippet about beta.</a>
  </div>
</div>
</body></html>`

func TestWebSearchParsesDDGHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		_ = r.ParseForm()
		if r.Form.Get("q") != "golang tools" {
			t.Fatalf("q=%q", r.Form.Get("q"))
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(sampleDDGHTML))
	}))
	defer srv.Close()

	reg, err := NewRegistry(Options{
		Workspace:      t.TempDir(),
		MaxOutputBytes: 50_000,
		EnableWeb:      true,
		SearchEndpoint: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := reg.Execute(t.Context(), "web_search", `{"query":"golang tools","max_results":5}`)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Count   int `json:"count"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Count != 2 {
		t.Fatalf("count=%d out=%s", parsed.Count, out)
	}
	if parsed.Results[0].URL != "https://example.com/a" {
		t.Fatalf("url0=%q", parsed.Results[0].URL)
	}
	if parsed.Results[0].Title != "Alpha Doc" {
		t.Fatalf("title0=%q", parsed.Results[0].Title)
	}
	if !strings.Contains(parsed.Results[0].Snippet, "alpha") {
		t.Fatalf("snippet0=%q", parsed.Results[0].Snippet)
	}
}

func TestFetchURLHTMLAndTruncate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><script>evil()</script><style>x{}</style></head><body><h1>Hello</h1><p>World page</p></body></html>`))
	}))
	defer srv.Close()

	reg, err := NewRegistry(Options{
		Workspace:      t.TempDir(),
		MaxOutputBytes: 50_000,
		EnableWeb:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := reg.Execute(t.Context(), "fetch_url", `{"url":`+mustJSON(srv.URL)+`}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "evil") {
		t.Fatalf("script leaked: %s", out)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "World") {
		t.Fatalf("missing text: %s", out)
	}

	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("A", 5000)))
	}))
	defer big.Close()

	reg2, err := NewRegistry(Options{
		Workspace:      t.TempDir(),
		MaxOutputBytes: 100,
		EnableWeb:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out2, err := reg2.Execute(t.Context(), "fetch_url", `{"url":`+mustJSON(big.URL)+`}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2, "…(truncated)") {
		t.Fatalf("expected truncation: %s", out2)
	}
}

func TestFetchURLRejectsNonHTTP(t *testing.T) {
	reg, err := NewRegistry(Options{
		Workspace:      t.TempDir(),
		MaxOutputBytes: 10_000,
		EnableWeb:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Execute(t.Context(), "fetch_url", `{"url":"file:///etc/passwd"}`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "http") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseDDGResultsDirect(t *testing.T) {
	hits := parseDDGResults(sampleDDGHTML, 1)
	if len(hits) != 1 {
		t.Fatalf("len=%d", len(hits))
	}
	if hits[0].URL != "https://example.com/a" {
		t.Fatalf("url=%q", hits[0].URL)
	}
}
