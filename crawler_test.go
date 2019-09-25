package main

import (
	"testing"
)

func TestExtractUrlsWithNoUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevrevtertretretretretretrezecnhfze</body></html>"

	urls := extractUrls([]byte(content))
	if len(urls) != 0 {
		t.Errorf("Urls found but no one exist")
	}
}

func TestExtractUrlsWithUrl(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevrevtertretretretretretrezecnhfze<div>https://creekorful.me</div></body></html>"

	urls := extractUrls([]byte(content))
	if url := urls[0]; url != "https://creekorful.me" {
		t.Errorf("Url found: %v should be: https://creekorful.me", url)
	}
}

func TestExtractUrlsWithUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevhttp://IamTheBest.de/tretretrezecnhfze<div>https://creekorful.me</div></body></html>"

	urls := extractUrls([]byte(content))
	if url := urls[0]; url != "http://IamTheBest.de/tretretrezecnhfze" {
		t.Errorf("Url found: %v should be: https://creekorful.me", url)
	}
	if url := urls[1]; url != "https://creekorful.me" {
		t.Errorf("Url found: %v should be: https://creekorful.me", url)
	}
}

//TODO: prevent crawler from crawling non .onion URLs by improving the regex

//TODO: add support for relative URL