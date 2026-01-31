package diff

import (
	"os"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/gabstv/go-bsdiff/pkg/bspatch"
)

// Generate creates a binary patch from old to new
func Generate(oldPath, newPath string) ([]byte, error) {
	oldBytes, err := os.ReadFile(oldPath)
	if err != nil {
		return nil, err
	}

	newBytes, err := os.ReadFile(newPath)
	if err != nil {
		return nil, err
	}

	return bsdiff.Bytes(oldBytes, newBytes)
}

// Apply applies a patch to an old file, producing the new file
func Apply(oldPath, patchPath, outPath string) error {
	oldBytes, err := os.ReadFile(oldPath)
	if err != nil {
		return err
	}

	patchBytes, err := os.ReadFile(patchPath)
	if err != nil {
		return err
	}

	newBytes, err := bspatch.Bytes(oldBytes, patchBytes)
	if err != nil {
		return err
	}

	return os.WriteFile(outPath, newBytes, 0644)
}

// GenerateBytes creates a patch from byte slices directly
func GenerateBytes(oldData, newData []byte) ([]byte, error) {
	return bsdiff.Bytes(oldData, newData)
}

// ApplyBytes applies a patch to old bytes, returning new bytes
func ApplyBytes(oldData, patchData []byte) ([]byte, error) {
	return bspatch.Bytes(oldData, patchData)
}
