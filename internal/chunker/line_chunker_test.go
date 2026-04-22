package chunker

import (
	"reflect"
	"testing"
)

func TestLineChunkerChunk(t *testing.T) {
	t.Parallel()

	got := LineChunker{}.Chunk("first\n\nthird\n")
	want := []Chunk{
		{LineNumber: 0, Content: "first"},
		{LineNumber: 2, Content: "third"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Chunk() = %#v, want %#v", got, want)
	}
}
