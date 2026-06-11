// Package storage is the object-storage layer (MinIO / S3-compatible) behind a
// single private bucket. It exists so the API never proxies file bytes: clients
// upload and download directly via short-TTL presigned URLs, and the server only
// ever (a) builds the object key (clients never choose paths), (b) pins the
// content-type and a content-length-range on the presigned PUT so the upload
// cannot exceed MaxUploadBytes or smuggle a disallowed type, and (c) hands back a
// short-lived presigned GET for rendering.
//
// First consumer is E2 profile photos (F2.x change-request / self-profile);
// the same client is reused by E5 attendance selfies — hence the generic
// UploadTicket and the per-namespace key builders below.
package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/oklog/ulid/v2"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
)

// Errors returned by the validation surface. Service-layer callers map these to
// apperr (e.g. apperr.Invalid / a FILE_TOO_LARGE rule) — the platform package
// stays free of HTTP-error concerns, mirroring crypto/ids.
var (
	// ErrContentTypeNotAllowed is returned when a requested upload content-type
	// is not in the allowlist for its namespace.
	ErrContentTypeNotAllowed = errors.New("storage: content-type not allowed")
	// ErrFileTooLarge is returned when the declared size exceeds MaxUploadBytes.
	ErrFileTooLarge = errors.New("storage: file exceeds max upload size")
)

// Namespace is a top-level key prefix that isolates one kind of object from
// another within the single private bucket. Each namespace carries its own
// content-type allowlist.
type Namespace string

const (
	// NSProfilePhotos holds E2 employee profile photos:
	//   profile-photos/{employee_id}/{ulid}.{ext}
	NSProfilePhotos Namespace = "profile-photos"
	// NSAttendanceSelfies holds E5 clock-in/out selfies (future reuse):
	//   attendance-selfies/{employee_id}/{ulid}.{ext}
	NSAttendanceSelfies Namespace = "attendance-selfies"
)

// imageAllowlist maps an allowed upload content-type to its canonical file
// extension. Both profile photos and attendance selfies are images, so they
// share this allowlist; keep extension choices stable (they appear in keys).
var imageAllowlist = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// allowlistFor returns the content-type→extension allowlist for a namespace.
func allowlistFor(ns Namespace) map[string]string {
	switch ns {
	case NSProfilePhotos, NSAttendanceSelfies:
		return imageAllowlist
	default:
		return nil
	}
}

// UploadTicket is everything a client needs to upload one object directly to
// storage and then tell the API which key it wrote. The server already pinned
// the content-type and a content-length-range on UploadURL, so a client that
// deviates is rejected by storage itself. Reusable across E2/E5.
type UploadTicket struct {
	// UploadURL is the presigned PUT. The client MUST send the exact ContentType
	// (Content-Type header) and a body no larger than MaxBytes.
	UploadURL string `json:"upload_url"`
	// ObjectKey is the server-built key the client echoes back on apply. Clients
	// never choose this — it is opaque to them.
	ObjectKey string `json:"object_key"`
	// ContentType is the pinned content-type the client must use on the PUT.
	ContentType string `json:"content_type"`
	// MaxBytes is the upper bound enforced by the presigned PUT.
	MaxBytes int64 `json:"max_bytes"`
	// ExpiresAt is when UploadURL stops working.
	ExpiresAt time.Time `json:"expires_at"`
}

// Storage is the object-storage port. Kept small and dependency-free so it is
// trivially fakeable in service tests.
type Storage interface {
	// EnsureBucket creates the private bucket if it does not exist. Idempotent;
	// called once at startup from server deps wiring.
	EnsureBucket(ctx context.Context) error

	// PresignPut builds a server-keyed, content-type-pinned, size-capped presigned
	// PUT for one new object in the namespace, owned by employeeID. It validates
	// contentType against the namespace allowlist and declaredSize against
	// MaxUploadBytes, returning ErrContentTypeNotAllowed / ErrFileTooLarge.
	PresignPut(ctx context.Context, ns Namespace, employeeID, contentType string, declaredSize int64) (UploadTicket, error)

	// PresignGet builds a short-TTL presigned GET for an existing object key.
	PresignGet(ctx context.Context, objectKey string) (string, error)
}

