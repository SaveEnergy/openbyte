package config_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

const (
	validPrimaryDark    = "#00D4AA"
	validPrimaryLight   = "#00796B"
	validSecondaryDark  = "#667EEA"
	validSecondaryLight = "#667EEA"
)

func TestBrandingColorsLoadNormalizeAndDerive(t *testing.T) {
	setValidBrandColorEnv(t)
	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if cfg.BrandPrimaryColorDark != "#00d4aa" || cfg.BrandPrimaryColorLight != "#00796b" {
		t.Fatalf("primary colors were not normalized: %#v", cfg)
	}
	palette, ok := cfg.BrandPalette()
	if !ok || !palette.HasPrimary || !palette.HasSecondary {
		t.Fatalf("palette = %#v, true; want both color pairs", palette)
	}
	if palette.Dark.OnBrand != "#000000" || palette.Light.OnBrand != "#ffffff" {
		t.Fatalf("on-brand colors = %q/%q, want black/white", palette.Dark.OnBrand, palette.Light.OnBrand)
	}
	if palette.Dark.AccentSecondary == palette.Dark.Primary || palette.Dark.AccentGlow == "" {
		t.Fatalf("derived dark accent values missing: %#v", palette.Dark)
	}
}

func TestBrandingColorPairsMustBeComplete(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		dark   string
		light  string
	}{
		{name: "primary dark only", prefix: "PRIMARY", dark: validPrimaryDark},
		{name: "primary light only", prefix: "PRIMARY", light: validPrimaryLight},
		{name: "secondary dark only", prefix: "SECONDARY", dark: validSecondaryDark},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BRAND_"+tt.prefix+"_COLOR_DARK", tt.dark)
			t.Setenv("BRAND_"+tt.prefix+"_COLOR_LIGHT", tt.light)
			cfg := config.DefaultConfig()
			if err := cfg.LoadFromEnv(); err == nil || !strings.Contains(err.Error(), "must both be set") {
				t.Fatalf("LoadFromEnv error = %v, want incomplete-pair error", err)
			}
		})
	}
}

