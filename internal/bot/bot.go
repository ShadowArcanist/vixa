package bot

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/vixa/cdn/internal/config"
	"github.com/vixa/cdn/internal/storage"
)

type Bot struct {
	session          *discordgo.Session
	storage          *storage.Storage
	configManager    *config.ConfigManager
	settingsManager  *config.SettingsManager
	defaultDomain    string
	domainsConfig    string
	categoriesConfig string
	mu               sync.Mutex
	commands         map[string]bool
}

func NewBot(token string, stor *storage.Storage, cm *config.ConfigManager, settingsManager *config.SettingsManager, defaultDomain, domainsConfig, categoriesConfig string) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	return &Bot{
		session:          session,
		storage:          stor,
		configManager:    cm,
		settingsManager:  settingsManager,
		defaultDomain:    defaultDomain,
		domainsConfig:    domainsConfig,
		categoriesConfig: categoriesConfig,
		commands:         make(map[string]bool),
	}, nil
}

func (b *Bot) Start() error {
	b.session.AddHandler(b.onReady)
	b.session.AddHandler(b.onInteractionCreate)
	b.session.AddHandler(b.onMessageCreate)

	return b.session.Open()
}

func (b *Bot) Stop() error {
	return b.session.Close()
}

func (b *Bot) registerCommands(s *discordgo.Session) {
	uploadCmd := &discordgo.ApplicationCommand{
		Name:        "upload",
		Description: "Upload a file to the CDN (uses defaults if domain/category not specified)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "file",
				Description: "The file to upload (max 500MB)",
				Required:    true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "category",
				Description:  "Category for the file (optional if defaults set)",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "domain",
				Description:  "CDN domain (optional if defaults set)",
				Required:     false,
				Autocomplete: true,
			},
		},
	}

	deleteCmd := &discordgo.ApplicationCommand{
		Name:        "delete",
		Description: "Delete a file from the CDN",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "Full URL of the file to delete",
				Required:    true,
			},
		},
	}

	listCmd := &discordgo.ApplicationCommand{
		Name:        "list",
		Description: "List all files in a category",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "domain",
				Description:  "CDN domain",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "category",
				Description:  "Category name",
				Required:     true,
				Autocomplete: true,
			},
		},
	}

	defaultCmd := &discordgo.ApplicationCommand{
		Name:        "default",
		Description: "Set default domain and category for uploads",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "domain",
				Description:  "Default CDN domain",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "category",
				Description:  "Default category",
				Required:     true,
				Autocomplete: true,
			},
		},
	}

	setChannelCmd := &discordgo.ApplicationCommand{
		Name:        "set-channel",
		Description: "Set auto-upload configuration for this channel",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "domain",
				Description:  "CDN domain for this channel",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "category",
				Description:  "Category for this channel",
				Required:     true,
				Autocomplete: true,
			},
		},
	}

	viewChannelDefaultCmd := &discordgo.ApplicationCommand{
		Name:        "view-channel-default",
		Description: "View the auto-upload settings for this channel",
	}

	resetChannelCmd := &discordgo.ApplicationCommand{
		Name:        "reset-channel",
		Description: "Remove the auto-upload configuration for this channel",
	}

	addDomainCmd := &discordgo.ApplicationCommand{
		Name:        "add-domain",
		Description: "Add a new CDN domain",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "domain-fqdn",
				Description: "Domain FQDN (e.g., cdn.example.com). Protocol will be stripped automatically.",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "display-name",
				Description: "Display name for the domain",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "folder-name",
				Description: "Folder name for the domain (no spaces)",
				Required:    true,
			},
		},
	}

	removeDomainCmd := &discordgo.ApplicationCommand{
		Name:        "remove-domain",
		Description: "Remove a CDN domain",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "domain-name",
				Description:  "Domain to remove",
				Required:     true,
				Autocomplete: true,
			},
		},
	}

	addCategoryCmd := &discordgo.ApplicationCommand{
		Name:        "add-category",
		Description: "Add a new category",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "category-name",
				Description: "Display name for the category",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "folder-name",
				Description: "Folder name for the category (no spaces)",
				Required:    true,
			},
		},
	}

	removeCategoryCmd := &discordgo.ApplicationCommand{
		Name:        "remove-category",
		Description: "Remove a category",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "category-name",
				Description:  "Category to remove",
				Required:     true,
				Autocomplete: true,
			},
		},
	}

	commands := []*discordgo.ApplicationCommand{uploadCmd, deleteCmd, listCmd, defaultCmd, setChannelCmd, viewChannelDefaultCmd, resetChannelCmd, addDomainCmd, removeDomainCmd, addCategoryCmd, removeCategoryCmd}

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			fmt.Printf("[Discord] Failed to create command %s: %v\n", cmd.Name, err)
		} else {
			fmt.Printf("[Discord] Registered command: %s\n", cmd.Name)
		}
	}
}

