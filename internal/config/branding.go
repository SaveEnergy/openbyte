package config

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"strconv"
)

const (
	maxBrandLogoBytes  = 1 << 20
	maxBrandLogoWidth  = 4096
	maxBrandLogoHeight = 4096
	maxBrandLogoPixels = 16_000_000

	darkThemeSurface  = "#1a1a24"
	lightThemeSurface = "#f6f8fb"
)

type BrandTheme struct {
	Primary          string
	AccentSecondary  string
	AccentGlow       string
	OnBrand          string
	Secondary        string
	AmbientPrimary   string
	AmbientSecondary string
}

type BrandPalette struct {
	Dark         BrandTheme
	Light        BrandTheme
	HasPrimary   bool
	HasSecondary bool
}

type BrandLogo struct {
	Data        []byte
	ContentType string
}

type rgbColor struct {
	r uint8
	g uint8
	b uint8
}

func (c *Config) BrandPalette() (BrandPalette, bool) {
	palette, configured, err := c.parseBrandPalette()
	if err != nil {
		return BrandPalette{}, false
	}
	return palette, configured
}

func (c *Config) BrandLogo() BrandLogo {
	return BrandLogo{
		Data:        bytes.Clone(c.brandLogoData),
		ContentType: c.brandLogoContentType,
	}
}

func (c *Config) validateBranding() error {
	if err := c.normalizeBrandColors(); err != nil {
		return err
	}
	return c.loadBrandLogo()
}

func (c *Config) normalizeBrandColors() error {
	palette, _, err := c.parseBrandPalette()
	if err != nil {
		return err
	}
	if palette.HasPrimary {
		c.BrandPrimaryColorDark = palette.Dark.Primary
		c.BrandPrimaryColorLight = palette.Light.Primary
	}
	if palette.HasSecondary {
		c.BrandSecondaryColorDark = palette.Dark.Secondary
		c.BrandSecondaryColorLight = palette.Light.Secondary
	}
	return nil
}

func (c *Config) parseBrandPalette() (BrandPalette, bool, error) {
	primarySet, err := validateBrandPair(
		"BRAND_PRIMARY_COLOR", c.BrandPrimaryColorDark, c.BrandPrimaryColorLight,
	)
	if err != nil {
		return BrandPalette{}, false, err
	}
	secondarySet, err := validateBrandPair(
		"BRAND_SECONDARY_COLOR", c.BrandSecondaryColorDark, c.BrandSecondaryColorLight,
	)
	if err != nil {
		return BrandPalette{}, false, err
	}

	palette := BrandPalette{HasPrimary: primarySet, HasSecondary: secondarySet}
	if primarySet {
		dark, err := parseAndCheckBrandColor(
			"BRAND_PRIMARY_COLOR_DARK", c.BrandPrimaryColorDark, darkThemeSurface, 4.5,
		)
		if err != nil {
			return BrandPalette{}, false, err
		}
		light, err := parseAndCheckBrandColor(
			"BRAND_PRIMARY_COLOR_LIGHT", c.BrandPrimaryColorLight, lightThemeSurface, 4.5,
		)
		if err != nil {
			return BrandPalette{}, false, err
		}
		palette.Dark = brandThemeFromPrimary(dark, rgbColor{255, 255, 255}, 0.30)
		palette.Light = brandThemeFromPrimary(light, rgbColor{}, 0.22)
	}
	if secondarySet {
		dark, err := parseAndCheckBrandColor(
			"BRAND_SECONDARY_COLOR_DARK", c.BrandSecondaryColorDark, darkThemeSurface, 3.0,
		)
		if err != nil {
			return BrandPalette{}, false, err
		}
		light, err := parseAndCheckBrandColor(
			"BRAND_SECONDARY_COLOR_LIGHT", c.BrandSecondaryColorLight, lightThemeSurface, 3.0,
		)
		if err != nil {
			return BrandPalette{}, false, err
		}
		palette.Dark.Secondary = dark.hex()
		palette.Dark.AmbientSecondary = dark.rgba(0.05)
		palette.Light.Secondary = light.hex()
		palette.Light.AmbientSecondary = light.rgba(0.05)
	}
	return palette, primarySet || secondarySet, nil
}

func validateBrandPair(name, dark, light string) (bool, error) {
	if (dark == "") != (light == "") {
		return false, fmt.Errorf("%s_DARK and %s_LIGHT must both be set or both be empty", name, name)
	}
	return dark != "", nil
}

func parseAndCheckBrandColor(name, value, surface string, minimum float64) (rgbColor, error) {
	color, err := parseHexColor(value)
	if err != nil {
		return rgbColor{}, fmt.Errorf("invalid %s %q: must be #RRGGBB", name, value)
	}
	background, _ := parseHexColor(surface)
	ratio := contrastRatio(color, background)
	if ratio < minimum {
		return rgbColor{}, fmt.Errorf(
			"invalid %s %q: contrast against %s is %.2f:1, must be >= %.1f:1",
			name, value, surface, ratio, minimum,
		)
	}
	return color, nil
}

