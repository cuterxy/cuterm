package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/cuterxy/cuterm/internal/hub"
)

// appConfig is the persisted configuration, written to
// ~/.cuterm-hub/config.json whenever a setting changes via the config page.
// An explicit -addr flag overrides Port at startup.
type appConfig struct {
	Port       int        `json:"port,omitempty"`
	Language   string     `json:"language,omitempty"` // "en" / "zh-CN"; empty follows the system
	FontFamily string     `json:"fontFamily,omitempty"`
	FontSize   int        `json:"fontSize,omitempty"`
	Theme      string     `json:"theme,omitempty"`
	Scrollback int        `json:"scrollback,omitempty"`
	Nodes      []hub.Node `json:"nodes,omitempty"`
}

// configPath returns the config file location (~/.cuterm-hub/config.json),
// falling back to the system temp dir when the home dir is unknown.
func configPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cuterm-hub", "config.json")
	}
	return filepath.Join(os.TempDir(), "cuterm-hub-config.json")
}

// loadConfig reads the persisted config. A missing or malformed file
// yields the zero config (all defaults).
func loadConfig() appConfig {
	var cfg appConfig
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("ignoring malformed config %s: %v", configPath(), err)
		return appConfig{}
	}
	return cfg
}

// save writes the config atomically (temp file + rename). Failures are
// logged, not fatal: the runtime change has already been applied.
func (cfg appConfig) save() {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("save config: %v", err)
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("save config: %v", err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("save config: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("save config: %v", err)
	}
}
