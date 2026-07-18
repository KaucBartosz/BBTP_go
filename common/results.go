package common

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func RandomSubjectID() string {
	return time.Now().Format("20060102150405")
}

func RandomID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(4)
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[rand.Intn(len(hex))]
	}
	return string(b)
}

func WriteResults(dir string, data interface{}) error {
	outPath := filepath.Join(dir, "results.json")
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, b, 0644)
}

func Timestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
