package qr

// GF(256) with the primitive polynomial x^8 + x^4 + x^3 + x^2 + 1 (0x11D).
var (
	gfExp [512]byte // doubled so products need no modulo
	gfLog [256]int
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = i
		x <<= 1
		if x >= 256 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[gfLog[a]+gfLog[b]]
}

// reedSolomon returns the n error correction codewords of data: the remainder
// of data·x^n divided by the generator ∏(x − α^i), i = 0…n−1.
func reedSolomon(data []byte, n int) []byte {
	gen := make([]byte, n) // coefficients below the leading 1, highest degree first
	gen[n-1] = 1
	for i := 0; i < n; i++ {
		root := gfExp[i]
		for j := range gen {
			gen[j] = gfMul(gen[j], root)
			if j+1 < n {
				gen[j] ^= gen[j+1]
			}
		}
	}
	rem := make([]byte, n)
	for _, b := range data {
		factor := b ^ rem[0]
		copy(rem, rem[1:])
		rem[n-1] = 0
		for j := range rem {
			rem[j] ^= gfMul(gen[j], factor)
		}
	}
	return rem
}
