package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestThumbnailKeysRoundtrip(t *testing.T) {
	t.Parallel()
	a := Avatar{ThumbnailS3Keys: map[string]string{Size100: "k1"}}
	parsed := ParseThumbnailKeys(a.ThumbnailKeysJSON())
	if parsed[Size100] != "k1" {
		t.Fatalf("got %#v", parsed)
	}
	if ParseThumbnailKeys(nil) == nil {
		t.Fatal("nil raw must return empty map")
	}
	if key, ok := (Avatar{}).ThumbnailKey(Size100); ok || key != "" {
		t.Fatal("empty thumbs")
	}
}

func TestObjectKeys(t *testing.T) {
	t.Parallel()
	id := uuid.NewString()
	orig := OriginalObjectKey("bob", id, ".png")
	if orig != "originals/bob/"+id+".png" {
		t.Fatalf("original key: %s", orig)
	}
	thumb := ThumbnailObjectKey(id, Size300, ".jpg")
	if thumb != "thumbnails/"+id+"/300x300.jpg" {
		t.Fatalf("thumb key: %s", thumb)
	}
}

func TestEventIDs(t *testing.T) {
	t.Parallel()
	if UploadEventID("a") != "upload:a" {
		t.Fatal("upload id")
	}
	if DeleteEventID("a") != "delete:a" {
		t.Fatal("delete id")
	}
	if ProcessEventID("a") != "process:a" {
		t.Fatal("process id")
	}
}
