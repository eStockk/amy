package models

import "time"

type News struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Intro     string    `json:"intro"`
	Tags      []string  `json:"tags"`
	Source    string    `json:"source,omitempty"`
	URL       string    `json:"url,omitempty"`
	Variant   string    `json:"variant,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Ticket represents a support request.
type Ticket struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Email            string     `json:"email"`
	DiscordNick      string     `json:"discordNick"`
	Subject          string     `json:"subject"`
	Category         string     `json:"category"`
	Message          string     `json:"message"`
	Status           string     `json:"status"`
	ModerationToken  string     `json:"-"`
	DiscordMessageID string     `json:"-"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}
