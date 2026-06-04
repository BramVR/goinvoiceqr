package invoiceqr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	goqr "github.com/piglig/go-qr"
)

const (
	QRFormatPNG QRFormat = "png"
	QRFormatSVG QRFormat = "svg"
)

type QRFormat string

type QROutputOptions struct {
	Out    string
	Format string
	Force  bool
}

const (
	qrScale            = 10
	qrQuietZoneModules = 4
)

func RenderQRCode(payload string, format QRFormat) ([]byte, error) {
	if format != QRFormatPNG && format != QRFormatSVG {
		return nil, fmt.Errorf("format: unsupported %q", format)
	}

	qr, err := encodeQRCode(payload)
	if err != nil {
		return nil, fmt.Errorf("qr render: %w", err)
	}
	config := qrCodeImgConfig(format)

	switch format {
	case QRFormatPNG:
		return qr.ToPNGBytes(config)
	case QRFormatSVG:
		return qr.ToSVGBytes(config)
	}
	return nil, fmt.Errorf("format: unsupported %q", format)
}

func encodeQRCode(payload string) (*goqr.QrCode, error) {
	segments, err := goqr.MakeSegments(payload)
	if err != nil {
		return nil, err
	}
	return goqr.EncodeSegments(segments, goqr.Medium, goqr.MinVersion, goqr.MaxVersion, -1, false)
}

func qrCodeImgConfig(format QRFormat) *goqr.QrCodeImgConfig {
	border := qrQuietZoneModules
	if format == QRFormatSVG {
		// go-qr renders SVG borders in output units, while PNG borders are modules.
		border *= qrScale
	}
	return goqr.NewQrCodeImgConfig(qrScale, border)
}

func WriteQRArtifact(payload string, options QROutputOptions) error {
	return writeQRArtifact(payload, options, RenderQRCode, writeFile, pathExists)
}

type qrRenderFunc func(string, QRFormat) ([]byte, error)
type qrWriteFunc func(string, []byte, bool) error
type qrExistsFunc func(string) (bool, error)

func writeQRArtifact(payload string, options QROutputOptions, render qrRenderFunc, write qrWriteFunc, exists qrExistsFunc) error {
	out := strings.TrimSpace(options.Out)
	if out == "" {
		return errors.New("out: required")
	}
	format, err := inferQRFormat(out, options.Format)
	if err != nil {
		return err
	}

	found, err := exists(out)
	if err != nil {
		return fmt.Errorf("out: %w", err)
	}
	if found && !options.Force {
		return errors.New("out: already exists; use --force to overwrite")
	}

	data, err := render(payload, format)
	if err != nil {
		return err
	}
	if err := write(out, data, options.Force); err != nil {
		return fmt.Errorf("out: %w", err)
	}
	return nil
}

func inferQRFormat(out, override string) (QRFormat, error) {
	if override != "" {
		return parseQRFormat(override)
	}
	switch strings.ToLower(filepath.Ext(out)) {
	case ".png":
		return QRFormatPNG, nil
	case ".svg":
		return QRFormatSVG, nil
	default:
		return "", errors.New("format: required for unknown output extension")
	}
}

func parseQRFormat(input string) (QRFormat, error) {
	format := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(input)), ".")
	switch format {
	case string(QRFormatPNG):
		return QRFormatPNG, nil
	case string(QRFormatSVG):
		return QRFormatSVG, nil
	default:
		return "", fmt.Errorf("format: unsupported %q", input)
	}
}

func writeFile(path string, data []byte, force bool) error {
	if force {
		info, err := os.Lstat(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("already exists as symlink; refusing to overwrite")
		}
		return os.WriteFile(path, data, 0o644)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}

	written, err := file.Write(data)
	if err != nil {
		_ = file.Close()
		return err
	}
	if written != len(data) {
		_ = file.Close()
		return io.ErrShortWrite
	}
	return file.Close()
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
