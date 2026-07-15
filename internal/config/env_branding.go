package config

import "os"

func (c *Config) loadBrandingEnv() error {
	setBrandingEnv("BRAND_PRIMARY_COLOR_DARK", &c.BrandPrimaryColorDark)
	setBrandingEnv("BRAND_PRIMARY_COLOR_LIGHT", &c.BrandPrimaryColorLight)
	setBrandingEnv("BRAND_SECONDARY_COLOR_DARK", &c.BrandSecondaryColorDark)
	setBrandingEnv("BRAND_SECONDARY_COLOR_LIGHT", &c.BrandSecondaryColorLight)
	setBrandingEnv("BRAND_LOGO_PATH", &c.BrandLogoPath)
	return c.normalizeBrandColors()
}

func setBrandingEnv(name string, target *string) {
	if value := os.Getenv(name); value != "" {
		*target = value
	}
}
