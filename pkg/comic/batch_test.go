package comic

import (
	"context"
	"testing"
)

func TestIsDirOutput(t *testing.T) {
	if isDirOutput("") {
		t.Error("empty string should not be dir output")
	}
	if !isDirOutput("/some/path") {
		t.Error("any explicit path should be treated as dir output")
	}
	if !isDirOutput(".") {
		t.Error("'.' should be dir output")
	}
}

func TestConvertBatchNoMatch(t *testing.T) {
	err := ConvertBatch(context.Background(), "/tmp/no-match-zzz-*.nonexistent", Options{})
	if err == nil {
		t.Error("expected error for pattern with no matches")
	}
}

func TestConvertBatchBadPattern(t *testing.T) {
	err := ConvertBatch(context.Background(), "[]", Options{})
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}
