package config

import "time"

// Config holds all application configuration.
type Config struct {
	ScrapeURL     string        `mapstructure:"scrape_url"`
	ImportFiles   []string      `mapstructure:"import_files"`
	Concurrency   int           `mapstructure:"concurrency"`
	Timeout       time.Duration `mapstructure:"timeout"`
	TargetURL     string        `mapstructure:"target_url"`
	OutputDir     string        `mapstructure:"output_dir"`
	OutputFormats []string      `mapstructure:"output_formats"`
	Interval      time.Duration `mapstructure:"interval"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		ScrapeURL:     "https://socks5-proxy.github.io/",
		Concurrency:   500,
		Timeout:       10 * time.Second,
		TargetURL:     "http://www.google.com/generate_204",
		OutputDir:     "./output",
		OutputFormats: []string{"table"},
		Interval:      30 * time.Minute,
	}
}
