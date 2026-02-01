package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type GlobalDefaults struct {
	Domain   string `json:"domain,omitempty"`
	Category string `json:"category,omitempty"`
}

type ChannelConfig struct {
	Domain   string `json:"domain"`
	Category string `json:"category"`
}

type Settings struct {
	GlobalDefaults GlobalDefaults           `json:"global_defaults,omitempty"`
	ChannelConfigs map[string]ChannelConfig `json:"channel_configs,omitempty"`
}

type SettingsManager struct {
	settingsPath string
	settings     Settings
	mu           sync.RWMutex
}

func NewSettingsManager(settingsPath string) (*SettingsManager, error) {
	sm := &SettingsManager{
		settingsPath: settingsPath,
		settings: Settings{
			ChannelConfigs: make(map[string]ChannelConfig),
		},
	}

	if err := sm.load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load settings: %w", err)
		}
		// File doesn't exist, create it with empty settings
		if err := sm.save(); err != nil {
			return nil, fmt.Errorf("failed to create initial settings file: %w", err)
		}
	}

	return sm, nil
}

func (sm *SettingsManager) load() error {
	data, err := os.ReadFile(sm.settingsPath)
	if err != nil {
		return err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	// Ensure ChannelConfigs is initialized
	if settings.ChannelConfigs == nil {
		settings.ChannelConfigs = make(map[string]ChannelConfig)
	}

	sm.mu.Lock()
	sm.settings = settings
	sm.mu.Unlock()

	return nil
}

func (sm *SettingsManager) save() error {
	sm.mu.RLock()
	data, err := json.MarshalIndent(sm.settings, "", "  ")
	sm.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Ensure directory exists
	dir := getDirectory(sm.settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	if err := os.WriteFile(sm.settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func getDirectory(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

func (sm *SettingsManager) SetGlobalDefaults(domain, category string) error {
	sm.mu.Lock()
	sm.settings.GlobalDefaults.Domain = domain
	sm.settings.GlobalDefaults.Category = category
	sm.mu.Unlock()

	return sm.save()
}

func (sm *SettingsManager) GetGlobalDefaults() (string, string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.GlobalDefaults.Domain, sm.settings.GlobalDefaults.Category
}

func (sm *SettingsManager) SetChannelConfig(channelID, domain, category string) error {
	sm.mu.Lock()
	sm.settings.ChannelConfigs[channelID] = ChannelConfig{
		Domain:   domain,
		Category: category,
	}
	sm.mu.Unlock()

	return sm.save()
}

func (sm *SettingsManager) GetChannelConfig(channelID string) (ChannelConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	config, ok := sm.settings.ChannelConfigs[channelID]
	return config, ok
}

func (sm *SettingsManager) RemoveChannelConfig(channelID string) error {
	sm.mu.Lock()
	delete(sm.settings.ChannelConfigs, channelID)
	sm.mu.Unlock()

	return sm.save()
}

func (sm *SettingsManager) ListChannelConfigs() map[string]ChannelConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]ChannelConfig)
	for k, v := range sm.settings.ChannelConfigs {
		result[k] = v
	}
	return result
}

func (sm *SettingsManager) HasGlobalDefaults() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.GlobalDefaults.Domain != "" && sm.settings.GlobalDefaults.Category != ""
}
