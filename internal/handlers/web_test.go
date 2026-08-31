package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadPage(t *testing.T) {
	h, _ := newTestHandler()
	webh, err := NewWebHandler(h, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	webh.UploadPage(rec, httptest.NewRequest(http.MethodGet, "/web/upload", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("GophProfile")) {
		t.Fatal("html")
	}

	rec = httptest.NewRecorder()
	webh.GalleryPage(rec, httptest.NewRequest(http.MethodGet, "/web/gallery/demo", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("gallery %d", rec.Code)
	}
}

func TestUploadForm(t *testing.T) {
	h, _ := newTestHandler()
	webh, err := NewWebHandler(h, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("user_id", "carol")
	fw, err := mw.CreateFormFile("file", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(pngFile(t)); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/web/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	webh.UploadForm(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/web/gallery/carol" {
		t.Fatalf("location %s", loc)
	}
}