func TestBrandingColorsRejectInvalidSyntaxAndContrast(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "short hex", value: "#0da", want: "must be #RRGGBB"},
		{name: "named", value: "teal", want: "must be #RRGGBB"},
		{name: "css injection", value: "#00d4aa;}", want: "must be #RRGGBB"},
		{name: "low contrast", value: "#111111", want: "contrast"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BRAND_PRIMARY_COLOR_DARK", tt.value)
			t.Setenv("BRAND_PRIMARY_COLOR_LIGHT", validPrimaryLight)
			cfg := config.DefaultConfig()
			if err := cfg.LoadFromEnv(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadFromEnv error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBrandingSecondaryColorRequiresThreeToOneContrast(t *testing.T) {
	t.Setenv("BRAND_SECONDARY_COLOR_DARK", "#1a1a24")
	t.Setenv("BRAND_SECONDARY_COLOR_LIGHT", validSecondaryLight)
	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err == nil || !strings.Contains(err.Error(), "must be >= 3.0:1") {
		t.Fatalf("LoadFromEnv error = %v, want secondary contrast error", err)
	}
}

func TestBrandLogoLoadsPNGAndJPEGIntoMemory(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		encode      func(*bytes.Buffer) error
	}{
		{
			name: "png", contentType: "image/png",
			encode: func(dst *bytes.Buffer) error {
				return png.Encode(dst, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
			},
		},
		{
			name: "jpeg", contentType: "image/jpeg",
			encode: func(dst *bytes.Buffer) error {
				img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
				for y := range 2 {
					for x := range 2 {
						img.Set(x, y, color.White)
					}
				}
				return jpeg.Encode(dst, img, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data bytes.Buffer
			if err := tt.encode(&data); err != nil {
				t.Fatalf("encode: %v", err)
			}
			path := filepath.Join(t.TempDir(), "logo."+tt.name)
			if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
				t.Fatalf("write logo: %v", err)
			}
			t.Setenv("BRAND_LOGO_PATH", path)
			cfg := config.DefaultConfig()
			if err := cfg.LoadFromEnv(); err != nil {
				t.Fatalf("LoadFromEnv: %v", err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			logo := cfg.BrandLogo()
			if logo.ContentType != tt.contentType || !bytes.Equal(logo.Data, data.Bytes()) {
				t.Fatalf("logo type/data = %q/%d bytes", logo.ContentType, len(logo.Data))
			}
			logo.Data[0] ^= 0xff
			if bytes.Equal(logo.Data, cfg.BrandLogo().Data) {
				t.Fatal("BrandLogo returned mutable config storage")
			}
		})
	}
}

func TestBrandLogoFollowsSymlinkToRegularImage(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "logo-v2.png")
	if err := os.WriteFile(target, data.Bytes(), 0o600); err != nil {
		t.Fatalf("write logo: %v", err)
	}
	path := filepath.Join(dir, "logo.png")
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink logo: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.BrandLogoPath = path
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate symlinked logo: %v", err)
	}
	if logo := cfg.BrandLogo(); logo.ContentType != "image/png" || !bytes.Equal(logo.Data, data.Bytes()) {
		t.Fatalf("logo type/data = %q/%d bytes", logo.ContentType, len(logo.Data))
	}
}

func TestBrandLogoRejectsUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	oversized := filepath.Join(dir, "oversized.png")
	if err := os.WriteFile(oversized, make([]byte, (1<<20)+1), 0o600); err != nil {
		t.Fatalf("write oversized logo: %v", err)
	}
	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty logo: %v", err)
	}
	text := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(text, []byte("not an image"), 0o600); err != nil {
		t.Fatalf("write text logo: %v", err)
	}
	svg := filepath.Join(dir, "logo.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), 0o600); err != nil {
		t.Fatalf("write SVG logo: %v", err)
	}
	truncated := filepath.Join(dir, "truncated.png")
	if err := os.WriteFile(truncated, pngHeader(2, 2), 0o600); err != nil {
		t.Fatalf("write truncated PNG logo: %v", err)
	}

	for _, path := range []string{
		dir, empty, oversized, text, svg, truncated, filepath.Join(dir, "missing.png"),
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.BrandLogoPath = path
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "BRAND_LOGO_PATH") {
				t.Fatalf("Validate error = %v, want BRAND_LOGO_PATH error", err)
			}
		})
	}
}

func TestBrandLogoRejectsExcessiveDimensionsAndPixels(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
	}{
		{name: "width", width: 4097, height: 1},
		{name: "pixel count", width: 4001, height: 4000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "logo.png")
			if err := os.WriteFile(path, pngHeader(tt.width, tt.height), 0o600); err != nil {
				t.Fatalf("write logo: %v", err)
			}
			cfg := config.DefaultConfig()
			cfg.BrandLogoPath = path
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "dimensions") {
				t.Fatalf("Validate error = %v, want dimensions error", err)
			}
		})
	}
}

func setValidBrandColorEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BRAND_PRIMARY_COLOR_DARK", validPrimaryDark)
	t.Setenv("BRAND_PRIMARY_COLOR_LIGHT", validPrimaryLight)
	t.Setenv("BRAND_SECONDARY_COLOR_DARK", validSecondaryDark)
	t.Setenv("BRAND_SECONDARY_COLOR_LIGHT", validSecondaryLight)
}

func pngHeader(width, height uint32) []byte {
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], width)
	binary.BigEndian.PutUint32(data[4:8], height)
	data[8] = 8
	data[9] = 6

	var encoded bytes.Buffer
	encoded.Write([]byte("\x89PNG\r\n\x1a\n"))
	_ = binary.Write(&encoded, binary.BigEndian, uint32(len(data)))
	encoded.WriteString("IHDR")
	encoded.Write(data)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte("IHDR"))
	_, _ = checksum.Write(data)
	_ = binary.Write(&encoded, binary.BigEndian, checksum.Sum32())
	return encoded.Bytes()
}
