package qr // QR codes for short texts (the LAN addresses): byte mode, level M, versions 1–6.

import "errors"

var ErrTooLong = errors.New("texto demasiado largo para el código QR")

// Code is a QR symbol without its quiet zone.
type Code struct {
	Size    int // modules per side: 17 + 4*Version
	Version int
	Mask    int
	dark    []bool // row-major
}

// Dark reports whether the module at column x, row y is dark.
func (c *Code) Dark(x, y int) bool { return c.dark[y*c.Size+x] }

// versionInfo describes one version at error correction level M.
type versionInfo struct {
	dataWords int // data codewords in total
	blocks    int
	ecPerBlk  int
	alignment []int // alignment pattern centers
	remainder int   // remainder bits after the codewords
}

// versions[v-1] is version v; every block of a version has the same size.
var versions = [...]versionInfo{
	{16, 1, 10, nil, 0},
	{28, 1, 16, []int{6, 18}, 7},
	{44, 1, 26, []int{6, 22}, 7},
	{64, 2, 18, []int{6, 26}, 7},
	{86, 2, 24, []int{6, 30}, 7},
	{108, 4, 16, []int{6, 34}, 7},
}

// Encode makes the symbol of text: the smallest version that fits and the
// mask with the lowest penalty (ties: the lowest mask number).
func Encode(text string) (*Code, error) {
	var best *Code
	bestScore := 0
	for mask := 0; mask < 8; mask++ {
		c, err := encode(text, mask)
		if err != nil {
			return nil, err
		}
		if score := penalty(c); best == nil || score < bestScore {
			best, bestScore = c, score
		}
	}
	return best, nil
}

// encode makes the symbol with a given mask (tests compare it with the
// reference matrices).
func encode(text string, mask int) (*Code, error) {
	version := 0
	for v, info := range versions {
		// 4 bits of mode + 8 of length leave dataWords-2 bytes for the text.
		if len(text) <= info.dataWords-2 {
			version = v + 1
			break
		}
	}
	if version == 0 {
		return nil, ErrTooLong
	}
	info := versions[version-1]
	size := 17 + 4*version

	words := interleave(splitBlocks(dataWords(text, info.dataWords), info), info)
	bits := make([]bool, 0, len(words)*8+info.remainder)
	for _, w := range words {
		for i := 7; i >= 0; i-- {
			bits = append(bits, w>>uint(i)&1 == 1)
		}
	}
	bits = append(bits, make([]bool, info.remainder)...)

	c := &Code{Size: size, Version: version, Mask: mask, dark: make([]bool, size*size)}
	fn := make([]bool, size*size) // function modules
	drawFunctionPatterns(c, fn, info)
	placeData(c, fn, bits, mask)
	drawFormat(c, mask)
	return c, nil
}

// dataWords builds the data codewords: mode, length, bytes, terminator and pads.
func dataWords(text string, n int) []byte {
	var bits []bool
	put := func(v, count int) {
		for i := count - 1; i >= 0; i-- {
			bits = append(bits, v>>uint(i)&1 == 1)
		}
	}
	put(0b0100, 4)
	put(len(text), 8)
	for i := 0; i < len(text); i++ {
		put(int(text[i]), 8)
	}
	put(0, min(4, n*8-len(bits)))
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	words := make([]byte, 0, n)
	for i := 0; i < len(bits); i += 8 {
		var w byte
		for _, b := range bits[i : i+8] {
			w <<= 1
			if b {
				w |= 1
			}
		}
		words = append(words, w)
	}
	for pad := byte(0xEC); len(words) < n; pad ^= 0xEC ^ 0x11 {
		words = append(words, pad)
	}
	return words
}

// block is the data and the error correction codewords of one block.
type block struct{ data, ec []byte }

func splitBlocks(data []byte, info versionInfo) []block {
	per := info.dataWords / info.blocks
	blocks := make([]block, info.blocks)
	for i := range blocks {
		d := data[i*per : (i+1)*per]
		blocks[i] = block{d, reedSolomon(d, info.ecPerBlk)}
	}
	return blocks
}

// interleave takes the data codewords column by column across the blocks,
// then the error correction codewords the same way.
func interleave(blocks []block, info versionInfo) []byte {
	var out []byte
	for i := 0; i < len(blocks[0].data); i++ {
		for _, b := range blocks {
			out = append(out, b.data[i])
		}
	}
	for i := 0; i < info.ecPerBlk; i++ {
		for _, b := range blocks {
			out = append(out, b.ec[i])
		}
	}
	return out
}

func (c *Code) set(fn []bool, x, y int, dark bool) {
	if x < 0 || y < 0 || x >= c.Size || y >= c.Size {
		return
	}
	c.dark[y*c.Size+x] = dark
	fn[y*c.Size+x] = true
}

