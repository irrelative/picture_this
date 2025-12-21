package server

import (
	"encoding/base64"
	"errors"
	"strings"
)

func decodeImageData(data string) ([]byte, error) {
	data = strings.TrimSpace(data)
	if data == "" {
		return nil, errors.New("no image data")
	}
	parts := strings.SplitN(data, ",", 2)
	if len(parts) == 2 {
		data = parts[1]
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func encodeImageData(image []byte) string {
	if len(image) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
}
