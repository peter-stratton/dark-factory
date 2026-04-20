package prompts

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"sort"
	"sync"
)

// Hash returns a SHA256 hex digest over all embedded prompt files. The digest
// is stable for a given binary: it identifies which prompt bundle shipped with
// godark at build time. A meta-agent editing prompts between runs can use the
// hash to attribute improvements to specific harness versions rather than to
// benchmark drift.
//
// The computation walks the embed.FS in sorted filename order and feeds each
// "<name>\x00<contents>\x00" segment into the hasher. It is computed once per
// process and memoized.
func Hash() string {
	hashOnce.Do(computeHash)
	return hashValue
}

var (
	hashOnce  sync.Once
	hashValue string
)

func computeHash() {
	names, err := collectNames()
	if err != nil {
		return
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		data, err := FS.ReadFile(name)
		if err != nil {
			return
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	hashValue = hex.EncodeToString(h.Sum(nil))
}

func collectNames() ([]string, error) {
	var names []string
	err := fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		names = append(names, path)
		return nil
	})
	return names, err
}
