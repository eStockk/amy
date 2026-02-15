package config

import "os"

type Config struct {
	Port                 string
	MongoURI             string
	MongoDB              string
	FrontendURL          string
	DiscordClientID      string
	DiscordClientSecret  string
	DiscordRedirectURL   string
	DiscordTicketWebhook string
	DiscordRPWebhook     string
	RPModeratorIDs       string
	MinecraftServerToken string
}

func Load() Config {
	return Config{
		Port:                 getEnv("PORT", "8080"),
		MongoURI:             getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:              getEnv("MONGO_DB", "minecraft"),
		FrontendURL:          getEnv("FRONTEND_URL", "http://localhost:3000"),
		DiscordClientID:      getEnv("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret:  getEnv("DISCORD_CLIENT_SECRET", ""),
		DiscordRedirectURL:   getEnv("DISCORD_REDIRECT_URL", "http://localhost:8080/api/auth/discord/callback"),
		DiscordTicketWebhook: getEnv("DISCORD_TICKET_WEBHOOK", ""),
		DiscordRPWebhook:     getEnv("DISCORD_RP_WEBHOOK", ""),
		RPModeratorIDs:       getEnv("DISCORD_RP_MODERATOR_IDS", ""),
		MinecraftServerToken: getEnv("MINECRAFT_SERVER_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