func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	fmt.Printf("[Discord] Logged in as %s#%s\n", event.User.Username, event.User.Discriminator)

	s.UpdateCustomStatus("Online quietly")

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.commands["registered"] {
		return
	}

	b.registerCommands(s)
	b.commands["registered"] = true
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()
		switch data.Name {
		case "upload":
			b.handleUpload(s, i)
		case "delete":
			b.handleDelete(s, i)
		case "list":
			b.handleList(s, i)
		case "default":
			b.handleDefault(s, i)
		case "set-channel":
			b.handleSetChannel(s, i)
		case "view-channel-default":
			b.handleViewChannelDefault(s, i)
		case "reset-channel":
			b.handleResetChannel(s, i)
		case "add-domain":
			b.handleAddDomain(s, i)
		case "remove-domain":
			b.handleRemoveDomain(s, i)
		case "add-category":
			b.handleAddCategory(s, i)
		case "remove-category":
			b.handleRemoveCategory(s, i)
		}
	case discordgo.InteractionMessageComponent:
		b.handleComponentInteraction(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(s, i)
	}
}

func (b *Bot) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	// Get the focused option
	var focusedOption *discordgo.ApplicationCommandInteractionDataOption
	for _, opt := range data.Options {
		if opt.Focused {
			focusedOption = opt
			break
		}
		// Handle sub-options for commands that have them
		if len(opt.Options) > 0 {
			for _, subOpt := range opt.Options {
				if subOpt.Focused {
					focusedOption = subOpt
					break
				}
			}
		}
	}

	if focusedOption == nil {
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	userInput := strings.ToLower(focusedOption.StringValue())

	switch focusedOption.Name {
	case "domain", "domain-name":
		domains := b.configManager.ListDomains()
		for _, domain := range domains {
			displayName, _ := b.configManager.GetDomainName(domain)
			// Filter based on user input
			if userInput == "" ||
				strings.Contains(strings.ToLower(domain), userInput) ||
				strings.Contains(strings.ToLower(displayName), userInput) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  displayName,
					Value: domain,
				})
			}
		}
	case "category", "category-name":
		categories := b.configManager.ListCategories()
		for _, category := range categories {
			displayName, _ := b.configManager.GetCategoryDisplayName(category)
			// Filter based on user input
			if userInput == "" ||
				strings.Contains(strings.ToLower(category), userInput) ||
				strings.Contains(strings.ToLower(displayName), userInput) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  displayName,
					Value: category,
				})
			}
		}
	}

	// Discord allows max 25 choices
	if len(choices) > 25 {
		choices = choices[:25]
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

func (b *Bot) handleUpload(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Check if any domains exist
	if !b.configManager.HasDomains() {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No domains configured. Please add a domain using `/add-domain` command.",
		})
		return
	}

	// Check if any categories exist
	if !b.configManager.HasCategories() {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No categories configured. Please add a category using `/add-category` command.",
		})
		return
	}

	data := i.ApplicationCommandData()

	attachmentID := data.Options[0].Value.(string)

	// Get domain from options or use global default
	var domain string
	if len(data.Options) > 2 && data.Options[2] != nil {
		domain = data.Options[2].Value.(string)
	} else {
		// Try to get from global defaults
		defaultDomain, _ := b.settingsManager.GetGlobalDefaults()
		if defaultDomain != "" {
			domain = defaultDomain
		} else {
			domain = b.defaultDomain
		}
	}

	// Get category from options or use global default
	var categoryName string
	if len(data.Options) > 1 && data.Options[1] != nil {
		categoryName = data.Options[1].Value.(string)
	} else {
		// Try to get from global defaults
		_, defaultCategory := b.settingsManager.GetGlobalDefaults()
		if defaultCategory != "" {
			categoryName = defaultCategory
		}
	}

	// Validate we have both domain and category
	if domain == "" || categoryName == "" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Domain and category are required. Either provide them as arguments or set defaults using `/default` command.",
		})
		return
	}

	attachment := b.findAttachment(i, attachmentID)
	if attachment == nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Attachment not found",
		})
		return
	}

	_, ok := b.configManager.GetDomainName(domain)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid domain",
		})
		return
	}

	_, ok = b.configManager.GetCategoryID(categoryName)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid category",
		})
		return
	}

	fileData, contentType, err := storage.DownloadFile(attachment.URL)
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to download file: %v", err),
		})
		return
	}

	ext := filepath.Ext(attachment.Filename)
	filename, _, err := b.storage.StoreFile(domain, categoryName, fileData, contentType, ext)
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to store file: %v", err),
		})
		return
	}

	domainURL, _ := b.configManager.GetDomainFQDN(domain)
	encodedCategory := url.PathEscape(categoryName)
	fileURL := fmt.Sprintf("https://%s/%s/%s", domainURL, encodedCategory, filename)

	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("<%s>", fileURL),
	})
}

