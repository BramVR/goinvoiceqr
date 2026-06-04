//go:build unix

package invoiceqr

import (
	"os"
	"syscall"
)

func openForceWriteFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
}
