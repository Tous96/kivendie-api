package models

import (
	"database/sql"
	"time"
)

// User représente la structure des données d'un utilisateur
type User struct {
	ID               int            `json:"id"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	Email            string         `json:"email"`
	PasswordHash     string         `json:"-"` // Ne jamais exposer le hash
	AccountType      string         `json:"account_type"`
	ShopName         sql.NullString `json:"-"` // Géré séparément pour la conversion JSON
	AvatarURL        sql.NullString `json:"-"` // Géré séparément pour la conversion JSON
	VerificationCode sql.NullString `json:"-"` // Usage interne uniquement
	IsVerified       bool           `json:"is_verified"`
	IsBlocked        bool           `json:"is_blocked"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`

	// Champs calculés (non-BDD) - ne pas inclure dans les scans de base de données
	DisplayName  string `json:"display_name" db:"-"`   // Calculé après récupération
	IsProAccount bool   `json:"is_pro_account" db:"-"` // Calculé après récupération
}

// ComputeFields calcule les champs DisplayName et IsProAccount
// À appeler après avoir récupéré un User de la base de données
func (u *User) ComputeFields() {
	// Calculer IsProAccount
	u.IsProAccount = u.AccountType == "Professionnel"

	// Calculer DisplayName
	if u.IsProAccount && u.ShopName.Valid {
		u.DisplayName = u.ShopName.String
	} else {
		u.DisplayName = u.FirstName + " " + u.LastName
	}
}

// UserResponse représente la structure pour les réponses JSON
type UserResponse struct {
	ID           int       `json:"id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Email        string    `json:"email"`
	AccountType  string    `json:"account_type"`
	ShopName     string    `json:"shop_name"`  // String simple pour JSON
	AvatarURL    string    `json:"avatar_url"` // String simple pour JSON
	IsVerified   bool      `json:"is_verified"`
	IsBlocked    bool      `json:"is_blocked"`
	DisplayName  string    `json:"display_name"`   // Calculé
	IsProAccount bool      `json:"is_pro_account"` // Calculé
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ToResponse convertit un User en UserResponse pour l'API
func (u *User) ToResponse() UserResponse {
	shopName := ""
	if u.ShopName.Valid {
		shopName = u.ShopName.String
	}

	avatarURL := ""
	if u.AvatarURL.Valid {
		avatarURL = u.AvatarURL.String
	}

	// Calculer DisplayName et IsProAccount si ce n'est pas déjà fait
	displayName := u.DisplayName
	if displayName == "" {
		if u.AccountType == "Professionnel" && u.ShopName.Valid {
			displayName = u.ShopName.String
		} else {
			displayName = u.FirstName + " " + u.LastName
		}
	}

	isProAccount := u.IsProAccount
	if !isProAccount {
		isProAccount = u.AccountType == "Professionnel"
	}

	return UserResponse{
		ID:           u.ID,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Email:        u.Email,
		AccountType:  u.AccountType,
		ShopName:     shopName,
		AvatarURL:    avatarURL,
		IsVerified:   u.IsVerified,
		IsBlocked:    u.IsBlocked,
		DisplayName:  displayName,
		IsProAccount: isProAccount,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// RegisterRequest représente les données d'inscription
type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	AccountType string `json:"account_type"`
	ShopName    string `json:"shop_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}
