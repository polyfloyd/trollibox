package player

import (
	"testing"
)

func TestAllTrackInfo(t *testing.T) {
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

	tracks, err := AllTrackInfo(libs)
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 0 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	}

	tracks, err = AllTrackInfo(libs, "x", "y", "z")
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

	tracks, err = AllTrackInfo(libs, "foo")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 1 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else if tracks[0].Title != "lib1" {
		t.Fatalf("Unexpected library: %s", tracks[0].Title)
	}

	tracks, err = AllTrackInfo(libs, "bar", "x")
	if err != nil {
		t.Fatal(err)
	} else if len(tracks) != 2 {
		t.Fatalf("Unexpected length: %v", len(tracks))
	} else if tracks[0].Title != "lib2" {
		t.Fatalf("Unexpected library: %s", tracks[0].Title)
	} else if tracks[1].URI != "" {
		t.Fatalf("The track at index %v should have been zero", 1)
	}

	tracks, err = AllTrackInfo(libs, "foo", "bar", "baz")
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
}
