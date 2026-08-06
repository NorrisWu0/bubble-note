package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/norriswu0/bubble-note/internal/config"
	"github.com/norriswu0/bubble-note/internal/notes"
)

type S3Client struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3(cfg config.StorageConfig) (*S3Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	awsConfig := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
	}
	options := func(options *s3.Options) {
		options.UsePathStyle = cfg.PathStyle
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	}
	return &S3Client{
		client: s3.NewFromConfig(awsConfig, options),
		bucket: cfg.Bucket,
		prefix: strings.Trim(cfg.Prefix, "/"),
	}, nil
}

func (s *S3Client) CheckConnection(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("check S3 bucket %q: %w", s.bucket, err)
	}
	return nil
}

type noteManifest struct {
	Version      int      `json:"version"`
	NoteID       string   `json:"note_id"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	CurrentRevID string   `json:"current_revision_id"`
	RevisionIDs  []string `json:"revision_ids"`
	DeletedAt    *string  `json:"deleted_at,omitempty"`
}

func (s *S3Client) SyncNote(ctx context.Context, note notes.Note, revisions []notes.Revision, expectedETag string) (string, error) {
	retained := make(map[string]struct{}, len(revisions))
	for _, revision := range revisions {
		retained[revision.ID] = struct{}{}
		data, err := json.Marshal(revision)
		if err != nil {
			return "", fmt.Errorf("encode revision %q: %w", revision.ID, err)
		}
		if err := s.put(ctx, s.revisionKey(note.ID, revision.ID), data); err != nil {
			return "", fmt.Errorf("upload revision %q: %w", revision.ID, err)
		}
	}
	ids := make([]string, 0, len(revisions))
	for _, revision := range revisions {
		ids = append(ids, revision.ID)
	}
	manifest, err := json.Marshal(noteManifest{
		Version:      1,
		NoteID:       note.ID,
		Title:        note.Title,
		Tags:         note.Tags,
		CreatedAt:    note.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:    note.UpdatedAt.UTC().Format(time.RFC3339Nano),
		CurrentRevID: note.CurrentRevID,
		RevisionIDs:  ids,
	})
	if err != nil {
		return "", fmt.Errorf("encode note manifest: %w", err)
	}
	etag, err := s.putWithETag(ctx, s.manifestKey(note.ID), manifest, expectedETag)
	if err != nil {
		return "", fmt.Errorf("publish note manifest: %w", err)
	}
	if err := s.pruneRemoteRevisions(ctx, note.ID, retained); err != nil {
		return "", fmt.Errorf("prune remote revisions: %w", err)
	}
	return etag, nil
}

func (s *S3Client) PullNote(ctx context.Context, noteID string) (notes.SyncSnapshot, string, error) {
	data, etag, err := s.get(ctx, s.manifestKey(noteID))
	if err != nil {
		return notes.SyncSnapshot{}, "", err
	}
	var manifest noteManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return notes.SyncSnapshot{}, "", fmt.Errorf("decode note manifest: %w", err)
	}
	created, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
	if err != nil {
		return notes.SyncSnapshot{}, "", err
	}
	updated, err := time.Parse(time.RFC3339Nano, manifest.UpdatedAt)
	if err != nil {
		return notes.SyncSnapshot{}, "", err
	}
	note := notes.Note{ID: manifest.NoteID, Title: manifest.Title, Tags: manifest.Tags, CreatedAt: created, UpdatedAt: updated, CurrentRevID: manifest.CurrentRevID}
	revisions := make([]notes.Revision, 0, len(manifest.RevisionIDs))
	for _, revisionID := range manifest.RevisionIDs {
		data, _, err := s.get(ctx, s.revisionKey(noteID, revisionID))
		if err != nil {
			return notes.SyncSnapshot{}, "", err
		}
		var revision notes.Revision
		if err := json.Unmarshal(data, &revision); err != nil {
			return notes.SyncSnapshot{}, "", err
		}
		revisions = append(revisions, revision)
	}
	for _, revision := range revisions {
		if revision.ID == note.CurrentRevID {
			note.Title = revision.Title
			note.Content = revision.Content
			break
		}
	}
	note.RevisionCount = len(revisions)
	return notes.SyncSnapshot{Note: note, Revisions: revisions}, etag, nil
}

func (s *S3Client) DeleteNote(ctx context.Context, noteID string) error {
	// Remove the manifest first so a partially completed cleanup cannot expose the note.
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.manifestKey(noteID))}); err != nil {
		return err
	}
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(s.revisionPrefix(noteID))})
	if err != nil {
		return err
	}
	for _, object := range result.Contents {
		if object.Key == nil {
			continue
		}
		if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: object.Key}); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3Client) get(ctx context.Context, key string) ([]byte, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, "", err
	}
	defer result.Body.Close()
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, "", err
	}
	etag := ""
	if result.ETag != nil {
		etag = strings.Trim(*result.ETag, `"`)
	}
	return data, etag, nil
}

func (s *S3Client) put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: strings.NewReader(string(data)), ContentType: aws.String("application/json")})
	return err
}

func (s *S3Client) putWithETag(ctx context.Context, key string, data []byte, expectedETag string) (string, error) {
	input := &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: strings.NewReader(string(data)), ContentType: aws.String("application/json")}
	if expectedETag != "" {
		input.IfMatch = aws.String(expectedETag)
	}
	result, err := s.client.PutObject(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "412") || strings.Contains(strings.ToLower(err.Error()), "precondition") {
			return "", fmt.Errorf("%w: manifest changed remotely", ErrConflict)
		}
		return "", err
	}
	if result.ETag == nil {
		return "", nil
	}
	return strings.Trim(*result.ETag, `"`), nil
}

func (s *S3Client) pruneRemoteRevisions(ctx context.Context, noteID string, retained map[string]struct{}) error {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(s.revisionPrefix(noteID))})
	if err != nil {
		return err
	}
	for _, object := range result.Contents {
		if object.Key == nil {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(*object.Key, s.revisionPrefix(noteID)), ".json")
		if _, ok := retained[id]; ok {
			continue
		}
		if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: object.Key}); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3Client) manifestKey(noteID string) string {
	return s.objectKey("notes", noteID, "manifest.json")
}

func (s *S3Client) revisionPrefix(noteID string) string {
	return s.objectKey("notes", noteID, "revisions") + "/"
}

func (s *S3Client) revisionKey(noteID, revisionID string) string {
	return s.revisionPrefix(noteID) + revisionID + ".json"
}

func (s *S3Client) objectKey(parts ...string) string {
	values := append([]string{s.prefix}, parts...)
	return strings.Trim(strings.Join(values, "/"), "/")
}

var _ ConnectionChecker = (*S3Client)(nil)
var _ NoteSyncer = (*S3Client)(nil)
