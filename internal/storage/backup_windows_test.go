package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestBackupDirectoryAllowsWindowsShortNames(t *testing.T) {
	// The source checkout and user profile can live on volumes with different
	// short-name settings. This check only reads the profile directory.
	for _, directory := range []string{t.TempDir(), os.Getenv("LOCALAPPDATA")} {
		if !filepath.IsAbs(directory) {
			continue
		}
		longPath, err := windows.UTF16PtrFromString(directory)
		if err != nil {
			t.Fatal(err)
		}
		var buffer [32768]uint16
		n, err := windows.GetShortPathName(longPath, &buffer[0], uint32(len(buffer)))
		if err != nil || n == 0 || n >= uint32(len(buffer)) {
			continue
		}
		shortPath := windows.UTF16ToString(buffer[:n])
		resolved, err := filepath.EvalSymlinks(shortPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.EqualFold(shortPath, resolved) {
			continue
		}
		if err := requireBackupDirectory(filepath.Join(shortPath, "new-child")); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Skip("no alternate short name is available on this host")
}
