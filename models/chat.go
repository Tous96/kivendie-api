// models/chat.go

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Conversation représente une discussion entre deux utilisateurs pour une annonce spécifique.
type Conversation struct {
	ID        int       `json:"id"`
	AdID      int       `json:"ad_id"`
	SellerID  int       `json:"seller_id"`
	BuyerID   int       `json:"buyer_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StringArray pour gérer les tableaux de strings avec PostgreSQL
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal([]byte(v), a)
	}

	return nil
}

// Message représente un message envoyé dans une conversation.
type Message struct {
	ID             int         `json:"id"`
	ConversationID int         `json:"conversation_id"`
	SenderID       string      `json:"sender_id"`
	Text           string      `json:"text,omitempty"`         // omitempty pour les messages de type "offre" ou "image"
	OfferAmount    *float64    `json:"offer_amount,omitempty"` // Pointeur pour gérer les valeurs nulles
	Type           string      `json:"type"`                   // 'text', 'offer' ou 'image'
	IsRead         bool        `json:"is_read"`
	ImageURLs      StringArray `json:"image_urls,omitempty"` // URLs des images uploadées
	CreatedAt      time.Time   `json:"created_at"`
}

// IncomingMessage pour recevoir les messages du WebSocket (avec les images en base64)
type IncomingMessage struct {
	Type        string   `json:"type"`
	SenderID    string   `json:"sender_id"`
	Text        string   `json:"text,omitempty"`
	OfferAmount *float64 `json:"offer_amount,omitempty"`
	Images      []string `json:"images,omitempty"` // Images en base64
}
