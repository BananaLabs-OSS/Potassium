package diff

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// GenerateHDiff creates a patch using hdiffz between oldPath and newPath,
// writing the patch to patchPath. Shells out to the hdiffz binary.
func GenerateHDiff(oldPath, newPath, patchPath string) error {
	bin, err := findHDiffBinary("hdiffz")
	if err != nil {
		return err
	}

	cmd := exec.Command(bin, oldPath, newPath, patchPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hdiffz failed: %w", err)
	}
	return nil
}

// ApplyHDiff applies a patch using hpatchz to oldPath, writing the result to newPath.
// Shells out to the hpatchz binary.
func ApplyHDiff(oldPath, patchPath, newPath string) error {
	bin, err := findHDiffBinary("hpatchz")
	if err != nil {
		return err
	}

	cmd := exec.Command(bin, oldPath, patchPath, newPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hpatchz failed: %w", err)
	}
	return nil
}

// findHDiffBinary locates an HDiffPatch binary by checking:
//  1. bin/{os}/ relative to the running executable
//  2. bin/{os}/ relative to the working directory
//  3. System PATH
func findHDiffBinary(name string) (string, error) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	// 1. Next to the running executable
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "bin", runtime.GOOS, name+suffix)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 2. Relative to working directory (covers `go run .` during dev)
	if wd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(wd, "bin", runtime.GOOS, name+suffix)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 3. System PATH
	if path, err := exec.LookPath(name + suffix); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("%s not found: place it in bin/%s/ or add to PATH", name+suffix, runtime.GOOS)
}
