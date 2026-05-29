package config

import "os"

type Config struct {
	Port                   string
	DatabaseURL            string
	FrontendURL            string
	DiscordClientID        string
	DiscordClientSecret    string
	DiscordRedirectURL     string
	DiscordTicketWebhook   string
	DiscordRPWebhook       string
	RPModeratorIDs         string
	MinecraftServerAddr    string
	TelegramNewsChannel    string
	DiscordNewsChannelID   string
	DiscordTicketChannelID string
	DiscordBotToken        string
	DiscordGuildID         string
	VAPIDPublicKey         string
	VAPIDPrivateKey        string
	SupportPushSubject     string
	SupportStorageDir      string
	SkinStorageDir         string
	MediaCacheDir          string
	TenorAPIKey            string
}

func Load() Config {
	return Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://amy_user:change_me@localhost:5432/amy?sslmode=disable"),
		FrontendURL:            getEnv("FRONTEND_URL", "http://localhost:3000"),
		DiscordClientID:        getEnv("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret:    getEnv("DISCORD_CLIENT_SECRET", ""),
		DiscordRedirectURL:     getEnv("DISCORD_REDIRECT_URL", "http://localhost:8080/api/auth/discord/callback"),
		DiscordTicketWebhook:   getEnv("DISCORD_TICKET_WEBHOOK", ""),
		DiscordRPWebhook:       getEnv("DISCORD_RP_WEBHOOK", ""),
		RPModeratorIDs:         getEnv("DISCORD_RP_MODERATOR_IDS", ""),
		MinecraftServerAddr:    getEnv("MINECRAFT_SERVER_ADDRESS", "amyworld.ru"),
		TelegramNewsChannel:    getEnv("TELEGRAM_NEWS_CHANNEL", ""),
		DiscordNewsChannelID:   getEnv("DISCORD_NEWS_CHANNEL_ID", ""),
		DiscordTicketChannelID: getEnv("DISCORD_TICKET_CHANNEL_ID", ""),
		DiscordBotToken:        getEnv("DISCORD_BOT_TOKEN", ""),
		DiscordGuildID:         getEnv("DISCORD_GUILD_ID", ""),
		VAPIDPublicKey:         getEnv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey:        getEnv("VAPID_PRIVATE_KEY", ""),
		SupportPushSubject:     getEnv("SUPPORT_PUSH_SUBJECT", "mailto:support@amyworld.ru"),
		SupportStorageDir:      getEnv("SUPPORT_STORAGE_DIR", "data/support"),
		SkinStorageDir:         getEnv("SKIN_STORAGE_DIR", "data/skins"),
		MediaCacheDir:          getEnv("MEDIA_CACHE_DIR", "data/media-cache"),
		TenorAPIKey:            getEnv("TENOR_API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
