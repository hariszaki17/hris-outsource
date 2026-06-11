// Package storage — unit tests for the object-storage key namespace and the
// presigned-PUT size/content-type policy. These exercise the pure, network-free
// surface (key builder + allowlist + the pre-presign validation branches of
// PresignPut), so they need no running MinIO: the content-type/size checks both
// short-circuit before any minio-go call, and buildKey is a pure function.
package storage

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Key namespace
// ---------------------------------------------------------------------------

func TestBuildKey_NamespaceAndShape(t *testing.T) {
	key := buildKey(NSProfilePhotos, "SWP-EMP-1042", "jpg")

	// Server-built keys live under the namespace + per-employee segment so a
	// client can never write outside its own owner namespace.
	wantPrefix := "profile-photos/SWP-EMP-1042/"
	if !strings.HasPrefix(key, wantPrefix) {
		t.Fatalf("buildKey = %q, want prefix %q", key, wantPrefix)
	}
	if !strings.HasSuffix(key, ".jpg") {
		t.Errorf("buildKey = %q, want .jpg extension", key)
	}

	// The filename segment (after the owner prefix) is a ULID + ext — non-empty
	// and free of path separators (no traversal).
	tail := strings.TrimPrefix(key, wantPrefix)
	if tail == "" || strings.Contains(tail, "/") {
		t.Errorf("buildKey tail = %q, want a single ulid.ext segment", tail)
	}
}

func TestBuildKey_DistinctPerCall(t *testing.T) {
	// The ULID makes every key collision-free even for the same owner/namespace.
	a := buildKey(NSAttendanceSelfies, "SWP-EMP-1", "png")
	b := buildKey(NSAttendanceSelfies, "SWP-EMP-1", "png")
	if a == b {
		t.Errorf("buildKey produced identical keys on two calls: %q", a)
	}
	if !strings.HasPrefix(a, "attendance-selfies/SWP-EMP-1/") {
		t.Errorf("buildKey = %q, want attendance-selfies namespace", a)
	}
}

// ---------------------------------------------------------------------------
// Content-type allowlist
// ---------------------------------------------------------------------------

func TestAllowlistFor_ImageNamespacesShareImageAllowlist(t *testing.T) {
	for _, ns := range []Namespace{NSProfilePhotos, NSAttendanceSelfies} {
		allow := allowlistFor(ns)
		if allow == nil {
			t.Fatalf("allowlistFor(%q) = nil, want image allowlist", ns)
		}
		for _, ct := range []string{"image/jpeg", "image/png", "image/webp"} {
			if _, ok := allow[ct]; !ok {
				t.Errorf("allowlistFor(%q) missing %q", ns, ct)
			}
		}
		if _, ok := allow["application/pdf"]; ok {
			t.Errorf("allowlistFor(%q) must NOT allow application/pdf", ns)
		}
	}
}

func TestAllowlistFor_UnknownNamespaceNil(t *testing.T) {
	if allowlistFor(Namespace("nope")) != nil {
		t.Error("allowlistFor(unknown) = non-nil, want nil")
	}
}

// ---------------------------------------------------------------------------
// PresignPut size + content-type policy (pre-presign branches; no MinIO needed)
// ---------------------------------------------------------------------------

func TestPresignPut_DisallowedContentType(t *testing.T) {
	// mc is nil — the content-type check returns before any minio-go call.
	c := &client{maxUploadBytes: 5 << 20}
	_, err := c.PresignPut(context.Background(), NSProfilePhotos, "SWP-EMP-1", "application/pdf", 1024)
	if err != ErrContentTypeNotAllowed {
		t.Fatalf("PresignPut(pdf) err = %v, want ErrContentTypeNotAllowed", err)
	}
}

func TestPresignPut_OversizeRejected(t *testing.T) {
	const maxBytes = 5 << 20 // 5 MiB
	c := &client{maxUploadBytes: maxBytes}
	// A declared size above the cap is rejected before any minio-go call.
	_, err := c.PresignPut(context.Background(), NSProfilePhotos, "SWP-EMP-1", "image/jpeg", maxBytes+1)
	if err != ErrFileTooLarge {
		t.Fatalf("PresignPut(oversize) err = %v, want ErrFileTooLarge", err)
	}
}

func TestPresignPut_ContentTypeCheckedBeforeSize(t *testing.T) {
	// A disallowed type that is ALSO oversize surfaces the content-type error
	// first (allowlist gate precedes the size gate).
	c := &client{maxUploadBytes: 5 << 20}
	_, err := c.PresignPut(context.Background(), NSProfilePhotos, "SWP-EMP-1", "application/zip", (5<<20)+999)
	if err != ErrContentTypeNotAllowed {
		t.Fatalf("PresignPut err = %v, want ErrContentTypeNotAllowed (type checked before size)", err)
	}
}