func parseHexColor(value string) (rgbColor, error) {
	if len(value) != 7 || value[0] != '#' {
		return rgbColor{}, fmt.Errorf("invalid hex color")
	}
	parsed, err := strconv.ParseUint(value[1:], 16, 24)
	if err != nil {
		return rgbColor{}, err
	}
	return rgbColor{
		r: uint8(parsed >> 16),
		g: uint8(parsed >> 8),
		b: uint8(parsed),
	}, nil
}

func brandThemeFromPrimary(primary, hoverTarget rgbColor, glowAlpha float64) BrandTheme {
	return BrandTheme{
		Primary:         primary.hex(),
		AccentSecondary: mixColor(primary, hoverTarget, 0.12).hex(),
		AccentGlow:      primary.rgba(glowAlpha),
		OnBrand:         highestContrastForeground(primary),
		AmbientPrimary:  primary.rgba(0.08),
	}
}

func mixColor(base, target rgbColor, targetWeight float64) rgbColor {
	baseWeight := 1 - targetWeight
	return rgbColor{
		r: uint8(math.Round(float64(base.r)*baseWeight + float64(target.r)*targetWeight)),
		g: uint8(math.Round(float64(base.g)*baseWeight + float64(target.g)*targetWeight)),
		b: uint8(math.Round(float64(base.b)*baseWeight + float64(target.b)*targetWeight)),
	}
}

func highestContrastForeground(background rgbColor) string {
	black := rgbColor{}
	white := rgbColor{255, 255, 255}
	if contrastRatio(background, black) >= contrastRatio(background, white) {
		return black.hex()
	}
	return white.hex()
}

func contrastRatio(a, b rgbColor) float64 {
	lighter := max(relativeLuminance(a), relativeLuminance(b))
	darker := min(relativeLuminance(a), relativeLuminance(b))
	return (lighter + 0.05) / (darker + 0.05)
}

func relativeLuminance(color rgbColor) float64 {
	channel := func(value uint8) float64 {
		normalized := float64(value) / 255
		if normalized <= 0.04045 {
			return normalized / 12.92
		}
		return math.Pow((normalized+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(color.r) + 0.7152*channel(color.g) + 0.0722*channel(color.b)
}

func (color rgbColor) hex() string {
	return fmt.Sprintf("#%02x%02x%02x", color.r, color.g, color.b)
}

func (color rgbColor) rgba(alpha float64) string {
	return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", color.r, color.g, color.b, alpha)
}

func (c *Config) loadBrandLogo() error {
	c.brandLogoData = nil
	c.brandLogoContentType = ""
	if c.BrandLogoPath == "" {
		return nil
	}

	pathInfo, err := os.Stat(c.BrandLogoPath)
	if err != nil {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: %w", c.BrandLogoPath, err)
	}
	if err := validateBrandLogoFileInfo(pathInfo); err != nil {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: %w", c.BrandLogoPath, err)
	}
	file, err := os.Open(c.BrandLogoPath)
	if err != nil {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: %w", c.BrandLogoPath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: stat: %w", c.BrandLogoPath, err)
	}
	if err := validateBrandLogoFileInfo(info); err != nil {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: %w", c.BrandLogoPath, err)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBrandLogoBytes+1))
	if err != nil {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: read: %w", c.BrandLogoPath, err)
	}
	if len(data) == 0 || len(data) > maxBrandLogoBytes {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: size must be 1-%d bytes", c.BrandLogoPath, maxBrandLogoBytes)
	}

	decoded, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: must be a PNG or JPEG image", c.BrandLogoPath)
	}
	if decoded.Width <= 0 || decoded.Height <= 0 ||
		decoded.Width > maxBrandLogoWidth || decoded.Height > maxBrandLogoHeight ||
		int64(decoded.Width)*int64(decoded.Height) > maxBrandLogoPixels {
		return fmt.Errorf(
			"invalid BRAND_LOGO_PATH %q: dimensions must be <= %dx%d and <= %d pixels",
			c.BrandLogoPath, maxBrandLogoWidth, maxBrandLogoHeight, maxBrandLogoPixels,
		)
	}
	if _, decodedFormat, decodeErr := image.Decode(bytes.NewReader(data)); decodeErr != nil || decodedFormat != format {
		return fmt.Errorf("invalid BRAND_LOGO_PATH %q: image data is malformed", c.BrandLogoPath)
	}

	c.brandLogoData = bytes.Clone(data)
	if format == "png" {
		c.brandLogoContentType = "image/png"
	} else {
		c.brandLogoContentType = "image/jpeg"
	}
	return nil
}

func validateBrandLogoFileInfo(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("must be a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxBrandLogoBytes {
		return fmt.Errorf("size must be 1-%d bytes", maxBrandLogoBytes)
	}
	return nil
}