func (b *Bot) handleDelete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	data := i.ApplicationCommandData()
	url := data.Options[0].Value.(string)

	domainFQDN, category, filename, err := b.parseURL(url)
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Invalid URL: %v", err),
		})
		return
	}

	domainFolder, _, ok := b.configManager.GetDomainByFQDN(domainFQDN)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Domain '%s' not found in configuration.", domainFQDN),
		})
		return
	}

	err = b.storage.DeleteFile(domainFolder, category, filename)
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to delete file: %v", err),
		})
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("<%s> has been deleted.", url),
	})
}

const itemsPerPage = 15

func (b *Bot) handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	data := i.ApplicationCommandData()
	domain := data.Options[0].Value.(string)
	category := data.Options[1].Value.(string)

	_, ok := b.configManager.GetDomainName(domain)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid domain",
		})
		return
	}

	_, ok = b.configManager.GetCategoryID(category)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid category",
		})
		return
	}

	files, err := b.storage.ListFiles(domain, category)
	if err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to list files: %v", err),
		})
		return
	}

	domainDisplayName, _ := b.configManager.GetDomainName(domain)
	categoryDisplayName, _ := b.configManager.GetCategoryDisplayName(category)

	if len(files) == 0 {
		_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("No files found in `%s/%s`", domainDisplayName, categoryDisplayName),
		})
		return
	}

	b.sendListPage(s, i.Interaction, domain, category, files, 0)
}