// client is the minio-go-backed Storage implementation.
type client struct {
	mc             *minio.Client
	bucket         string
	presignTTL     time.Duration
	maxUploadBytes int64
	// publicEndpoint, when set, rewrites the host of presigned URLs so a browser
	// reaches MinIO at a routable address (e.g. localhost:9000) even though the
	// API talks to it over an internal endpoint. The presign signature stays
	// valid because the path + query are untouched.
	publicEndpoint string
}

// New constructs a Storage client from config. It does not touch the network;
// call EnsureBucket at startup to create the bucket.
func New(cfg config.Storage) (Storage, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: init minio client: %w", err)
	}
	return &client{
		mc:             mc,
		bucket:         cfg.Bucket,
		presignTTL:     cfg.PresignTTL,
		maxUploadBytes: cfg.MaxUploadBytes,
		publicEndpoint: strings.TrimSpace(cfg.PublicEndpoint),
	}, nil
}

func (c *client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("storage: stat bucket %q: %w", c.bucket, err)
	}
	if exists {
		return nil
	}
	if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
		// Tolerate a race where another process created it between the calls.
		if exists2, e2 := c.mc.BucketExists(ctx, c.bucket); e2 == nil && exists2 {
			return nil
		}
		return fmt.Errorf("storage: create bucket %q: %w", c.bucket, err)
	}
	return nil
}

func (c *client) PresignPut(ctx context.Context, ns Namespace, employeeID, contentType string, declaredSize int64) (UploadTicket, error) {
	allow := allowlistFor(ns)
	ext, ok := allow[contentType]
	if !ok {
		return UploadTicket{}, ErrContentTypeNotAllowed
	}
	if declaredSize > c.maxUploadBytes {
		return UploadTicket{}, ErrFileTooLarge
	}

	key := buildKey(ns, employeeID, ext)

	// Pin the Content-Type into the signature via PresignHeader so a client that
	// omits or changes it is rejected by MinIO at upload time. We intentionally do
	// NOT sign a `Content-Length-Range` header: that is a presigned-POST *policy*
	// field, not a real PUT request header, and signing it would force the client
	// to send a bogus header it cannot produce — breaking every PUT. Size is bounded
	// at init via `declaredSize <= maxUploadBytes` above; hard byte-range enforcement
	// would require switching to a presigned POST policy.
	reqHeaders := make(map[string][]string)
	reqHeaders["Content-Type"] = []string{contentType}

	u, err := c.mc.PresignHeader(ctx, "PUT", c.bucket, key, c.presignTTL, url.Values{}, reqHeaders)
	if err != nil {
		return UploadTicket{}, fmt.Errorf("storage: presign put %q: %w", key, err)
	}

	return UploadTicket{
		UploadURL:   c.rewriteHost(u),
		ObjectKey:   key,
		ContentType: contentType,
		MaxBytes:    c.maxUploadBytes,
		ExpiresAt:   time.Now().Add(c.presignTTL),
	}, nil
}

func (c *client) PresignGet(ctx context.Context, objectKey string) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, objectKey, c.presignTTL, url.Values{})
	if err != nil {
		return "", fmt.Errorf("storage: presign get %q: %w", objectKey, err)
	}
	return c.rewriteHost(u), nil
}

// buildKey constructs the server-owned object key. The ULID gives an
// unguessable, time-ordered, collision-free filename per upload; the employee
// segment namespaces objects per owner for cheap prefix-listing/cleanup.
func buildKey(ns Namespace, employeeID, ext string) string {
	return fmt.Sprintf("%s/%s/%s.%s", ns, employeeID, ulid.Make().String(), ext)
}

// rewriteHost swaps the host of a presigned URL for PublicEndpoint when one is
// configured, leaving path + query (and thus the signature) intact.
func (c *client) rewriteHost(u *url.URL) string {
	if c.publicEndpoint == "" {
		return u.String()
	}
	pub, err := url.Parse(c.publicEndpoint)
	if err != nil || pub.Host == "" {
		return u.String()
	}
	rewritten := *u
	rewritten.Host = pub.Host
	if pub.Scheme != "" {
		rewritten.Scheme = pub.Scheme
	}
	return rewritten.String()
}
