package tokenizer

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	t.Parallel()

	got := Tokenize("Hello, Go_sqlc!\nJSON-API 123")
	want := []string{"hello", "go_sqlc", "json", "api", "123"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokenize() = %#v, want %#v", got, want)
	}
}

func TestTokenizeSkipsEmptyParts(t *testing.T) {
	t.Parallel()

	got := Tokenize("...___...")
	want := []string{"___"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokenize() = %#v, want %#v", got, want)
	}
}