func (b *Bot) sendListPage(s *discordgo.Session, i *discordgo.Interaction, domainFolder, category string, files []string, page int) {
	totalPages := (len(files) + itemsPerPage - 1) / itemsPerPage
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * itemsPerPage
	end := start + itemsPerPage
	if end > len(files) {
		end = len(files)
	}

	domainURL, _ := b.configManager.GetDomainFQDN(domainFolder)
	var sb strings.Builder

	encodedCategory := url.PathEscape(category)
	for _, file := range files[start:end] {
		encodedFile := url.PathEscape(file)
		fileURL := fmt.Sprintf("https://%s/%s/%s", domainURL, encodedCategory, encodedFile)
		sb.WriteString(fmt.Sprintf("- %s\n", fileURL))
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Files (%d total)", len(files)),
		Description: sb.String(),
		Color:       0x808080,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Page %d/%d", page+1, totalPages),
		},
	}

	var components []discordgo.MessageComponent
	if totalPages > 1 {
		prevDisabled := page == 0
		nextDisabled := page >= totalPages-1

		prevCustomID := fmt.Sprintf("list_prev:%s:%s:%d", domainFolder, category, page)
		nextCustomID := fmt.Sprintf("list_next:%s:%s:%d", domainFolder, category, page)

		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Previous",
						Style:    discordgo.SecondaryButton,
						CustomID: prevCustomID,
						Disabled: prevDisabled,
					},
					discordgo.Button{
						Label:    "Next",
						Style:    discordgo.SecondaryButton,
						CustomID: nextCustomID,
						Disabled: nextDisabled,
					},
				},
			},
		}
	}

	_, _ = s.FollowupMessageCreate(i, false, &discordgo.WebhookParams{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
}

func (b *Bot) handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	parts := strings.Split(data.CustomID, ":")
	if len(parts) < 4 {
		return
	}

	action := parts[0]
	domain := parts[1]
	category := parts[2]
	currentPage, _ := strconv.Atoi(parts[3])

	if action != "list_prev" && action != "list_next" {
		return
	}

	domainName, ok := b.configManager.GetDomainName(domain)
	if !ok {
		return
	}

	files, err := b.storage.ListFiles(domainName, category)
	if err != nil {
		return
	}

	var newPage int
	if action == "list_prev" {
		newPage = currentPage - 1
	} else {
		newPage = currentPage + 1
	}

	totalPages := (len(files) + itemsPerPage - 1) / itemsPerPage
	if newPage < 0 {
		newPage = 0
	}
	if newPage >= totalPages {
		newPage = totalPages - 1
	}

	start := newPage * itemsPerPage
	end := start + itemsPerPage
	if end > len(files) {
		end = len(files)
	}

	var sb strings.Builder

	for _, file := range files[start:end] {
		encodedFile := url.PathEscape(file)
		fileURL := fmt.Sprintf("https://%s/%s/%s", domain, category, encodedFile)
		sb.WriteString(fmt.Sprintf("- %s\n", fileURL))
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Files (%d total)", len(files)),
		Description: sb.String(),
		Color:       0x808080,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Page %d/%d", newPage+1, totalPages),
		},
	}

	var components []discordgo.MessageComponent
	if totalPages > 1 {
		prevDisabled := newPage == 0
		nextDisabled := newPage >= totalPages-1

		prevCustomID := fmt.Sprintf("list_prev:%s:%s:%d", domain, category, newPage)
		nextCustomID := fmt.Sprintf("list_next:%s:%s:%d", domain, category, newPage)

		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Previous",
						Style:    discordgo.SecondaryButton,
						CustomID: prevCustomID,
						Disabled: prevDisabled,
					},
					discordgo.Button{
						Label:    "Next",
						Style:    discordgo.SecondaryButton,
						CustomID: nextCustomID,
						Disabled: nextDisabled,
					},
				},
			},
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})
}

func (b *Bot) parseURL(urlStr string) (string, string, string, error) {
	urlStr = strings.TrimPrefix(urlStr, "https://")
	urlStr = strings.TrimPrefix(urlStr, "http://")

	parts := strings.Split(urlStr, "/")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid URL format")
	}

	domain := parts[0]
	category := parts[1]
	filename := parts[2]

	return domain, category, filename, nil
}

func (b *Bot) findAttachment(i *discordgo.InteractionCreate, attachmentID string) *discordgo.MessageAttachment {
	data := i.ApplicationCommandData()
	if data.Resolved != nil && data.Resolved.Attachments != nil {
		if attachment, ok := data.Resolved.Attachments[attachmentID]; ok {
			return attachment
		}
	}
	return nil
}

