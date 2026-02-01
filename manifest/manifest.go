package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"time"
)

type Action string

const (
	ActionPatch  Action = "patch"
	ActionAdd    Action = "add"
	ActionDelete Action = "delete"
)

type FileEntry struct {
	Path      string `json:"path"`
	Action    Action `json:"action"`
	OldHash   string `json:"old_hash,omitempty"`
	NewHash   string `json:"new_hash,omitempty"`
	PatchFile string `json:"patch_file,omitempty"`
	Algorithm string `json:"algorithm,omitempty"`
}

type Manifest struct {
	FromVersion string      `json:"from_version"`
	ToVersion   string      `json:"to_version"`
	Created     time.Time   `json:"created"`
	Files       []FileEntry `json:"files"`
}

// New creates a new manifest
func New(fromVersion, toVersion string) *Manifest {
	return &Manifest{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Created:     time.Now().UTC(),
		Files:       []FileEntry{},
	}
}

// AddFile adds a file entry to the manifest
func (m *Manifest) AddFile(entry FileEntry) {
	m.Files = append(m.Files, entry)
}

// Save writes the manifest to a file
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads a manifest from a file
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

// HashFile computes SHA256 hash of a file
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
