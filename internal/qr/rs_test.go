package qr

import (
	"slices"
	"testing"
)

func TestReedSolomonHelloWorld(t *testing.T) {
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	if got := reedSolomon(data, 10); !slices.Equal(got, want) {
		t.Errorf("EC codewords = %v, want %v", got, want)
	}
}

func TestGaloisField(t *testing.T) {
	for x := 1; x < 256; x++ {
		if got := gfExp[gfLog[x]]; int(got) != x {
			t.Fatalf("exp[log[%d]] = %d", x, got)
		}
	}
	if gfExp[8] != 0x1D {
		t.Errorf("alpha^8 = %#x, want 0x1d", gfExp[8])
	}
}
