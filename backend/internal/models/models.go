package models

import "time"

type News struct {
	ID        any       `bson:"_id,omitempty" json:"id"`
	Title     string    `bson:"title" json:"title"`
	Intro     string    `bson:"intro" json:"intro"`
	Tags      []string  `bson:"tags" json:"tags"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

// User represents a registered account.
type User struct {
	ID           any       `bson:"_id,omitempty" json:"id"`
	Email        string    `bson:"email" json:"email"`
	Name         string    `bson:"name" json:"name"`
	PasswordHash string    `bson:"passwordHash" json:"-"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

// Ticket represents a support request.
type Ticket struct {
	ID        any       `bson:"_id,omitempty" json:"id"`
	Name      string    `bson:"name" json:"name"`
	Email     string    `bson:"email" json:"email"`
	Subject   string    `bson:"subject" json:"subject"`
	Category  string    `bson:"category" json:"category"`
	Message   string    `bson:"message" json:"message"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}
