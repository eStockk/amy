package models

import "time"

type News struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Intro     string    `json:"intro"`
	Tags      []string  `json:"tags"`
	Source    string    `json:"source,omitempty"`
	URL       string    `json:"url,omitempty"`
	ImageURL  string    `json:"imageUrl,omitempty"`
	Category  string    `json:"category,omitempty"`
	Author    string    `json:"author,omitempty"`
	AuthorID  string    `json:"authorId,omitempty"`
	Variant   string    `json:"variant,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Ticket represents a support request.
type Ticket struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Email            string     `json:"email"`
	DiscordNick      string     `json:"discordNick"`
	OwnerDiscordID   string     `json:"-"`
	Subject          string     `json:"subject"`
	Category         string     `json:"category"`
	Message          string     `json:"message"`
	Status           string     `json:"status"`
	ModerationToken  string     `json:"-"`
	DiscordMessageID string     `json:"-"`
	DiscordChannelID string     `json:"-"`
	UnreadAdminCount int        `json:"unreadAdminCount"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type TicketMessage struct {
	ID                  int64              `json:"id"`
	TicketID            int64              `json:"ticketId"`
	AuthorType          string             `json:"authorType"`
	AuthorName          string             `json:"authorName"`
	AuthorDiscordID     string             `json:"authorDiscordId,omitempty"`
	AuthorDiscordStatus string             `json:"authorDiscordStatus,omitempty"`
	Message             string             `json:"message"`
	Attachments         []TicketAttachment `json:"attachments,omitempty"`
	ReadByUser          bool               `json:"readByUser"`
	CreatedAt           time.Time          `json:"createdAt"`
}

type TicketAttachment struct {
	ID          int64     `json:"id"`
	TicketID    int64     `json:"ticketId"`
	MessageID   int64     `json:"messageId"`
	FileName    string    `json:"fileName"`
	MimeType    string    `json:"mimeType"`
	SizeBytes   int64     `json:"sizeBytes"`
	URL         string    `json:"url"`
	StoragePath string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}
