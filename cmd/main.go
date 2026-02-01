package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/vixa/cdn/internal/bot"
	"github.com/vixa/cdn/internal/cdn"
	"github.com/vixa/cdn/internal/config"
	"github.com/vixa/cdn/internal/storage"
)

func main() {
	cfg := loadConfig()

	if cfg.BotToken == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	cm := config.NewConfigManager()

	// Try to load domains config, create empty file if it doesn't exist
	if err := cm.LoadDomains(cfg.DomainsConfig); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Domains config not found, creating empty file: %s", cfg.DomainsConfig)
			if err := cm.SaveDomains(cfg.DomainsConfig); err != nil {
				log.Printf("Warning: Failed to create empty domains config: %v", err)
			}
		} else {
			log.Printf("Warning: Failed to load domains config: %v", err)
		}
	}

	// Try to load categories config, create empty file if it doesn't exist
	if err := cm.LoadCategories(cfg.CategoriesConfig); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Categories config not found, creating empty file: %s", cfg.CategoriesConfig)
			if err := cm.SaveCategories(cfg.CategoriesConfig); err != nil {
				log.Printf("Warning: Failed to create empty categories config: %v", err)
			}
		} else {
			log.Printf("Warning: Failed to load categories config: %v", err)
		}
	}

	stor, err := storage.NewStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	settingsManager, err := config.NewSettingsManager(cfg.SettingsPath)
	if err != nil {
		log.Fatalf("Failed to initialize settings manager: %v", err)
	}

	defaultDomain := getDefaultDomain(cm)

	cdnServer := cdn.NewServer(stor, cm)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		log.Printf("[Main] Starting server on port %s", addr)
		if err := cdnServer.ListenAndServe(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] Server error: %v", err)
		}
	}()

	discordBot, err := bot.NewBot(cfg.BotToken, stor, cm, settingsManager, defaultDomain, cfg.DomainsConfig, cfg.CategoriesConfig)
	if err != nil {
		log.Fatalf("Failed to initialize Discord bot: %v", err)
	}

	if err := discordBot.Start(); err != nil {
		log.Fatalf("Failed to start Discord bot: %v", err)
	}

	fmt.Println("[Main] Server started successfully!")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n[Main] Shutting down...")

	discordBot.Stop()
	fmt.Println("[Main] Shutdown complete.")
}

type Config struct {
	BotToken         string
	StoragePath      string
	Port             int
	DomainsConfig    string
	CategoriesConfig string
	SettingsPath     string
}

func loadConfig() *Config {
	return &Config{
		BotToken:         getEnv("BOT_TOKEN", ""),
		StoragePath:      "/app/storage",
		Port:             getEnvInt("PORT", 8080),
		DomainsConfig:    "/app/configs/domains.json",
		CategoriesConfig: "/app/configs/categories.json",
		SettingsPath:     "/app/configs/settings.json",
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal := parseEnvInt(value); intVal != 0 {
			return intVal
		}
	}
	return defaultValue
}

func parseEnvInt(s string) int {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		return 0
	}
	return i
}

func getDefaultDomain(cm *config.ConfigManager) string {
	if domain := cm.GetFirstDomain(); domain != "" {
		return domain
	}
	return "cdn.example.com"
}
