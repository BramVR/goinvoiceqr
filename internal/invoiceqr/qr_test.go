package invoiceqr

import (
	"bytes"
	"errors"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleEPCPayload = "BCD\n002\n1\nSCT\n\nACME BV\nBE68539007547034\nEUR42.50\n\n+++123/4567/89002+++\n\n"

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
		func(string, []byte) error {
			writeCalled = true
			return nil
		},
		func(string) (bool, error) { return false, nil },
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
		func(string, []byte) error { return writeErr },
		func(string) (bool, error) { return false, nil },
	)

	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error, got %v", err)
	}
}
