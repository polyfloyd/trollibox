package library

import (
	"context"
	"testing"
)

func TestAllTrackInfo(t *testing.T) {
	ctx := context.Background()

	lib1 := DummyLibrary([]Track{
		{URI: "foo", Title: "lib1"},
	})
	lib2 := DummyLibrary([]Track{
		{URI: "foo", Title: "lib2"},
		{URI: "bar", Title: "lib2"},
	})
	lib3 := DummyLibrary([]Track{
		{URI: "foo", Title: "lib3"},
		{URI: "bar", Title: "lib3"},
		{URI: "baz", Title: "lib3"},
	})
	libs := []Library{&lib1, &lib2, &lib3}

	// No URIs should no tracks.
	tracks, err := AllTrackInfo(ctx, libs)
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 0 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	}

	// Non-existing tracks should have a zero value.
	tracks, err = AllTrackInfo(ctx, libs, "x", "y", "z")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 3 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else {
		for i, tr := range tracks {
			if tr.URI != "" {
				t.Fatalf("The track at index %v should have been zero", i)
			}
		}
	}

	tracks, err = AllTrackInfo(ctx, libs, "foo")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 1 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else if tracks[0].Title != "lib1" {
		t.Fatalf("Unexpected library: %s", tracks[0].Title)
	}

	tracks, err = AllTrackInfo(ctx, libs, "bar", "x")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 2 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else if tracks[0].Title != "lib2" {
		t.Fatalf("Unexpected library: %s", tracks[0].Title)
	} else if tracks[1].URI != "" {
		t.Fatalf("The track at index %v should have been zero", 1)
	}

	tracks, err = AllTrackInfo(ctx, libs, "foo", "bar", "baz")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 3 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else if tracks[0].Title != "lib1" {
		t.Fatalf("Unexpected library: %s", tracks[0].Title)
	} else if tracks[1].Title != "lib2" {
		t.Fatalf("Unexpected library: %s", tracks[1].Title)
	} else if tracks[2].Title != "lib3" {
		t.Fatalf("Unexpected library: %s", tracks[2].Title)
	}

	// https://github.com/polyfloyd/trollibox/issues/5
	tracks, err = AllTrackInfo(ctx, []Library{&lib3}, "foo", "bar", "baz")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 3 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else {
		for i, tr := range tracks {
			if tr.URI == "" {
				t.Fatalf("The track at index %v is zero", i)
			} else if tr.Title != "lib3" {
				t.Fatalf("Unexpected library: %s", tr.Title)
			}
		}
	}
}
