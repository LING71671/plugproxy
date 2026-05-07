package discover

import "testing"

func TestExtractURLsNormalizesAndDeduplicates(t *testing.T) {
	text := `
		https://raw.githubusercontent.com/a/proxy/main/http.txt
		"https://raw.githubusercontent.com/a/proxy/main/http.txt",
		https://github.com/zloi-user/hideip.me/raw/refs/heads/master/http.txt,,ColonURL
	`

	got := ExtractURLs(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 URLs, got %d: %#v", len(got), got)
	}
	if got[0] == "" || got[1] == "" {
		t.Fatalf("expected normalized URLs, got %#v", got)
	}
}

func TestLooksLikeProxyList(t *testing.T) {
	content := "1.1.1.1:8080\nhttp://2.2.2.2:3128\n"
	if !LooksLikeProxyList(content) {
		t.Fatal("expected proxy list")
	}
}

func TestAnalyzeHTMLTextProxyList(t *testing.T) {
	content := `<html><body>1.1.1.1:8080<br>2.2.2.2:3128<br></body></html>`

	candidates := NewAnalyzer().AnalyzeURLContent("https://www.89ip.cn/tqdl.html", content, "test")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].SourceKind != KindHTMLText {
		t.Fatalf("expected html text kind, got %s", candidates[0].SourceKind)
	}
	if candidates[0].AdapterRequired {
		t.Fatal("expected html text source to be supported")
	}
	if candidates[0].Recipe == nil || candidates[0].Recipe.Parser != "html_text_auto" {
		t.Fatalf("unexpected recipe %#v", candidates[0].Recipe)
	}
}

func TestAnalyzeHTMLTextProxyTableCells(t *testing.T) {
	content := `<html><body><table><tr><td>1.1.1.1</td><td>8080</td><td>HTTP</td></tr></table></body></html>`

	candidates := NewAnalyzer().AnalyzeURLContent("https://www.kuaidaili.com/free/inha/1/", content, "test")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].SourceKind != KindHTMLText {
		t.Fatalf("expected html text kind, got %s", candidates[0].SourceKind)
	}
	if candidates[0].AdapterRequired {
		t.Fatal("expected split table source to be supported")
	}
	if candidates[0].ProtocolHint != "http" {
		t.Fatalf("expected http protocol hint, got %q", candidates[0].ProtocolHint)
	}
}

func TestLooksLikeSourceList(t *testing.T) {
	content := "https://raw.githubusercontent.com/a/b/main/http.txt\nhttps://api.example.com/proxies.txt\n"
	if !LooksLikeSourceList(content) {
		t.Fatal("expected source list")
	}
}

func TestInferProtocolHintDoesNotUseURLScheme(t *testing.T) {
	got := InferProtocolHint("https://www.89ip.cn/tqdl.html?api=1", "1.1.1.1:8080<br>")
	if got != "http" {
		t.Fatalf("expected http hint, got %q", got)
	}
}

func TestInferProtocolHintUsesExplicitProxyScheme(t *testing.T) {
	got := InferProtocolHint("https://example.com/proxies", "socks5://1.1.1.1:1080<br>")
	if got != "socks5" {
		t.Fatalf("expected socks5 hint, got %q", got)
	}
}

func TestAnalyzeCrawlerCodeReference(t *testing.T) {
	content := `
	def freeProxy02():
	    url = "http://www.66ip.cn/"
	    yield "1.1.1.1:8080"
	`

	candidates := NewAnalyzer().AnalyzeText(content, "github:test/repo:fetcher/proxyFetcher.py")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].SourceKind != KindCrawlerCodeReference {
		t.Fatalf("expected crawler code reference, got %s", candidates[0].SourceKind)
	}
	if !candidates[0].AdapterRequired {
		t.Fatal("expected adapter required")
	}
}

func TestAnalyzeTextSkipsNonSourceLinks(t *testing.T) {
	content := `
		[badge](https://img.shields.io/badge/test.svg)
		localhost http://127.0.0.1:5010/get/
		代理源 https://www.66ip.cn/
	`

	candidates := NewAnalyzer().AnalyzeText(content, "readme")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %#v", len(candidates), candidates)
	}
	if candidates[0].URL != "https://www.66ip.cn/" {
		t.Fatalf("expected 66ip candidate, got %s", candidates[0].URL)
	}
}

func TestDeduplicateKeepsHigherConfidence(t *testing.T) {
	low := NewCandidate("https://example.com/http.txt", "", KindRawText, "a", "")
	high := low
	high.Confidence = 0.9
	high.DiscoveredFrom = "b"

	got := Deduplicate([]CandidateSource{low, high})
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	if got[0].DiscoveredFrom != "b" {
		t.Fatalf("expected higher confidence candidate, got %#v", got[0])
	}
}
