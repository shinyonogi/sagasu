package index

import (
	"encoding/binary"
	"fmt"
	"math"
)

func EncodeFloat32Vector(vector []float32) []byte {
	buf := make([]byte, len(vector)*4)
	for i, value := range vector {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(value))
	}
	return buf
}

func DecodeFloat32Vector(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid vector byte length: %d", len(data))
	}
	vector := make([]float32, len(data)/4)
	for i := range vector {
		vector[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return vector, nil
}