func (b *Bot) handleDefault(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	data := i.ApplicationCommandData()
	domain := data.Options[0].Value.(string)
	category := data.Options[1].Value.(string)

	domainName, ok := b.configManager.GetDomainName(domain)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid domain",
		})
		return
	}

	_, ok = b.configManager.GetCategoryID(category)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid category",
		})
		return
	}

	if err := b.settingsManager.SetGlobalDefaults(domain, category); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to save defaults: %v", err),
		})
		return
	}

	categoryDisplayName, _ := b.configManager.GetCategoryDisplayName(category)
	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Default settings updated: Domain: `%s`, Category: `%s`", domainName, categoryDisplayName),
	})
}

func (b *Bot) handleSetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	data := i.ApplicationCommandData()
	domain := data.Options[0].Value.(string)
	category := data.Options[1].Value.(string)
	channelID := i.ChannelID

	domainName, ok := b.configManager.GetDomainName(domain)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid domain",
		})
		return
	}

	_, ok = b.configManager.GetCategoryID(category)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Invalid category",
		})
		return
	}

	if err := b.settingsManager.SetChannelConfig(channelID, domain, category); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to save channel config: %v", err),
		})
		return
	}

	categoryDisplayName, _ := b.configManager.GetCategoryDisplayName(category)
	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Channel auto-upload configured: Domain: `%s`, Category: `%s`. Files uploaded to this channel will be automatically uploaded to the CDN.", domainName, categoryDisplayName),
	})
}

