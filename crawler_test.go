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

func TestExtractUrlsWithNonOnionUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevhttp://IamTheBest.de/tretretrezecnhfze<div>https://creekorful.me</div></body></html>"

	urls := extractUrls([]byte(content))
	if len(urls) != 0 {
		t.Errorf("Urls found but shouldn't because non .onion URLs")
	}
}

func TestExtractUrlsWithOnionUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevhttp://IamTheBest.onion/tretretrezecnhfze<div>https://creekorful.me</div>https://efezfezf.onion</body></html>"

	urls := extractUrls([]byte(content))
	if len(urls) != 2 {
		t.Errorf("More or less than 2 urls found")
		return
	}

	if url := urls[0]; url != "http://IamTheBest.onion/tretretrezecnhfze" {
		t.Errorf("Urls does match http://IamTheBest.onion/tretretrezecnhfze (value: " + url + ")")
		return
	}

	if url := urls[1]; url != "https://efezfezf.onion" {
		t.Errorf("Urls does match https://efezfezf.onion (value: " + url + ")")
		return
	}
}

//TODO: add support for relative URL
