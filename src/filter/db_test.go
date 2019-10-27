package filter

import (
	"os"
	"path"
	"testing"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/util"
)

func init() {
	RegisterFactory(func() Filter {
		return &dummyFilter{}
	})
}

type dummyFilter struct {
	Foo string
	Bar string
}

func (*dummyFilter) Filter(library.Track) (SearchResult, bool) {
	return SearchResult{}, true
}

func TestDBGetSetRemove(t *testing.T) {
	db, err := NewDB(path.Join(os.TempDir(), "filter-db-test-getsetremove"))
	if err != nil {
		t.Fatal(err)
	}

	filter1 := &dummyFilter{
		Foo: "foo",
		Bar: "bar",
	}
	if err := db.Set("001", filter1); err != nil {
		t.Fatal(err)
	}

	filter2 := &dummyFilter{
		Foo: "baz",
		Bar: "bux",
	}
	if err := db.Set("002", filter2); err != nil {
		t.Fatal(err)
	}

	names, err := db.Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("Bad number of names: %v", len(names))
	}
	if names[0] != "001" {
		t.Fatalf("Bad name at index %v: %q", 0, names[0])
	}
	if names[1] != "002" {
		t.Fatalf("Bad name at index %v: %q", 1, names[1])
	}

	loadedFilter1, err := db.Get("001")
	if err != nil {
		t.Fatal(err)
	}
	if *loadedFilter1.(*dummyFilter) != *filter1 {
		t.Fatalf("Filter 1 was not loaded correctly: %#v", filter1)
	}

	loadedFilter2, err := db.Get("002")
	if err != nil {
		t.Fatal(err)
	}
	if *loadedFilter2.(*dummyFilter) != *filter2 {
		t.Fatalf("Filter 2 was not loaded correctly: %#v", filter2)
	}

	if err := db.Remove("001"); err != nil {
		t.Fatal(err)
	}
	if names, err := db.Names(); err != nil {
		t.Fatal(err)
	} else if len(names) != 1 {
		t.Fatalf("Unexpected number of names: %v", len(names))
	}
	if err := db.Remove("002"); err != nil {
		t.Fatal(err)
	}
	if names, err := db.Names(); err != nil {
		t.Fatal(err)
	} else if len(names) != 0 {
		t.Fatalf("Unexpected number of names: %v", len(names))
	}
}

func TestDBGetNonExistent(t *testing.T) {
	db, err := NewDB(path.Join(os.TempDir(), "filter-db-test-getnonexistent"))
	if err != nil {
		t.Fatal(err)
	}

	filter, err := db.Get("non-existing")
	if err != nil {
		t.Fatalf("Getting a non-existing filter should return no error, got: %v", err)
	}
	if filter != nil {
		t.Fatalf("A %#v was loaded from a non-existing filter", filter)
	}
}

func TestDBRemoveNonExistent(t *testing.T) {
	db, err := NewDB(path.Join(os.TempDir(), "filter-db-test-removenonexistent"))
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Remove("non-existing"); err != nil {
		t.Fatalf("Removing non-existing filters should return no error, got: %v", err)
	}
}

func TestDBSetInvalid(t *testing.T) {
	db, err := NewDB(path.Join(os.TempDir(), "filter-db-test-setinvalid"))
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Set("", &dummyFilter{}); err == nil {
		t.Fatalf("An empty filter name was allowed to be set")
	}
	if err := db.Set("foo/bar/baz", &dummyFilter{}); err == nil {
		t.Fatalf("Slashes were allowed to be set")
	}
}

func TestDBEvents(t *testing.T) {
	db, err := NewDB(path.Join(os.TempDir(), "filter-db-test-events"))
	if err != nil {
		t.Fatal(err)
	}

	util.TestEventEmission(t, db, UpdateEvent{}, func() {
		filter := &dummyFilter{Foo: "foo"}
		if err := db.Set("filter", filter); err != nil {
			t.Fatal(err)
		}
	})
	util.TestEventEmission(t, db, UpdateEvent{}, func() {
		if err := db.Remove("filter"); err != nil {
			t.Fatal(err)
		}
	})
}