func drawFunctionPatterns(c *Code, fn []bool, info versionInfo) {
	size := c.Size
	// Finder patterns with their separators, and the format areas next to them.
	for _, o := range [][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
		for dy := -1; dy <= 7; dy++ {
			for dx := -1; dx <= 7; dx++ {
				ring := max(abs(dx-3), abs(dy-3))
				c.set(fn, o[0]+dx, o[1]+dy, ring != 2 && ring <= 3)
			}
		}
	}
	for i := 0; i < 9; i++ {
		c.set(fn, 8, i, false)
		c.set(fn, i, 8, false)
	}
	for i := 0; i < 8; i++ {
		c.set(fn, size-1-i, 8, false)
		c.set(fn, 8, size-1-i, false)
	}
	// Timing patterns.
	for i := 8; i < size-8; i++ {
		c.set(fn, i, 6, i%2 == 0)
		c.set(fn, 6, i, i%2 == 0)
	}
	// Alignment patterns, except where they would overlap a finder.
	for _, cy := range info.alignment {
		for _, cx := range info.alignment {
			last := info.alignment[len(info.alignment)-1]
			if (cx == 6 && cy == 6) || (cx == 6 && cy == last) || (cx == last && cy == 6) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					c.set(fn, cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	c.set(fn, 8, size-8, true) // the always-dark module
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// maskFunc reports whether the mask inverts the module at row i, column j.
func maskFunc(mask, i, j int) bool {
	switch mask {
	case 0:
		return (i+j)%2 == 0
	case 1:
		return i%2 == 0
	case 2:
		return j%3 == 0
	case 3:
		return (i+j)%3 == 0
	case 4:
		return (i/2+j/3)%2 == 0
	case 5:
		return (i*j)%2+(i*j)%3 == 0
	case 6:
		return ((i*j)%2+(i*j)%3)%2 == 0
	default:
		return ((i+j)%2+(i*j)%3)%2 == 0
	}
}

// placeData writes the bits in the zig-zag order from the bottom-right corner,
// two columns at a time, applying the mask to them.
func placeData(c *Code, fn []bool, bits []bool, mask int) {
	size := c.Size
	n := 0
	upward := true
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5 // column 6 holds the timing pattern
		}
		for step := 0; step < size; step++ {
			y := step
			if upward {
				y = size - 1 - step
			}
			for dx := 0; dx < 2; dx++ {
				x := right - dx
				if fn[y*size+x] {
					continue
				}
				dark := n < len(bits) && bits[n]
				n++
				c.dark[y*size+x] = dark != maskFunc(mask, y, x)
			}
		}
		upward = !upward
	}
}

// formatBits is the 15-bit format information for level M and the mask:
// 5 data bits, BCH(15,5) remainder, XOR 101010000010010.
func formatBits(mask int) int {
	data := mask // level M is 00
	rem := data
	for i := 0; i < 10; i++ {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	return (data<<10 | rem) ^ 0x5412
}

func drawFormat(c *Code, mask int) {
	size := c.Size
	bits := formatBits(mask)
	bit := func(i int) bool { return bits>>uint(i)&1 == 1 }
	set := func(x, y int, dark bool) { c.dark[y*size+x] = dark }
	for i := 0; i <= 5; i++ {
		set(8, i, bit(i))
	}
	set(8, 7, bit(6))
	set(8, 8, bit(7))
	set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		set(size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		set(8, size-15+i, bit(i))
	}
}

// penalty scores the symbol with the four rules of ISO 18004 §7.8.3.
func penalty(c *Code) int {
	size := c.Size
	score := 0
	line := make([]bool, size)
	for dir := 0; dir < 2; dir++ { // rows, then columns
		for a := 0; a < size; a++ {
			for b := 0; b < size; b++ {
				if dir == 0 {
					line[b] = c.Dark(b, a)
				} else {
					line[b] = c.Dark(a, b)
				}
			}
			score += runPenalty(line) + finderPenalty(line)
		}
	}
	for y := 0; y+1 < size; y++ {
		for x := 0; x+1 < size; x++ {
			d := c.Dark(x, y)
			if d == c.Dark(x+1, y) && d == c.Dark(x, y+1) && d == c.Dark(x+1, y+1) {
				score += 3
			}
		}
	}
	dark := 0
	for _, d := range c.dark {
		if d {
			dark++
		}
	}
	total := size * size
	score += 10 * (abs(dark*100-50*total) / (5 * total))
	return score
}

// runPenalty scores runs of 5 or more modules of one color: 3 + run − 5.
func runPenalty(line []bool) int {
	score, run := 0, 1
	for i := 1; i <= len(line); i++ {
		if i < len(line) && line[i] == line[i-1] {
			run++
			continue
		}
		if run >= 5 {
			score += run - 2
		}
		run = 1
	}
	return score
}

// finderPenalty scores each 1011101 with four light modules on either side.
func finderPenalty(line []bool) int {
	pattern := [7]bool{true, false, true, true, true, false, true}
	score := 0
	for i := 0; i+7 <= len(line); i++ {
		match := true
		for k, p := range pattern {
			if line[i+k] != p {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		before, after := true, true
		for k := 1; k <= 4; k++ {
			// Modules outside the symbol do not count as light.
			if i-k < 0 || line[i-k] {
				before = false
			}
			if i+6+k >= len(line) || line[i+6+k] {
				after = false
			}
		}
		if before {
			score += 40
		}
		if after {
			score += 40
		}
	}
	return score
}
