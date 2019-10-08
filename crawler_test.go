package main

import (
	"strconv"
	"testing"
)

func TestExtractUrlsWithNoUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevrevtertretretretretretrezecnhfze</body></html>"

	urls := extractUrls("", []byte(content))
	if len(urls) != 0 {
		t.Errorf("Urls found but no one exist")
	}
}

func TestExtractUrlsWithNonOnionUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevhttp://IamTheBest.de/tretretrezecnhfze<div>https://creekorful.me</div></body></html>"

	urls := extractUrls("", []byte(content))
	if len(urls) != 0 {
		t.Errorf("Urls found but shouldn't because non .onion URLs")
	}
}

func TestExtractUrlsWithOnionUrls(t *testing.T) {
	var content = "<html><title>My WebSite</title><body>'tevhttp://IamTheBest.onion/tretretrezecnhfze<div>https://creekorful.me</div>https://efezfezf.onion</body></html>"

	urls := extractUrls("", []byte(content))
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

func TestExtractUrlsWithRelativeUrls(t *testing.T) {
	var content = "<a href=\"https://google.com\"></a><a href=\"http://google.onion\"></a><a href=\"the-best/page.img\"></a><a href=\"/the-best/page.iso\"></a>"

	urls := extractUrls("http://mysite.onion", []byte(content))
	if len(urls) != 3 {
		t.Errorf("More or less than 3 urls found (" + strconv.Itoa(len(urls)) + ")")
		return
	}

	if url := urls[0]; url != "http://google.onion" {
		t.Errorf("Urls does match http://google.onion (value: " + url + ")")
		return
	}
	if url := urls[1]; url != "http://mysite.onion/the-best/page.img" {
		t.Errorf("Urls does match http://mysite.onion/the-best/page.img (value: " + url + ")")
		return
	}
	if url := urls[2]; url != "http://mysite.onion/the-best/page.iso" {
		t.Errorf("Urls does match http://mysite.onion/the-best/page.iso (value: " + url + ")")
		return
	}
}
