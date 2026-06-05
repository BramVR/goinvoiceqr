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

type QROutputPreflight struct {
	Path          string
	Format        QRFormat
	Force         bool
	Exists        bool
	IsSymlink     bool
	WillOverwrite bool
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
	return writeQRArtifact(payload, options, RenderQRCode, writeFile, pathStatus)
}

func WritePlannedQRArtifact(payload string, output QROutputPreflight) error {
	return writePlannedQRArtifact(payload, output, RenderQRCode, writeFile)
}

func PreflightQROutput(options QROutputOptions) (QROutputPreflight, error) {
	return preflightQROutput(options, pathStatus)
}

type qrRenderFunc func(string, QRFormat) ([]byte, error)
type qrWriteFunc func(string, []byte, bool) error
type qrPathStatus struct {
	Exists    bool
	IsSymlink bool
	IsDir     bool
}
type qrPathStatusFunc func(string) (qrPathStatus, error)

func preflightQROutput(options QROutputOptions, stat qrPathStatusFunc) (QROutputPreflight, error) {
	out := strings.TrimSpace(options.Out)
	if out == "" {
		return QROutputPreflight{}, errors.New("out: required")
	}
	format, err := inferQRFormat(out, options.Format)
	if err != nil {
		return QROutputPreflight{}, err
	}

	if err := ensureQROutputParent(out); err != nil {
		return QROutputPreflight{}, err
	}

	status, err := stat(out)
	if err != nil {
		return QROutputPreflight{}, fmt.Errorf("out: %w", err)
	}
	if status.IsDir {
		return QROutputPreflight{}, errors.New("out: already exists as directory; refusing to overwrite")
	}
	if status.Exists && !options.Force {
		return QROutputPreflight{}, errors.New("out: already exists; use --force to overwrite")
	}
	if status.IsSymlink {
		return QROutputPreflight{}, errors.New("out: already exists as symlink; refusing to overwrite")
	}

	return QROutputPreflight{
		Path:          out,
		Format:        format,
		Force:         options.Force,
		Exists:        status.Exists,
		IsSymlink:     status.IsSymlink,
		WillOverwrite: status.Exists && options.Force,
	}, nil
}

func ensureQROutputParent(out string) error {
	parent := filepath.Dir(out)
	if parent == "." || parent == "" {
		return nil
	}
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("out: parent directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("out: parent directory is not a directory")
	}
	return nil
}

func writeQRArtifact(payload string, options QROutputOptions, render qrRenderFunc, write qrWriteFunc, stat qrPathStatusFunc) error {
	preflight, err := preflightQROutput(options, stat)
	if err != nil {
		return err
	}
	return writePlannedQRArtifact(payload, preflight, render, write)
}

func writePlannedQRArtifact(payload string, output QROutputPreflight, render qrRenderFunc, write qrWriteFunc) error {
	data, err := render(payload, output.Format)
	if err != nil {
		return err
	}
	if err := write(output.Path, data, output.Force); err != nil {
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
	var (
		file *os.File
		err  error
	)
	if force {
		file, err = openForceWriteFile(path)
	} else {
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	}
	if err != nil {
		return err
	}
	return writeAllAndClose(file, data)
}

func writeAllAndClose(file *os.File, data []byte) error {
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

func pathStatus(path string) (qrPathStatus, error) {
	info, err := os.Lstat(path)
	if err == nil {
		return qrPathStatus{
			Exists:    true,
			IsSymlink: info.Mode()&os.ModeSymlink != 0,
			IsDir:     info.IsDir(),
		}, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return qrPathStatus{}, nil
	}
	return qrPathStatus{}, err
}
