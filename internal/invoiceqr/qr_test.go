package invoiceqr

import (
	"bytes"
	"errors"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	goqr "github.com/piglig/go-qr"
)

const sampleEPCPayload = "BCD\n002\n1\nSCT\n\nACME BV\nBE68539007547034\nEUR42.50\n\n+++123/4567/89002+++\n\n"
const shortEPCPayload = "BCD\n002\n1\nSCT\n\nA\nBE68539007547034\nEUR1.00\n\nX\n\n"

func TestRenderQRCodePNG(t *testing.T) {
	data, err := RenderQRCode(sampleEPCPayload, QRFormatPNG)
	if err != nil {
		t.Fatalf("expected png, got %v", err)
	}
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG signature, got %q", data[:8])
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected decodable PNG, got %v", err)
	}
	if format != "png" {
		t.Fatalf("expected png format, got %q", format)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		t.Fatalf("expected non-empty image bounds, got %v", img.Bounds())
	}
}

func TestRenderQRCodeSVG(t *testing.T) {
	data, err := RenderQRCode(sampleEPCPayload, QRFormatSVG)
	if err != nil {
		t.Fatalf("expected svg, got %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "<svg") || !strings.Contains(text, "</svg>") {
		t.Fatalf("expected SVG document, got %q", text[:min(len(text), 120)])
	}
	if !strings.Contains(text, `viewBox="0 0 450 450"`) {
		t.Fatalf("expected four-module quiet zone in viewBox, got %q", text[:min(len(text), 120)])
	}
	if !strings.Contains(text, `<path d="M40,40`) {
		t.Fatalf("expected SVG modules to start after four-module quiet zone, got %q", text[:min(len(text), 160)])
	}
}

func TestRenderQRCodeUsesMediumErrorCorrection(t *testing.T) {
	data, err := RenderQRCode(shortEPCPayload, QRFormatPNG)
	if err != nil {
		t.Fatalf("expected png, got %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected decodable PNG, got %v", err)
	}
	decoded, err := goqr.DecodeDetailed(img)
	if err != nil {
		t.Fatalf("expected decodable QR, got %v", err)
	}
	if decoded.Ecc != goqr.Medium {
		t.Fatalf("expected medium error correction, got %v", decoded.Ecc)
	}
}

func TestInferQRFormat(t *testing.T) {
	tests := []struct {
		name     string
		out      string
		override string
		want     QRFormat
	}{
		{name: "png extension", out: "invoice.png", want: QRFormatPNG},
		{name: "svg extension", out: "invoice.svg", want: QRFormatSVG},
		{name: "uppercase extension", out: "invoice.PNG", want: QRFormatPNG},
		{name: "override unusual name", out: "invoice.qr", override: "svg", want: QRFormatSVG},
		{name: "override extension", out: "invoice.png", override: "svg", want: QRFormatSVG},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inferQRFormat(tt.out, tt.override)
			if err != nil {
				t.Fatalf("expected format, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestInferQRFormatRejectsUnknownExtensionWithoutOverride(t *testing.T) {
	_, err := inferQRFormat("invoice.qr", "")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "format") {
		t.Fatalf("expected format error, got %v", err)
	}
}

func TestPreflightQROutputTrimsInfersAndReportsOverwrite(t *testing.T) {
	preflight, err := preflightQROutput(
		QROutputOptions{Out: " invoice.svg ", Force: true},
		func(path string) (qrPathStatus, error) {
			if path != "invoice.svg" {
				t.Fatalf("expected trimmed path, got %q", path)
			}
			return qrPathStatus{Exists: true}, nil
		},
	)

	if err != nil {
		t.Fatalf("expected preflight, got %v", err)
	}
	if preflight.Path != "invoice.svg" || preflight.Format != QRFormatSVG || !preflight.Force || !preflight.Exists || !preflight.WillOverwrite {
		t.Fatalf("unexpected preflight: %+v", preflight)
	}
}

func TestPreflightQROutputRefusesOverwriteBeforeRendering(t *testing.T) {
	renderCalled := false
	writeCalled := false

	err := writeQRArtifact(
		sampleEPCPayload,
		QROutputOptions{Out: "invoice.svg"},
		func(string, QRFormat) ([]byte, error) {
			renderCalled = true
			return []byte("<svg></svg>"), nil
		},
		func(string, []byte, bool) error {
			writeCalled = true
			return nil
		},
		func(string) (qrPathStatus, error) { return qrPathStatus{Exists: true}, nil },
	)

	if err == nil {
		t.Fatalf("expected overwrite error")
	}
	if renderCalled {
		t.Fatalf("expected no render after preflight failure")
	}
	if writeCalled {
		t.Fatalf("expected no write after preflight failure")
	}
}

func TestPreflightQROutputRefusesForceSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.svg")
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	symlinkOrSkip(t, target, out)

	_, err := PreflightQROutput(QROutputOptions{Out: out, Force: true})

	if err == nil {
		t.Fatalf("expected symlink preflight error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestWriteQRArtifactRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(out, []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed output: %v", err)
	}

	err := WriteQRArtifact(sampleEPCPayload, QROutputOptions{Out: out})
	if err == nil {
		t.Fatalf("expected overwrite error")
	}
	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if string(got) != "existing" {
		t.Fatalf("expected existing content to remain, got %q", got)
	}
}

func TestWriteQRArtifactRefusesDanglingSymlinkOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	symlinkOrSkip(t, filepath.Join(dir, "missing-target.svg"), out)

	err := WriteQRArtifact(sampleEPCPayload, QROutputOptions{Out: out})
	if err == nil {
		t.Fatalf("expected overwrite error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "missing-target.svg")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected dangling symlink target to remain absent, got %v", statErr)
	}
}

func TestWriteQRArtifactRefusesRaceOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	err := writeQRArtifact(
		sampleEPCPayload,
		QROutputOptions{Out: out},
		func(string, QRFormat) ([]byte, error) { return []byte("<svg>new</svg>"), nil },
		writeFile,
		func(string) (qrPathStatus, error) {
			if err := os.WriteFile(out, []byte("raced"), 0o644); err != nil {
				return qrPathStatus{}, err
			}
			return qrPathStatus{}, nil
		},
	)

	if err == nil {
		t.Fatalf("expected exclusive write error")
	}
	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if string(got) != "raced" {
		t.Fatalf("expected raced content to remain, got %q", got)
	}
}

func TestWriteQRArtifactAllowsForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(out, []byte("existing"), 0o644); err != nil {
		t.Fatalf("seed output: %v", err)
	}

	err := WriteQRArtifact(sampleEPCPayload, QROutputOptions{Out: out, Force: true})
	if err != nil {
		t.Fatalf("expected overwrite, got %v", err)
	}
	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if !strings.Contains(string(got), "<svg") {
		t.Fatalf("expected SVG output, got %q", got[:min(len(got), 120)])
	}
}

func TestWriteQRArtifactRefusesForceSymlinkOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.svg")
	out := filepath.Join(dir, "invoice.svg")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	symlinkOrSkip(t, target, out)

	err := WriteQRArtifact(sampleEPCPayload, QROutputOptions{Out: out, Force: true})
	if err == nil {
		t.Fatalf("expected symlink overwrite error")
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target: %v", readErr)
	}
	if string(got) != "target" {
		t.Fatalf("expected symlink target to remain unchanged, got %q", got)
	}
}

func TestWriteQRArtifactUsesFormatOverride(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "invoice.qr")

	err := WriteQRArtifact(sampleEPCPayload, QROutputOptions{Out: out, Format: "png"})
	if err != nil {
		t.Fatalf("expected write, got %v", err)
	}
	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if len(got) < 8 || string(got[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG output, got %q", got[:min(len(got), 8)])
	}
}

func TestWriteQRArtifactPropagatesRenderError(t *testing.T) {
	renderErr := errors.New("render failed")
	writeCalled := false
	err := writeQRArtifact(
		sampleEPCPayload,
		QROutputOptions{Out: "invoice.png"},
		func(string, QRFormat) ([]byte, error) { return nil, renderErr },
		func(string, []byte, bool) error {
			writeCalled = true
			return nil
		},
		func(string) (qrPathStatus, error) { return qrPathStatus{}, nil },
	)

	if !errors.Is(err, renderErr) {
		t.Fatalf("expected render error, got %v", err)
	}
	if writeCalled {
		t.Fatalf("expected no write after render error")
	}
}

func TestWriteQRArtifactPropagatesWriteError(t *testing.T) {
	writeErr := errors.New("write failed")
	err := writeQRArtifact(
		sampleEPCPayload,
		QROutputOptions{Out: "invoice.svg"},
		func(string, QRFormat) ([]byte, error) { return []byte("<svg></svg>"), nil },
		func(string, []byte, bool) error { return writeErr },
		func(string) (qrPathStatus, error) { return qrPathStatus{}, nil },
	)

	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error, got %v", err)
	}
}

func symlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()

	if err := os.Symlink(oldname, newname); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink setup requires Windows symlink privileges: %v", err)
		}
		t.Fatalf("seed symlink: %v", err)
	}
}
