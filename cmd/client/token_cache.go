package main

import (
	"flag"
	"os"
	"path/filepath"
)

var (
	noCache = flag.Bool("no-cache", false, "skip writing token to local cache file")
)

// TokenStore abstracts token persistence; production would use OS keychain.
type TokenStore interface {
	Write(token string) error
}

type fileTokenStore struct {
	path string
}

func newFileTokenStore() (*fileTokenStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &fileTokenStore{path: filepath.Join(home, ".gh-int-demo-token")}, nil
}

func (s *fileTokenStore) Write(token string) error {
	content := "# demo only; do not commit\n" + token + "\n"
	return os.WriteFile(s.path, []byte(content), 0o600)
}

func writeTokenCache(token string) error {
	if *noCache {
		return nil
	}
	store, err := newFileTokenStore()
	if err != nil {
		return err
	}
	return store.Write(token)
}
