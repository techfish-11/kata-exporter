package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Token             string        `json:"token"`
	Secret            string        `json:"secret"`
	DeviceIDs         []string      `json:"device_ids,omitempty"`
	Listen            string        `json:"listen"`
	APIBaseURL        string        `json:"api_base_url,omitempty"`
	HTTPTimeout       time.Duration `json:"-"`
	HTTPTimeoutText   string        `json:"http_timeout,omitempty"`
	ScrapeCache       time.Duration `json:"-"`
	ScrapeCacheText   string        `json:"scrape_cache,omitempty"`
	DiaryEnabled      bool          `json:"diary_enabled"`
	DiaryWindow       time.Duration `json:"-"`
	DiaryWindowText   string        `json:"diary_window,omitempty"`
	DiaryRefresh      time.Duration `json:"-"`
	DiaryRefreshText  string        `json:"diary_refresh,omitempty"`
	DiscoveryRefresh  time.Duration `json:"-"`
	DiscoveryText     string        `json:"discovery_refresh,omitempty"`
	MaxConcurrency    int           `json:"max_concurrency,omitempty"`
}

func Default() Config {
	return Config{
		Listen: ":9788", APIBaseURL: "https://api.switch-bot.com",
		HTTPTimeout: 12*time.Second, HTTPTimeoutText: "12s",
		ScrapeCache: 10*time.Second, ScrapeCacheText: "10s",
		DiaryEnabled: true, DiaryWindow: 24*time.Hour, DiaryWindowText: "24h",
		DiaryRefresh: 15*time.Minute, DiaryRefreshText: "15m",
		DiscoveryRefresh: 30*time.Minute, DiscoveryText: "30m",
		MaxConcurrency: 4,
	}
}

func DefaultPath() string {
	if p := os.Getenv("KATA_CONFIG"); p != "" { return p }
	if runtime.GOOS == "windows" { return "kata-exporter.json" }
	return "/etc/kata-exporter/config.json"
}

func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) { return c, fmt.Errorf("read config: %w", err) }
		if err == nil && len(b) > 0 {
			if err := json.Unmarshal(b, &c); err != nil { return c, fmt.Errorf("parse config: %w", err) }
		}
	}
	applyEnv(&c)
	if err := c.normalize(); err != nil { return c, err }
	return c, nil
}

func (c Config) Marshal() ([]byte, error) {
	c.HTTPTimeoutText = c.HTTPTimeout.String()
	c.ScrapeCacheText = c.ScrapeCache.String()
	c.DiaryWindowText = c.DiaryWindow.String()
	c.DiaryRefreshText = c.DiaryRefresh.String()
	c.DiscoveryText = c.DiscoveryRefresh.String()
	return json.MarshalIndent(c, "", "  ")
}

func (c *Config) normalize() error {
	if c.Listen == "" { c.Listen = ":9788" }
	if c.APIBaseURL == "" { c.APIBaseURL = "https://api.switch-bot.com" }
	var err error
	if c.HTTPTimeout, err = duration(c.HTTPTimeoutText, c.HTTPTimeout, 12*time.Second); err != nil { return fmt.Errorf("http_timeout: %w", err) }
	if c.ScrapeCache, err = duration(c.ScrapeCacheText, c.ScrapeCache, 10*time.Second); err != nil { return fmt.Errorf("scrape_cache: %w", err) }
	if c.DiaryWindow, err = duration(c.DiaryWindowText, c.DiaryWindow, 24*time.Hour); err != nil { return fmt.Errorf("diary_window: %w", err) }
	if c.DiaryWindow > 31*24*time.Hour { return errors.New("diary_window must not exceed SwitchBot's 31 day limit") }
	if c.DiaryRefresh, err = duration(c.DiaryRefreshText, c.DiaryRefresh, 15*time.Minute); err != nil { return fmt.Errorf("diary_refresh: %w", err) }
	if c.DiscoveryRefresh, err = duration(c.DiscoveryText, c.DiscoveryRefresh, 30*time.Minute); err != nil { return fmt.Errorf("discovery_refresh: %w", err) }
	if c.MaxConcurrency < 1 { c.MaxConcurrency = 4 }
	if strings.TrimSpace(c.Token) == "" { return errors.New("SwitchBot token is missing (config token or KATA_TOKEN)") }
	if strings.TrimSpace(c.Secret) == "" { return errors.New("SwitchBot secret is missing (config secret or KATA_SECRET)") }
	return nil
}

func duration(text string, current, fallback time.Duration) (time.Duration, error) {
	if text != "" { return time.ParseDuration(text) }
	if current > 0 { return current, nil }
	return fallback, nil
}

func applyEnv(c *Config) {
	if v := os.Getenv("KATA_TOKEN"); v != "" { c.Token = v }
	if v := os.Getenv("KATA_SECRET"); v != "" { c.Secret = v }
	if v := os.Getenv("KATA_DEVICE_IDS"); v != "" { c.DeviceIDs = split(v) }
	if v := os.Getenv("KATA_LISTEN"); v != "" { c.Listen = v }
	if v := os.Getenv("KATA_API_BASE_URL"); v != "" { c.APIBaseURL = v }
	if v := os.Getenv("KATA_HTTP_TIMEOUT"); v != "" { c.HTTPTimeoutText = v }
	if v := os.Getenv("KATA_SCRAPE_CACHE"); v != "" { c.ScrapeCacheText = v }
	if v := os.Getenv("KATA_DIARY_WINDOW"); v != "" { c.DiaryWindowText = v }
	if v := os.Getenv("KATA_DIARY_REFRESH"); v != "" { c.DiaryRefreshText = v }
	if v := os.Getenv("KATA_DISCOVERY_REFRESH"); v != "" { c.DiscoveryText = v }
	if v := os.Getenv("KATA_DIARY_ENABLED"); v != "" { if b, e := strconv.ParseBool(v); e == nil { c.DiaryEnabled = b } }
	if v := os.Getenv("KATA_MAX_CONCURRENCY"); v != "" { if n, e := strconv.Atoi(v); e == nil { c.MaxConcurrency = n } }
}

func split(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") { if v = strings.TrimSpace(v); v != "" { out = append(out, v) } }
	return out
}

