package qr

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type fixture struct {
	name    string
	text    string
	version int
	mask    int
	rows    []string
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	var out []fixture
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fx := fixture{name: filepath.Base(f)}
		for _, line := range strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n") {
			line = strings.TrimRight(line, "\r")
			switch {
			case strings.HasPrefix(line, "# text: "):
				fx.text = strings.TrimPrefix(line, "# text: ")
			case strings.HasPrefix(line, "# version: "):
				fmt.Sscanf(line, "# version: %d", &fx.version)
			case strings.HasPrefix(line, "# mask: "):
				fmt.Sscanf(line, "# mask: %d", &fx.mask)
			case strings.HasPrefix(line, "# level"):
			default:
				fx.rows = append(fx.rows, line)
			}
		}
		out = append(out, fx)
	}
	return out
}

func TestEncodeMatchesFixtures(t *testing.T) {
	for _, fx := range loadFixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			c, err := encode(fx.text, fx.mask)
			if err != nil {
				t.Fatal(err)
			}
			if c.Version != fx.version || c.Size != 17+4*fx.version || len(fx.rows) != c.Size {
				t.Fatalf("version %d size %d, fixture version %d with %d rows", c.Version, c.Size, fx.version, len(fx.rows))
			}
			for y, row := range fx.rows {
				if len(row) != c.Size {
					t.Fatalf("fixture row %d has %d columns", y, len(row))
				}
				for x := 0; x < c.Size; x++ {
					if c.Dark(x, y) != (row[x] == '#') {
						t.Fatalf("first difference at column %d, row %d: got dark=%v", x, y, c.Dark(x, y))
					}
				}
			}
		})
	}
}

func TestEncodePicksLowestPenalty(t *testing.T) {
	for _, fx := range loadFixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			c, err := Encode(fx.text)
			if err != nil {
				t.Fatal(err)
			}
			if c.Version != fx.version {
				t.Fatalf("version %d, want %d", c.Version, fx.version)
			}
			best, bestScore := -1, 0
			for m := 0; m < 8; m++ {
				cm, _ := encode(fx.text, m)
				if s := penalty(cm); best < 0 || s < bestScore {
					best, bestScore = m, s
				}
			}
			if c.Mask != best {
				t.Errorf("mask %d, want %d (lowest penalty %d)", c.Mask, best, bestScore)
			}
			want, _ := encode(fx.text, c.Mask)
			if !slices.Equal(c.dark, want.dark) {
				t.Error("modules differ from encode with the chosen mask")
			}
		})
	}
}

// blankFormat returns a copy of c with every format-information module light
// (the dark module stays), as python-qrcode has them when it scores the masks.
func blankFormat(c *Code) *Code {
	b := &Code{Size: c.Size, Version: c.Version, Mask: c.Mask, dark: slices.Clone(c.dark)}
	n := c.Size
	for i := 0; i < 9; i++ {
		if i == 6 {
			continue // timing pattern
		}
		b.dark[i*n+8] = false // column 8, rows 0-8
		b.dark[8*n+i] = false // row 8, columns 0-8
	}
	for i := 0; i < 8; i++ {
		b.dark[8*n+n-1-i] = false // row 8, right copy
		if i > 0 {
			b.dark[(n-1-i)*n+8] = false // column 8, bottom copy (not the dark module)
		}
	}
	return b
}

// The reference library scores the masks with blank format modules; with
// that, our penalty must choose the fixture's mask.
func TestPenaltyReproducesReferenceMask(t *testing.T) {
	for _, fx := range loadFixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			best, bestScore := -1, 0
			scores := make([]int, 8)
			for m := 0; m < 8; m++ {
				cm, _ := encode(fx.text, m)
				scores[m] = penalty(blankFormat(cm))
				if best < 0 || scores[m] < bestScore {
					best, bestScore = m, scores[m]
				}
			}
			if best != fx.mask {
				t.Errorf("argmin %d, fixture mask %d (penalties %v)", best, fx.mask, scores)
			}
		})
	}
}

func TestFormatBits(t *testing.T) {
	if got := formatBits(0); got != 0b101010000010010 {
		t.Errorf("format bits for mask 0 = %015b", got)
	}
}

func TestEncodeLengths(t *testing.T) {
	c, err := Encode(strings.Repeat("a", 106))
	if err != nil || c.Version != 6 {
		t.Fatalf("106 bytes: %v, %+v", err, c)
	}
	if _, err := Encode(strings.Repeat("a", 107)); err != ErrTooLong {
		t.Errorf("107 bytes: err = %v, want ErrTooLong", err)
	}
}

func TestSVG(t *testing.T) {
	var v1 fixture
	for _, fx := range loadFixtures(t) {
		if fx.version == 1 {
			v1 = fx
		}
	}
	c, err := Encode(v1.text)
	if err != nil {
		t.Fatal(err)
	}
	svg := c.SVG(4)
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, `viewBox="0 0 29 29"`) || strings.Count(svg, "<path") != 1 {
		t.Errorf("unexpected SVG: %s", svg)
	}
}
