package cache

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
)

const appName = "req2"

func cachePath(templateStr string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	hash := md5.Sum([]byte(templateStr))
	cacheKey := hex.EncodeToString(hash[:])
	return filepath.Join(cacheDir, appName, cacheKey), nil
}

func RetrieveCache(templateStr string) (string, error) {
	cachePath, err := cachePath(templateStr)
	if err != nil {
		return "", err
	}
	cacheContent, err := os.ReadFile(cachePath)
	if err != nil {
		return "", err
	}

	return string(cacheContent), nil
}

func StoreCache(templateStr, response string) error {
	cachePath, err := cachePath(templateStr)
	if err != nil {
		return err
	}
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return err
	}
	return os.WriteFile(cachePath, []byte(response), 0644)
}

func ClearCache() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(cacheDir, appName))
}
