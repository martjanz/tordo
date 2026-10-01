package nameparse

import "strings"

// Compose turns decomposed Latin letters ("e" + U+0301, as macOS writes file
// names) into precomposed ones ("é"), so names from folders read like names
// from tags. Other text is left as is.
func Compose(s string) string {
	if !strings.ContainsFunc(s, isMark) {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) {
			if c, ok := composed[[2]rune{rs[i], rs[i+1]}]; ok {
				b.WriteRune(c)
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

func isMark(r rune) bool { return r >= 0x300 && r <= 0x36F }

// composed maps a base letter and a combining mark to the precomposed letter.
var composed = func() map[[2]rune]rune {
	m := map[[2]rune]rune{}
	add := func(mark rune, pairs string) { // base, composed, base, composed…
		rs := []rune(pairs)
		for i := 0; i+1 < len(rs); i += 2 {
			m[[2]rune{rs[i], mark}] = rs[i+1]
		}
	}
	add('\u0300', "aàeèiìoòuùAÀEÈIÌOÒUÙ")
	add('\u0301', "aáeéiíoóuúyýAÁEÉIÍOÓUÚYÝcćCĆnńNŃsśSŚzźZŹ")
	add('\u0302', "aâeêiîoôuûAÂEÊIÎOÔUÛ")
	add('\u0303', "aãoõnñAÃOÕNÑ")
	add('\u0308', "aäeëiïoöuüyÿAÄEËIÏOÖUÜ")
	add('\u0327', "cçCÇ")
	add('\u030a', "aåAÅ")
	return m
}()
