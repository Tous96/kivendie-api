package models

import (
	"database/sql"
	"time"
)

// Category représente une catégorie principale (ex: "Véhicules")
type Category struct {
	ID            int           `json:"id"`
	Name          string        `json:"name"`
	Icon          string        `json:"icon"`
	SubCategories []SubCategory `json:"sub_categories"`
}

// SubCategory représente une sous-catégorie (ex: "Voitures")
type SubCategory struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Icon       string `json:"icon"`
	CategoryID int    `json:"category_id"`
}

// Nouveau modèle pour les détails de vente
type SoldAd struct {
	Ad
	SoldAt       time.Time `json:"sold_at"`
	SalePrice    *float64  `json:"sale_price"`
	BuyerContact *string   `json:"buyer_contact"`
	Notes        *string   `json:"notes"`
}

// PaginatedAdsResponse contient les résultats de la recherche avec les métadonnées de pagination
type PaginatedAdsResponse struct {
	TotalCount int  `json:"total_count"` // Nombre total d'annonces correspondantes
	Page       int  `json:"page"`        // Numéro de la page actuelle
	Limit      int  `json:"limit"`       // Nombre d'annonces par page
	Ads        []Ad `json:"ads"`         // La liste des annonces pour la page actuelle
}

// Ad représente une annonce
type Ad struct {
	ID              int      `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Price           float64  `json:"price"`
	SubCategoryID   int      `json:"sub_category_id"`
	SubCategoryName string   `json:"sub_category_name"` // Nouveau champ
	CategoryName    string   `json:"category_name"`     // Nouveau champ
	IsValidated     bool     `json:"is_validated"`
	IsDeactivated   bool     `json:"is_deactivated"`
	IsRejected      bool     `json:"is_rejected"`
	Images          []string `json:"images"`
	//FormData      []byte   `json:"form_data"`
	FormData            map[string]interface{} `json:"form_data"`
	City                string                 `json:"city"`
	PhoneNumber         string                 `json:"phone_number"`
	IsPhoneVisible      bool                   `json:"is_phone_visible"`
	IsDeliveryAvailable bool                   `json:"is_delivery_available"` // NOUVEAU CHAMP
	Latitude            sql.NullFloat64        `json:"latitude"`
	Longitude           sql.NullFloat64        `json:"longitude"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	ViewsCount          int                    `json:"views_count"`
	// Nouveau champ pour le nombre de favoris
	FavoritesCount int  `json:"favorites_count"`
	IsSold         bool `json:"is_sold"`

	IsBoosted      bool       `json:"is_boosted"`
	BoostExpiresAt *time.Time `json:"boost_expires_at,omitempty"`

	// Informations de l'utilisateur
	User struct {
		ID           int            `json:"id"`
		FirstName    string         `json:"first_name,omitempty"`
		LastName     string         `json:"last_name,omitempty"`
		ShopName     sql.NullString `json:"shop_name,omitempty"`
		AvatarURL    sql.NullString `json:"avatar_url,omitempty"` // NOUVEAU CHAMP
		IsProAccount bool           `json:"is_pro_account"`
		DisplayName  string         `json:"display_name"`
	} `json:"user"`
}

// BoostOffer représente une offre de boost disponible à l'achat
type BoostOffer struct {
	ID               int                    `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	DurationDays     int                    `json:"duration_days"`
	Price            float64                `json:"price"`
	PositionPriority int                    `json:"position_priority"`
	Features         map[string]interface{} `json:"features"`
	Color            string                 `json:"color"`
	IsActive         bool                   `json:"is_active"`
	DisplayOrder     int                    `json:"display_order"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}
