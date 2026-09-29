package localstore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dujiao-next/internal/modules/upload/contract"
)

func TestStoreSavesFileUnderConfiguredRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	store := New(root)
	url, err := store.Save(contract.StoreInput{
		Source:   bytes.NewBufferString("asset-content"),
		Scene:    "common",
		Year:     "2026",
		Month:    "07",
		Filename: "asset.txt",
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if url != "/uploads/common/2026/07/asset.txt" {
		t.Fatalf("public URL got %q", url)
	}
	data, err := os.ReadFile(filepath.Join(root, "common", "2026", "07", "asset.txt"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "asset-content" {
		t.Fatalf("saved content got %q", data)
	}
}
