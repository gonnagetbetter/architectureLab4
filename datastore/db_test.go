package datastore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type Data struct {
	key   string
	value string
}

func TestPutGet(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDb(dir, 300)
	if err != nil {
		t.Fatal(err)
	}
	defer db.out.Close()

	data := []Data{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	outPath := filepath.Join(dir, outFileName+"0")
	outFile, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Put/Get Check", func(t *testing.T) {
		for i := 0; i < len(data); i++ {
			key := data[i].key
			value := data[i].value

			err := db.Put(key, value)
			if err != nil {
				t.Errorf("Cannot put %s: %s", key, err)
			}

			result, err := db.Get(key)
			if err != nil {
				t.Errorf("Cannot get %s: %s", key, err)
			}

			if result != value {
				t.Errorf("Bad value returned expected %s, got %s", value, result)
			}
		}
	})

	outInfo, err := outFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	size1 := outInfo.Size()

	t.Run("Size Check", func(t *testing.T) {
		for i := 0; i < len(data); i++ {
			key := data[i].key
			value := data[i].value

			err := db.Put(key, value)
			if err != nil {
				t.Errorf("Cannot put %s: %s", key, err)
			}
		}

		outInfo, err := outFile.Stat()
		if err != nil {
			t.Fatal(err)
		}

		if size1*2 != outInfo.Size() {
			t.Errorf("Unexpected size (%d vs %d)", size1, outInfo.Size())
		}
	})

	t.Run("New DB Process", func(t *testing.T) {
		if err := db.out.Close(); err != nil {
			t.Fatal(err)
		}

		db, err = NewDb(dir, 100)
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < len(data); i++ {
			key := data[i].key
			value := data[i].value

			result, err := db.Get(key)
			if err != nil {
				t.Errorf("'Cannot get %s: %s", key, err)
			}

			if result != value {
				t.Errorf("Bad value returned expected %s, got %s", value, result)
			}
		}
	})
}

func TestDelete(t *testing.T) {
  dir, err := ioutil.TempDir("", "test-db")
  if err != nil {
    t.Fatal(err)
  }
  defer os.RemoveAll(dir)

  db, err := NewDb(dir, 300)
  if err != nil {
    t.Fatal(err)
  }
  defer db.out.Close()

  data := []Data{
    {"key1", "value1"},
    {"key2", "value2"},
    {"key3", "value3"},
  }

  t.Run("Delete Check", func(t *testing.T) {
    for i := 0; i < len(data); i++ {
      key := data[i].key
      value := data[i].value

      err := db.Put(key, value)
      if err != nil {
        t.Errorf("Cannot put %s: %s", key, err)
      }

      err = db.Delete(key)
      if err != nil {
        t.Errorf("Cannot delete %s: %s", key, err)
      }

      _, err = db.Get(key)
      if err != ErrNotFound {
        t.Errorf("Expected ErrNotFound for key %s, got %s", key, err)
      }
    }
  })

  t.Run("New DB Process", func(t *testing.T) {
    if err := db.out.Close(); err != nil {
      t.Fatal(err)
    }

    db, err = NewDb(dir, 100)
    if err != nil {
      t.Fatal(err)
    }

    for i := 0; i < len(data); i++ {
      key := data[i].key

      _, err := db.Get(key)
      if err != ErrNotFound {
        t.Errorf("Expected ErrNotFound for key %s, got %s", key, err)
      }
    }
  })
}

func TestSegmentation(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDb(dir, 45)
	if err != nil {
		t.Fatal(err)
	}
	defer db.out.Close()

	t.Run("Segmentation Check", func(t *testing.T) {
		db.Put("key1", "value1")
		db.Put("key2", "value2")
		db.Put("key3", "value3")

		if len(db.segments) != 2 {
			t.Errorf("Expected 2 files instead %d", len(db.segments))
		}
	})

	t.Run("Merging Check", func(t *testing.T) {
		db.Put("key4", "value4")
		db.Put("key5", "value5")

		if len(db.segments) != 3 {
			t.Errorf("Expected 3 files instead %d", len(db.segments))
		}

		time.Sleep(2 * time.Second)

		if len(db.segments) != 2 {
			t.Errorf("Expected 2 files instead %d", len(db.segments))
		}
	})

	t.Run("Size Check", func(t *testing.T) {
		file1, err := os.Open(db.segments[0].outPath)
		defer file1.Close()
		if err != nil {
			t.Error(err)
		}
		info1, _ := file1.Stat()

		file2, err := os.Open(db.segments[1].outPath)
		defer file2.Close()
		if err != nil {
			t.Error(err)
		}
		info2, _ := file2.Stat()

		if info1.Size() != 88 {
			t.Errorf("Expected size 88, got instead %d", info1.Size())
		}

		if info2.Size() != 22 {
			t.Errorf("Expected size 22, got instead %d", info2.Size())
		}
	})

	t.Run("Newer Values Check", func(t *testing.T) {
		db.Put("key2", "value0")
		value, _ := db.Get("key2")
		if value != "value0" {
			t.Errorf("Bad value returned expected value0, got %s", value)
		}
	})

	t.Run("Full Check", func(t *testing.T) {
		db.Put("key6", "value6")

		if len(db.segments) != 3 {
			t.Errorf("Expected 3 files instead %d", len(db.segments))
		}

		time.Sleep(2 * time.Second)

		if len(db.segments) != 2 {
			t.Errorf("Expected 2 file instead %d", len(db.segments))
		}

		expected := []Data{
			{"key1", "value1"},
			{"key2", "value0"},
			{"key3", "value3"},
			{"key4", "value4"},
			{"key5", "value5"},
			{"key6", "value6"},
		}

		for i := 0; i < len(expected); i++ {
			key := expected[i].key
			value := expected[i].value
			result, _ := db.Get(key)
			if result != value {
				t.Errorf("Bad value returned expected %s, got %s", expected[i], result)
			}
		}
	})
}
