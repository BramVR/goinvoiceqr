//go:build !unix && !windows

package invoiceqr

import (
	"errors"
	"os"
)

func openForceWriteFile(path string) (*os.File, error) {
	// Fallback targets lack a standard-library no-following open. Keep the
	// symlink refusal behavior, but Unix and Windows provide the hard guarantee.
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("already exists as symlink; refusing to overwrite")
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
}
