package index

import "testing"

func TestEncodeDecodeFloat32Vector(t *testing.T) {
	t.Parallel()

	original := []float32{0.1, -0.2, 0.3}
	encoded := EncodeFloat32Vector(original)
	decoded, err := DecodeFloat32Vector(encoded)
	if err != nil {
		t.Fatalf("decodeFloat32Vector() error = %v", err)
	}
	if len(decoded) != len(original) {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		if decoded[i] != original[i] {
			t.Fatalf("decoded[%d] = %f, want %f", i, decoded[i], original[i])
		}
	}
}
