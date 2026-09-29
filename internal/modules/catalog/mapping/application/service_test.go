package application

import (
	"context"
	"errors"
	"testing"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
)

type recordingMediaRecorder struct {
	ctx       context.Context
	localPath string
	scene     string
}

func (r *recordingMediaRecorder) RecordLocalFile(ctx context.Context, localPath, scene string) {
	r.ctx = ctx
	r.localPath = localPath
	r.scene = scene
}

func TestNewServiceRejectsNilMediaRecorder(t *testing.T) {
	if _, err := NewService(Options{}); !errors.Is(err, mappingcontract.ErrMediaRecorderRequired) {
		t.Fatalf("error = %v, want mappingcontract.ErrMediaRecorderRequired", err)
	}
}

func TestRecordUpstreamMediaPropagatesCallerContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "mapping-request")
	recorder := &recordingMediaRecorder{}
	service := &Service{media: recorder}

	service.recordUpstreamMedia(ctx, "/uploads/upstream/image.png")

	if recorder.ctx.Value(contextKey{}) != "mapping-request" {
		t.Fatal("caller context was not propagated")
	}
	if recorder.localPath != "/uploads/upstream/image.png" || recorder.scene != "upstream" {
		t.Fatalf("recorded media = (%q, %q)", recorder.localPath, recorder.scene)
	}
}
