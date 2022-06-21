package library

import (
	"testing"
)

func TestInterpolateMissingFields(t *testing.T) {
	// Streams should use the URL as title.
	track := Track{URI: "http://example.com/"}
	InterpolateMissingFields(&track)
	if track.Artist != "" || track.Title != "http://example.com/" {
		t.Fatalf("Artist should be the stream URL")
	}
	track = Track{URI: "https://example.com/"}
	InterpolateMissingFields(&track)
	if track.Artist != "" || track.Title != "https://example.com/" {
		t.Fatalf("Artist should be the stream URL")
	}

	// When the artist or title are already set, the track should be left as is.
	track = Track{URI: "file://Wrong Artist - Wrong Title.wav", Artist: "Some Artist", Title: "Some Title"}
	InterpolateMissingFields(&track)
	if track.Artist != "Some Artist" || track.Title != "Some Title" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}
	track = Track{URI: "file://foo/bar/baz/Some Artist - Some Title.wav", Artist: "Some Artist", Title: "Some Title"}
	InterpolateMissingFields(&track)
	if track.Artist != "Some Artist" || track.Title != "Some Title" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}

	// <artist> - <filename> in the title
	track = Track{Title: "Some Artist - Some Title"}
	InterpolateMissingFields(&track)
	if track.Artist != "Some Artist" || track.Title != "Some Title" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}

	// <artist> - <filename> in the filename
	track = Track{URI: "file://foo/bar/baz/Some Artist - Some Title.wav"}
	InterpolateMissingFields(&track)
	if track.Artist != "Some Artist" || track.Title != "Some Title" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}
	track = Track{URI: "file://foo/bar/baz/01. Some Artist - Some Title.wav"}
	InterpolateMissingFields(&track)
	if track.Artist != "Some Artist" || track.Title != "Some Title" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}
	track = Track{URI: "file://foo/bar/baz/01 - Some Artist - Some Title.wav"}
	InterpolateMissingFields(&track)
	if track.Artist != "Some Artist" || track.Title != "Some Title" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}

	// Use the filename as title as fallback.
	track = Track{URI: "file://foo/bar/baz/Unintelligible.wav"}
	InterpolateMissingFields(&track)
	if track.Artist != "" || track.Title != "Unintelligible" {
		t.Fatalf("Unexpected artist and title: %q - %q", track.Artist, track.Title)
	}
}
