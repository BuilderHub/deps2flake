package flake

import (
	"strings"
	"testing"
)

func TestEnsureWritableRequiresPath(t *testing.T) {
	err := EnsureWritable("", false)
	if err == nil {
		t.Fatal("expected path error")
	}
	if !strings.Contains(err.Error(), "flake path is required") {
		t.Fatalf("error does not explain missing path: %v", err)
	}
}
