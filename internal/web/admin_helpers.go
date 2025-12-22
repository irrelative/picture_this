package web

import (
	"encoding/base64"
	"time"
)

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}

func encodeImageData(image []byte) string {
	if len(image) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
}