func (b *Bot) handleViewChannelDefault(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	config, ok := b.settingsManager.GetChannelConfig(channelID)

	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No auto-upload configuration found for this channel. Use `/set-channel` to configure.",
			},
		})
		return
	}

	domainName, _ := b.configManager.GetDomainName(config.Domain)
	categoryDisplayName, _ := b.configManager.GetCategoryDisplayName(config.Category)

	embed := &discordgo.MessageEmbed{
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Domain",
				Value:  fmt.Sprintf("%s (%s)", config.Domain, domainName),
				Inline: false,
			},
			{
				Name:   "Category",
				Value:  categoryDisplayName,
				Inline: false,
			},
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func (b *Bot) handleResetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID

	// Check if there's a config to remove
	_, ok := b.settingsManager.GetChannelConfig(channelID)
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No auto-upload configuration found for this channel. Use `/set-channel` to configure.",
			},
		})
		return
	}

	// Remove the channel config
	if err := b.settingsManager.RemoveChannelConfig(channelID); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to remove channel configuration: %v", err),
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Channel auto-upload configuration has been removed. This channel will no longer auto-upload files.",
		},
	})
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if message has attachments
	if len(m.Attachments) == 0 {
		return
	}

	// Check for bot mention
	botMentioned := false
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			botMentioned = true
			break
		}
	}

	// Get channel config
	channelConfig, hasChannelConfig := b.settingsManager.GetChannelConfig(m.ChannelID)

	// Determine which domain and category to use
	var domain, category string
	useDefaults := false

	if botMentioned {
		// Bot mentioned - use global defaults
		defaultDomain, defaultCategory := b.settingsManager.GetGlobalDefaults()
		if defaultDomain == "" || defaultCategory == "" {
			// No global defaults set, check if channel has config
			if hasChannelConfig {
				domain = channelConfig.Domain
				category = channelConfig.Category
				useDefaults = true
			}
		} else {
			domain = defaultDomain
			category = defaultCategory
			useDefaults = true
		}
	} else if hasChannelConfig {
		// Channel has auto-upload config
		domain = channelConfig.Domain
		category = channelConfig.Category
		useDefaults = true
	}

	if !useDefaults {
		// Bot was mentioned but no defaults configured - provide guidance
		if botMentioned {
			var content string
			// Check if domains exist first
			if !b.configManager.HasDomains() {
				content = "No domains configured. Please use `/add-domain` to add a domain."
			} else if !b.configManager.HasCategories() {
				// Check if categories exist
				content = "No categories configured. Please use `/add-category` to add a category."
			} else {
				// Both exist but no defaults set
				content = "Please set default domain and category using `/default` command, or configure this channel with `/set-channel`."
			}
			msg := &discordgo.MessageSend{
				Content: content,
				Reference: &discordgo.MessageReference{
					MessageID: m.ID,
					ChannelID: m.ChannelID,
					GuildID:   m.GuildID,
				},
				AllowedMentions: &discordgo.MessageAllowedMentions{
					Parse: []discordgo.AllowedMentionType{},
				},
			}
			s.ChannelMessageSendComplex(m.ChannelID, msg)
		}
		return
	}

	// Check if domains exist
	if !b.configManager.HasDomains() {
		msg := &discordgo.MessageSend{
			Content: "No domains configured. Please use `/add-domain` to add a domain.",
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		}
		s.ChannelMessageSendComplex(m.ChannelID, msg)
		return
	}

	// Check if categories exist
	if !b.configManager.HasCategories() {
		msg := &discordgo.MessageSend{
			Content: "No categories configured. Please use `/add-category` to add a category.",
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		}
		s.ChannelMessageSendComplex(m.ChannelID, msg)
		return
	}

	// Validate domain and category
	if !b.configManager.DomainExists(domain) {
		msg := &discordgo.MessageSend{
			Content: fmt.Sprintf("Domain '%s' does not exist. Use `/list` to see available domains or `/add-domain` to add it.", domain),
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		}
		s.ChannelMessageSendComplex(m.ChannelID, msg)
		return
	}

	_, ok := b.configManager.GetCategoryID(category)
	if !ok {
		msg := &discordgo.MessageSend{
			Content: fmt.Sprintf("Category '%s' does not exist. Use `/list` to see available categories or `/add-category` to add it.", category),
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		}
		s.ChannelMessageSendComplex(m.ChannelID, msg)
		return
	}

	// Get domain URL for file URLs
	domainURL, ok := b.configManager.GetDomainFQDN(domain)
	if !ok {
		msg := &discordgo.MessageSend{
			Content: "Failed to get domain URL. Please check your domain configuration.",
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		}
		s.ChannelMessageSendComplex(m.ChannelID, msg)
		return
	}

	// Process each attachment
	var uploadedURLs []string
	for _, attachment := range m.Attachments {
		fileData, ct, err := storage.DownloadFile(attachment.URL)
		if err != nil {
			continue
		}

		ext := filepath.Ext(attachment.Filename)
		filename, _, err := b.storage.StoreFile(domain, category, fileData, ct, ext)
		if err != nil {
			continue
		}

		encodedCategory := url.PathEscape(category)
		fileURL := fmt.Sprintf("https://%s/%s/%s", domainURL, encodedCategory, filename)
		uploadedURLs = append(uploadedURLs, fileURL)
	}

	// Send response with uploaded URLs
	if len(uploadedURLs) > 0 {
		var content string
		if len(uploadedURLs) == 1 {
			content = uploadedURLs[0]
		} else {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Auto-uploaded %d file(s):\n", len(uploadedURLs)))
			for _, url := range uploadedURLs {
				sb.WriteString(fmt.Sprintf("- %s\n", url))
			}
			content = sb.String()
		}

		msg := &discordgo.MessageSend{
			Content: content,
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Parse: []discordgo.AllowedMentionType{},
			},
		}

		s.ChannelMessageSendComplex(m.ChannelID, msg)
	}
}

func (b *Bot) handleAddDomain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	data := i.ApplicationCommandData()
	domainURL := data.Options[0].Value.(string)
	displayName := data.Options[1].Value.(string)
	folderName := data.Options[2].Value.(string)

	// Validate folder name (no spaces)
	if strings.Contains(folderName, " ") {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Folder name cannot contain spaces. Use dashes instead (e.g., 'my-domain' instead of 'my domain').",
		})
		return
	}

	// Check if domain already exists
	if b.configManager.DomainExists(folderName) {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Domain with folder-name '%s' already exists.", folderName),
		})
		return
	}

	// Add the domain
	if err := b.configManager.AddDomain(folderName, displayName, domainURL); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to add domain: %v", err),
		})
		return
	}

	// Save domains to file
	if err := b.configManager.SaveDomains(b.domainsConfig); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Domain added to memory but failed to save to file: %v", err),
		})
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Domain `%s` (%s) added successfully!", domainURL, displayName),
	})
}

