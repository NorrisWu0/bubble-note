package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/notes"
)

func TestS3ClientChecksCustomEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/notes-bucket" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewS3(config.StorageConfig{
		Endpoint:        server.URL,
		Region:          "nyc3",
		Bucket:          "notes-bucket",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		PathStyle:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewS3RejectsIncompleteStorage(t *testing.T) {
	if _, err := NewS3(config.StorageConfig{}); err == nil {
		t.Fatal("expected incomplete storage error")
	}
}

func TestS3ClientPublishesNoteManifestAfterRevisions(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.Method {
		case http.MethodPut:
			w.Header().Set("ETag", `"manifest-etag"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></ListBucketResult>`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client, err := NewS3(config.StorageConfig{Endpoint: server.URL, Region: "us-east-1", Bucket: "notes", AccessKeyID: "access", SecretAccessKey: "secret", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	note := notes.Note{ID: "note-1", Title: "hello", CurrentRevID: "rev-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	revisions := []notes.Revision{{ID: "rev-1", NoteID: "note-1", Title: "hello", Content: "body", CreatedAt: time.Now()}}
	etag, err := client.SyncNote(context.Background(), note, revisions, "")
	if err != nil {
		t.Fatal(err)
	}
	if etag != "manifest-etag" {
		t.Fatalf("etag = %q, want manifest-etag", etag)
	}
	if len(requests) != 3 || requests[0] != "PUT /notes/notes/note-1/revisions/rev-1.json" || requests[1] != "PUT /notes/notes/note-1/manifest.json" || requests[2] != "GET /notes" {
		t.Fatalf("requests = %#v, want revision PUT, manifest PUT, then prune listing", requests)
	}
}
