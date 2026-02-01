# Vixa
A simple, lightweight, single-origin, self-hosted CDN server with a Discord bot for managing files — all in a single binary.

<img src="/.github/screenshot.jpg"/>

## Features
- Add unlimited domains to organize your CDN
- Organize files into categories within each domain
- Upload and manage files directly from Discord
- Automatic file uploads when mentioning the bot
- Set default values or channel-specific configurations (for automatic file uploads without mentioning the bot)
- List and delete files through Discord commands
- Randomly generates filenames to prevent guessing file URLs
- Static file serving with caching headers
- Support for CORS requests

More aren't planned but feel free to add them yourself.

## How it works
This project consists of a **web server** and a **Discord bot** that work together to provide a simple, domain-based file hosting and CDN system.

### Web server
The web server runs on port 8080 and serves files through HTTP. It organizes files by domains and categories. When you request a file, the server checks the domain from the host header, finds the category and filename from the URL path, then returns the file with proper headers for caching.

The server supports:
- GET and HEAD requests
- CORS headers for cross-origin requests
- ETag for cache validation
- Long cache times (1 year) for static files

Files are stored in the local filesystem and organized as `storage/domain/category/filename`.

### Discord bot
The Discord bot lets you manage files without using HTTP requests. It connects to your Discord server using a bot token and responds to slash commands and mentions.

You can:
- Upload files to the CDN
- Delete files from the CDN
- List files in a category
- Manage domains and categories
- Set default values for uploads
- Configure channels for automatic uploads

The bot has three upload modes:
1. Use the `/upload` slash command to upload a file with specific domain and category
2. Mention the bot in a message with an attachment to auto-upload (requires defaults or channel config)
3. Send files to a channel and bot automatically uploads them (requires channel configuration (use `/set-channel`))

## Bot commands

| Command | Description | Arguments |
|--------|-------------|-----------|
| `/upload` | Upload a file to the CDN | file (required), category (optional), domain (optional) |
| `/delete` | Delete a file from the CDN | url (required) |
| `/list` | List all files in a category | domain (required), category (required) |
| `/default` | Set default domain and category for uploads | domain (required), category (required) |
| `/set-channel` | Set auto-upload config for channel | domain (required), category (required) |
| `/view-channel-default` | View the auto-upload settings for channel | none |
| `/reset-channel` | Remove the auto-upload configuration for channel | none |
| `/add-domain` | Add a new CDN domain | domain-fqdn (required), display-name (required), folder-name (required) |
| `/remove-domain` | Remove a CDN domain | domain-name (required) |
| `/add-category` | Add a new category | category-name (required), folder-name (required) |
| `/remove-category` | Remove a category | category-name (required) |

## Key concepts (Discord bot)

### domain-fqdn
This is the actual domain name used in URLs. When someone accesses your CDN, they use this domain in their browser or application. For example, if your domain-fqdn is `shadowarcanist.com`, the file URL would be `https://shadowarcanist.com/images/filename.ext`.

### folder-name
This is the internal folder name used in the storage system. It cannot contain spaces and should use dashes instead. This name is used for organizing files on the server filesystem. For example, a domain with folder-name `main-cdn` would store files in `storage/main-cdn/`.

### category
A category is a folder within a domain that groups related files together. Each category has a display name (shown in Discord) and a folder-name (used for storage). For example, a category with display name "Images" and folder-name "images" would store files in `storage/domain-name/images/`.

The complete file structure on disk is: `storage/domain-folder-name/category-folder-name/filename.ext`

The complete URL structure is: `https://domain-fqdn/category-folder-name/filename.ext`

## File upload limits
The maximum file size you can upload depends on your Discord account subscription level and Discord server boost level:
- Account: free users: 10MB
- Account: Nitro Basic users: 50MB
- Account: Nitro users: 500MB
- Server: boosting level 0: 10MB
- Server: boosting level 1: 10MB
- Server: boosting level 2: 50MB
- Server: boosting level 3: 100MB

These limits are enforced by Discord, not by this application.

## Deployment
You can deploy this application using the provided Docker Compose file or through Coolify.
<details>
<summary><strong>Deploy Using Docker Compose</strong></summary>

1. Create a `docker-compose.yaml` file on your server and paste the contents of the `docker-compose.yaml` from this repo.
2. Update the volume paths in the compose file if needed.
3. Enter value for `BOT_TOKEN` variable on compose file.
4. Run `docker compose up`.

Note: You will need to set up a reverse proxy in front of this application to forward requests to the CDN domain you are using. I haven’t configured this myself, so I’m not including step-by-step instructions here. You can use your preferred AI tool for guidance on setting this up.
</details>

<details>
<summary><strong>Deploy Using Coolify</strong></summary>

1. Add a new resource in Coolify → "Docker Compose Empty."
2. Paste the contents of the `coolify-compose.yaml` from the repo into the input field.
3. Update the volume paths in the compose file if needed.
4. On the Coolify General page, click the Settings option for the service. Then enter the domain you want to use for the CDN, making sure to append port `:8080`.
    - Example (single domain): `https://storage.shadowarcanist.com:8080`
    - Example (multiple domains): `https://storage.shadowarcanist.com:8080,https://sub.domain2.com:8080,https://test.domain3.com:8080`
    - Make sure all domains are separated by commas and include port `:8080`.
4. Go to "Environment Variables" page on Coolify and enter value for `BOT_TOKEN` variable
3. Click "Deploy!"
</details>


### Local development
1. Install Go 1.23 or later
2. Copy `.env.example` to a file named `.env` and set your `BOT_TOKEN`
3. Run the application:

```bash
go run cmd/main.go
```

4. The server will start on port 8080

## Environment variables
- `BOT_TOKEN` (required): Your Discord bot token from the Discord Developer Portal
- `PORT` (optional): The port for the web server (default: 8080)

## Storage
Files are stored in the `storage` directory, organized by domain and category. The `configs` directory contains configuration files for domains, categories, and settings.


## Getting a Discord bot token
1. Go to the Discord Developer Portal at https://discord.com/developers/applications
2. Create a new application
3. Go to the "Bot" section and click "Add Bot"
4. Copy the token under "Token" section (Discord won't show token by default so you have to click on `reset token` button to get token)
5. Enable the following bot intents under the "Privileged Gateway Intents" section:
   - Message Content Intent
6. Under "OAuth2" > "URL Generator", select `bot` scope, and select `Send Message` permission
7. Select "Guild Install" as Integration Type
7. Use the generated URL to invite the bot to your server


## Notes
1. This project was entirely created using AI, but the application has been thoroughly tested.
2. It is inspired by [Sapphire Images](https://github.com/Sapphire-Discord-Bot/images).
3. This project was built primarily for my personal use, so I will not be merging pull requests or adding new features unless I need them myself. If you want to make changes or add features, feel free to fork this repository. It’s open-sourced so others can learn from it, use it as a base for their own projects, or even run the application as-is.