func (b *Bot) handleRemoveDomain(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Check if any domains exist
	if !b.configManager.HasDomains() {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No domains exist. Use `/add-domain` to add a domain.",
		})
		return
	}

	data := i.ApplicationCommandData()
	domainName := data.Options[0].Value.(string)

	// Check if domain exists
	displayName, ok := b.configManager.GetDomainName(domainName)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Domain '%s' not found.", domainName),
		})
		return
	}

	// Check if domain is in use (global defaults or channel configs)
	globalDomain, _ := b.settingsManager.GetGlobalDefaults()
	if globalDomain == domainName {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Cannot remove domain '%s' - it is currently set as the global default. Use `/default` to change the default.", displayName),
		})
		return
	}

	// Check channel configs
	channelConfigs := b.settingsManager.ListChannelConfigs()
	for channelID, cfg := range channelConfigs {
		if cfg.Domain == domainName {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Cannot remove domain '%s' - it is currently configured for channel <#%s>. Use `/set-channel` to change the channel config.", displayName, channelID),
			})
			return
		}
	}

	// Remove the domain
	if err := b.configManager.RemoveDomain(domainName); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to remove domain: %v", err),
		})
		return
	}

	// Save changes to file
	if err := b.configManager.SaveDomains(b.domainsConfig); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Domain removed from memory but failed to save to file: %v", err),
		})
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Domain `%s` (%s) has been removed successfully.", domainName, displayName),
	})
}

func (b *Bot) handleAddCategory(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	data := i.ApplicationCommandData()
	displayName := data.Options[0].Value.(string)
	folderName := data.Options[1].Value.(string)

	// Validate folder name (no spaces)
	if strings.Contains(folderName, " ") {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "Folder name cannot contain spaces. Use dashes instead (e.g., 'my-category' instead of 'my category').",
		})
		return
	}

	// Check if category already exists
	if _, ok := b.configManager.GetCategoryID(folderName); ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Category with folder-name '%s' already exists.", folderName),
		})
		return
	}

	// Add the category
	if err := b.configManager.AddCategory(folderName, displayName); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to add category: %v", err),
		})
		return
	}

	// Save categories to file
	if err := b.configManager.SaveCategories(b.categoriesConfig); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Category added to memory but failed to save to file: %v", err),
		})
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Category `%s` (%s) added successfully!", folderName, displayName),
	})
}

func (b *Bot) handleRemoveCategory(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Check if any categories exist
	if !b.configManager.HasCategories() {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "No categories exist. Use `/add-category` to add a category.",
		})
		return
	}

	data := i.ApplicationCommandData()
	categoryName := data.Options[0].Value.(string)

	// Check if category exists
	displayName, ok := b.configManager.GetCategoryDisplayName(categoryName)
	if !ok {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Category '%s' not found.", categoryName),
		})
		return
	}

	// Check if category is in use (global defaults or channel configs)
	_, globalCategory := b.settingsManager.GetGlobalDefaults()
	if globalCategory == categoryName {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Cannot remove category '%s' - it is currently set as the global default. Use `/default` to change the default.", displayName),
		})
		return
	}

	// Check channel configs
	channelConfigs := b.settingsManager.ListChannelConfigs()
	for channelID, cfg := range channelConfigs {
		if cfg.Category == categoryName {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("Cannot remove category '%s' - it is currently configured for channel <#%s>. Use `/set-channel` to change the channel config.", displayName, channelID),
			})
			return
		}
	}

	// Remove the category
	if err := b.configManager.RemoveCategory(categoryName); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Failed to remove category: %v", err),
		})
		return
	}

	// Save changes to file
	if err := b.configManager.SaveCategories(b.categoriesConfig); err != nil {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Category removed from memory but failed to save to file: %v", err),
		})
		return
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Category `%s` (%s) has been removed successfully.", categoryName, displayName),
	})
}
