package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type Domain struct {
	FolderName  string `json:"folder-name"`
	DisplayName string `json:"display-name"`
	DomainFQDN  string `json:"domain-fqdn"`
}

type Category struct {
	FolderName  string `json:"folder-name"`
	DisplayName string `json:"display-name"`
}

type Config struct {
	BotToken         string `env:"BOT_TOKEN" envDefault:""`
	StoragePath      string `env:"STORAGE_PATH" envDefault:"./storage"`
	Port             int    `env:"PORT" envDefault:"8080"`
	DomainsConfig    string `env:"DOMAINS_CONFIG" envDefault:"./configs/domains.json"`
	CategoriesConfig string `env:"CATEGORIES_CONFIG" envDefault:"./configs/categories.json"`
}

type ConfigManager struct {
	domains              map[string]string // folder-name -> exists
	domainDisplayNames   map[string]string // folder-name -> display-name
	domainFQDNs          map[string]string // folder-name -> domain-fqdn
	categories           map[string]string // folder-name -> exists
	categoryDisplayNames map[string]string // folder-name -> display-name
	mu                   sync.RWMutex
}

func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		domains:              make(map[string]string),
		domainDisplayNames:   make(map[string]string),
		domainFQDNs:          make(map[string]string),
		categories:           make(map[string]string),
		categoryDisplayNames: make(map[string]string),
	}
}

func (cm *ConfigManager) LoadDomains(configPath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read domains config: %w", err)
	}

	var domains []Domain
	if err := json.Unmarshal(data, &domains); err != nil {
		return fmt.Errorf("failed to parse domains config: %w", err)
	}

	cm.domains = make(map[string]string)
	cm.domainDisplayNames = make(map[string]string)
	cm.domainFQDNs = make(map[string]string)
	for _, d := range domains {
		// Normalize folder name: replace spaces with dashes, keep casing
		normalizedFolderName := strings.ReplaceAll(d.FolderName, " ", "-")
		cm.domains[normalizedFolderName] = "exists"
		cm.domainDisplayNames[normalizedFolderName] = d.DisplayName
		cm.domainFQDNs[normalizedFolderName] = d.DomainFQDN
	}

	return nil
}

func (cm *ConfigManager) LoadCategories(configPath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read categories config: %w", err)
	}

	var categories []Category
	if err := json.Unmarshal(data, &categories); err != nil {
		return fmt.Errorf("failed to parse categories config: %w", err)
	}

	cm.categories = make(map[string]string)
	cm.categoryDisplayNames = make(map[string]string)
	for _, c := range categories {
		// Normalize folder name: replace spaces with dashes, keep casing
		normalizedFolderName := strings.ReplaceAll(c.FolderName, " ", "-")
		cm.categories[normalizedFolderName] = "exists"
		cm.categoryDisplayNames[normalizedFolderName] = c.DisplayName
	}

	return nil
}

func (cm *ConfigManager) GetCategoryID(name string) (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.categories[name]
	return name, ok
}

func (cm *ConfigManager) ListCategories() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	names := make([]string, 0, len(cm.categories))
	for name := range cm.categories {
		names = append(names, name)
	}
	return names
}

func (cm *ConfigManager) ListDomains() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	domains := make([]string, 0, len(cm.domains))
	for domain := range cm.domains {
		domains = append(domains, domain)
	}
	return domains
}

func (cm *ConfigManager) GetFirstDomain() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for domain := range cm.domains {
		return domain
	}
	return ""
}

func (cm *ConfigManager) DomainExists(folderName string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.domains[folderName]
	return ok
}

func (cm *ConfigManager) GetDomainName(folderName string) (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	displayName, ok := cm.domainDisplayNames[folderName]
	if !ok {
		return folderName, false
	}
	return displayName, true
}

func (cm *ConfigManager) GetDomainFQDN(folderName string) (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	url, ok := cm.domainFQDNs[folderName]
	if !ok {
		return "", false
	}
	return url, true
}

func (cm *ConfigManager) GetDomainByFQDN(fqdn string) (folderName string, displayName string, ok bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for folder, domainFQDN := range cm.domainFQDNs {
		if domainFQDN == fqdn {
			display := cm.domainDisplayNames[folder]
			return folder, display, true
		}
	}
	return "", "", false
}

func (cm *ConfigManager) GetCategoryDisplayName(folderName string) (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	displayName, ok := cm.categoryDisplayNames[folderName]
	if !ok {
		return folderName, false
	}
	return displayName, true
}

func (cm *ConfigManager) HasDomains() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.domains) > 0
}

func (cm *ConfigManager) HasCategories() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.categories) > 0
}

func stripProtocol(fqdn string) string {
	fqdn = strings.TrimPrefix(fqdn, "https://")
	fqdn = strings.TrimPrefix(fqdn, "http://")
	fqdn = strings.TrimPrefix(fqdn, "ftp://")
	fqdn = strings.TrimSuffix(fqdn, "/")
	return fqdn
}

func (cm *ConfigManager) AddDomain(folderName, displayName, domainFQDN string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	normalizedFolderName := strings.ReplaceAll(folderName, " ", "-")
	cleanFQDN := stripProtocol(domainFQDN)

	if _, exists := cm.domains[normalizedFolderName]; exists {
		return fmt.Errorf("domain with folder-name '%s' already exists", normalizedFolderName)
	}

	cm.domains[normalizedFolderName] = "exists"
	cm.domainDisplayNames[normalizedFolderName] = displayName
	cm.domainFQDNs[normalizedFolderName] = cleanFQDN

	return nil
}

func (cm *ConfigManager) RemoveDomain(folderName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.domains[folderName]; !exists {
		return fmt.Errorf("domain '%s' not found", folderName)
	}

	delete(cm.domains, folderName)
	delete(cm.domainDisplayNames, folderName)
	delete(cm.domainFQDNs, folderName)

	return nil
}

func (cm *ConfigManager) AddCategory(folderName, displayName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	normalizedFolderName := strings.ReplaceAll(folderName, " ", "-")

	if _, exists := cm.categories[normalizedFolderName]; exists {
		return fmt.Errorf("category with folder-name '%s' already exists", normalizedFolderName)
	}

	cm.categories[normalizedFolderName] = "exists"
	cm.categoryDisplayNames[normalizedFolderName] = displayName

	return nil
}

func (cm *ConfigManager) RemoveCategory(folderName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.categories[folderName]; !exists {
		return fmt.Errorf("category '%s' not found", folderName)
	}

	delete(cm.categories, folderName)
	delete(cm.categoryDisplayNames, folderName)

	return nil
}

func (cm *ConfigManager) SaveDomains(configPath string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	domains := make([]Domain, 0, len(cm.domains))
	for folderName := range cm.domains {
		domains = append(domains, Domain{
			FolderName:  folderName,
			DisplayName: cm.domainDisplayNames[folderName],
			DomainFQDN:  cm.domainFQDNs[folderName],
		})
	}

	data, err := json.MarshalIndent(domains, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal domains: %w", err)
	}

	dir := getDirectory(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create domains directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write domains file: %w", err)
	}

	return nil
}

func (cm *ConfigManager) SaveCategories(configPath string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	categories := make([]Category, 0, len(cm.categories))
	for folderName := range cm.categories {
		categories = append(categories, Category{
			FolderName:  folderName,
			DisplayName: cm.categoryDisplayNames[folderName],
		})
	}

	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal categories: %w", err)
	}

	dir := getDirectory(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create categories directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write categories file: %w", err)
	}

	return nil
}
