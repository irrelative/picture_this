package web

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func itoa(value int) string {
	return strconv.Itoa(value)
}

func utoa(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func pageURL(base string, page, perPage int) string {
	if strings.Contains(base, "?") {
		return base + "&page=" + itoa(page) + "&per_page=" + itoa(perPage)
	}
	return base + "?page=" + itoa(page) + "&per_page=" + itoa(perPage)
}

var prodAssetVersion = func() string {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	sum := sha256.Sum256([]byte(startedAt))
	return hex.EncodeToString(sum[:8])
}()

func assetPath(path string) string {
	if path == "" || !strings.HasPrefix(path, "/static/") {
		return path
	}
	if os.Getenv("ENV") == "prod" {
		return appendAssetVersion(path, prodAssetVersion)
	}
	trimmed := strings.TrimPrefix(path, "/static/")
	fsPath := filepath.Join("static", trimmed)
	data, err := os.ReadFile(fsPath)
	if err != nil {
		return path
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:8])
	return appendAssetVersion(path, hash)
}

func appendAssetVersion(path string, hash string) string {
	if hash == "" {
		return path
	}
	if strings.Contains(path, "?") {
		return path + "&v=" + hash
	}
	return path + "?v=" + hash
}
