package file

import "testing"

func TestNormalizeExt(t *testing.T) {
	t.Parallel()

	if got := NormalizeExt("foo/Bar.GO"); got != ".go" {
		t.Fatalf("NormalizeExt() = %q, want %q", got, ".go")
	}
}

func TestIsAllowed(t *testing.T) {
	t.Parallel()

	if !IsAllowed("main.go") {
		t.Fatalf("IsAllowed(main.go) = false, want true")
	}

	if IsAllowed("archive.zip") {
		t.Fatalf("IsAllowed(archive.zip) = true, want false")
	}
}
