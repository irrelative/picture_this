package server

import "crypto/rand"

func newJoinCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "AAAAAA"
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

func pickPlayerColor(index int) string {
	palette := []string{
		"#ff6b6b",
		"#4dabf7",
		"#51cf66",
		"#ffa94d",
		"#ffd43b",
		"#845ef7",
		"#20c997",
		"#e64980",
	}
	if len(palette) == 0 {
		return "#1a1a1a"
	}
	if index < 0 {
		index = 0
	}
	return palette[index%len(palette)]
}
