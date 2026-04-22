package output

import (
	"strings"
	"testing"
)

func TestNormalizeExts(t *testing.T) {
	t.Parallel()

	got := normalizeExts([]string{".GO", "md", ".go", "", ".MD"})
	want := []string{"go", "md"}

	if len(got) != len(want) {
		t.Fatalf("len(normalizeExts()) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeExts()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHighlight(t *testing.T) {
	t.Parallel()

	p := Printer{color: true}
	got := p.highlight("hello sqlc world", []string{"sqlc"})

	if !strings.Contains(got, colorYellow) {
		t.Fatalf("highlight() missing color code: %q", got)
	}
	if !strings.Contains(got, "sqlc") {
		t.Fatalf("highlight() missing token: %q", got)
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		size int64
		want string
	}{
		{size: 999, want: "999 B"},
		{size: 1024, want: "1.0 KB"},
		{size: 1024 * 1024, want: "1.0 MB"},
	}

	for _, tt := range tests {
		if got := formatBytes(tt.size); got != tt.want {
			t.Fatalf("formatBytes(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}
