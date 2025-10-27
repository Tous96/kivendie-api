package models

import (
	"database/sql"
	"time"
)

// AppSettings représente les paramètres généraux de l'application
type AppSettings struct {
	ID int `json:"id"`

	// Informations générales
	AppName        string `json:"app_name"`
	AppTagline     string `json:"app_tagline,omitempty"`
	AppDescription string `json:"app_description,omitempty"`

	// Logos et icônes
	LogoURL     string `json:"logo_url,omitempty"`
	FaviconURL  string `json:"favicon_url,omitempty"`
	LogoDarkURL string `json:"logo_dark_url,omitempty"`

	// Contact
	SupportEmail   string `json:"support_email,omitempty"`
	ContactPhone   string `json:"contact_phone,omitempty"`
	WhatsAppNumber string `json:"whatsapp_number,omitempty"`

	// Réseaux sociaux
	FacebookURL  string `json:"facebook_url,omitempty"`
	InstagramURL string `json:"instagram_url,omitempty"`
	TwitterURL   string `json:"twitter_url,omitempty"`
	LinkedInURL  string `json:"linkedin_url,omitempty"`
	YouTubeURL   string `json:"youtube_url,omitempty"`
	TikTokURL    string `json:"tiktok_url,omitempty"`

	// Adresse
	PhysicalAddress string `json:"physical_address,omitempty"`
	City            string `json:"city,omitempty"`
	Country         string `json:"country,omitempty"`

	// Informations légales
	CompanyName        string `json:"company_name,omitempty"`
	RegistrationNumber string `json:"registration_number,omitempty"`
	TaxID              string `json:"tax_id,omitempty"`

	// SEO
	MetaTitle       string `json:"meta_title,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`
	MetaKeywords    string `json:"meta_keywords,omitempty"`

	// Paramètres app
	DefaultLanguage string `json:"default_language"`
	Currency        string `json:"currency"`
	Timezone        string `json:"timezone"`

	// Modération
	AutoValidateAds          bool `json:"auto_validate_ads"`
	RequirePhoneVerification bool `json:"require_phone_verification"`
	MaxImagesPerAd           int  `json:"max_images_per_ad"`
	MaxAdDurationDays        int  `json:"max_ad_duration_days"`

	// Email (sensible - ne pas exposer en JSON)
	SMTPHost      string `json:"-"`
	SMTPPort      int    `json:"-"`
	SMTPUsername  string `json:"-"`
	SMTPPassword  string `json:"-"`
	SMTPFromEmail string `json:"smtp_from_email,omitempty"`
	SMTPFromName  string `json:"smtp_from_name,omitempty"`

	// Paiement (sensible)
	KKiaPayPublicKey  string `json:"kkiapay_public_key,omitempty"`
	KKiaPayPrivateKey string `json:"-"`
	KKiaPaySecret     string `json:"-"`
	PaymentEnabled    bool   `json:"payment_enabled"`

	// Métadonnées
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	UpdatedBy sql.NullInt64 `json:"updated_by,omitempty"`
}

// AppSettingsPublic représente les paramètres publics (sans infos sensibles)
type AppSettingsPublic struct {
	AppName         string `json:"app_name"`
	AppTagline      string `json:"app_tagline,omitempty"`
	AppDescription  string `json:"app_description,omitempty"`
	LogoURL         string `json:"logo_url,omitempty"`
	FaviconURL      string `json:"favicon_url,omitempty"`
	LogoDarkURL     string `json:"logo_dark_url,omitempty"`
	SupportEmail    string `json:"support_email,omitempty"`
	ContactPhone    string `json:"contact_phone,omitempty"`
	WhatsAppNumber  string `json:"whatsapp_number,omitempty"`
	FacebookURL     string `json:"facebook_url,omitempty"`
	InstagramURL    string `json:"instagram_url,omitempty"`
	TwitterURL      string `json:"twitter_url,omitempty"`
	LinkedInURL     string `json:"linkedin_url,omitempty"`
	YouTubeURL      string `json:"youtube_url,omitempty"`
	TikTokURL       string `json:"tiktok_url,omitempty"`
	PhysicalAddress string `json:"physical_address,omitempty"`
	City            string `json:"city,omitempty"`
	Country         string `json:"country,omitempty"`
	DefaultLanguage string `json:"default_language"`
	Currency        string `json:"currency"`
	PaymentEnabled  bool   `json:"payment_enabled"`
}

// MaintenanceMode représente le mode maintenance
type MaintenanceMode struct {
	ID int `json:"id"`

	// Statut
	IsActive bool `json:"is_active"`

	// Messages
	Title   string `json:"title"`
	Message string `json:"message"`

	// Planification
	ScheduledStart           *time.Time `json:"scheduled_start,omitempty"`
	ScheduledEnd             *time.Time `json:"scheduled_end,omitempty"`
	EstimatedDurationMinutes int        `json:"estimated_duration_minutes,omitempty"`

	// Notifications
	NotifyUsers   bool `json:"notify_users"`
	ShowCountdown bool `json:"show_countdown"`

	// Accès
	AllowAdminAccess   bool     `json:"allow_admin_access"`
	AllowedIPAddresses []string `json:"allowed_ip_addresses,omitempty"`

	// Contact d'urgence
	EmergencyContactEmail string `json:"emergency_contact_email,omitempty"`
	EmergencyContactPhone string `json:"emergency_contact_phone,omitempty"`

	// Détails
	Reason string `json:"reason,omitempty"`

	// Métadonnées
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	ActivatedBy   sql.NullInt64 `json:"activated_by,omitempty"`
	ActivatedAt   *time.Time    `json:"activated_at,omitempty"`
	DeactivatedAt *time.Time    `json:"deactivated_at,omitempty"`
}

// MaintenanceStatus représente le statut de maintenance (pour les utilisateurs)
type MaintenanceStatus struct {
	IsActive                 bool       `json:"is_active"`
	Title                    string     `json:"title,omitempty"`
	Message                  string     `json:"message,omitempty"`
	ScheduledEnd             *time.Time `json:"scheduled_end,omitempty"`
	EstimatedDurationMinutes int        `json:"estimated_duration_minutes,omitempty"`
	ShowCountdown            bool       `json:"show_countdown"`
	EmergencyContactEmail    string     `json:"emergency_contact_email,omitempty"`
	EmergencyContactPhone    string     `json:"emergency_contact_phone,omitempty"`
}

// UpdateAppSettingsRequest représente la requête de mise à jour des paramètres
type UpdateAppSettingsRequest struct {
	// Informations générales
	AppName        *string `json:"app_name,omitempty"`
	AppTagline     *string `json:"app_tagline,omitempty"`
	AppDescription *string `json:"app_description,omitempty"`

	// Logos
	LogoURL     *string `json:"logo_url,omitempty"`
	FaviconURL  *string `json:"favicon_url,omitempty"`
	LogoDarkURL *string `json:"logo_dark_url,omitempty"`

	// Contact
	SupportEmail   *string `json:"support_email,omitempty"`
	ContactPhone   *string `json:"contact_phone,omitempty"`
	WhatsAppNumber *string `json:"whatsapp_number,omitempty"`

	// Réseaux sociaux
	FacebookURL  *string `json:"facebook_url,omitempty"`
	InstagramURL *string `json:"instagram_url,omitempty"`
	TwitterURL   *string `json:"twitter_url,omitempty"`
	LinkedInURL  *string `json:"linkedin_url,omitempty"`
	YouTubeURL   *string `json:"youtube_url,omitempty"`
	TikTokURL    *string `json:"tiktok_url,omitempty"`

	// Adresse
	PhysicalAddress *string `json:"physical_address,omitempty"`
	City            *string `json:"city,omitempty"`
	Country         *string `json:"country,omitempty"`

	// Légal
	CompanyName        *string `json:"company_name,omitempty"`
	RegistrationNumber *string `json:"registration_number,omitempty"`
	TaxID              *string `json:"tax_id,omitempty"`

	// SEO
	MetaTitle       *string `json:"meta_title,omitempty"`
	MetaDescription *string `json:"meta_description,omitempty"`
	MetaKeywords    *string `json:"meta_keywords,omitempty"`

	// Paramètres
	DefaultLanguage          *string `json:"default_language,omitempty"`
	Currency                 *string `json:"currency,omitempty"`
	Timezone                 *string `json:"timezone,omitempty"`
	AutoValidateAds          *bool   `json:"auto_validate_ads,omitempty"`
	RequirePhoneVerification *bool   `json:"require_phone_verification,omitempty"`
	MaxImagesPerAd           *int    `json:"max_images_per_ad,omitempty"`
	MaxAdDurationDays        *int    `json:"max_ad_duration_days,omitempty"`

	// Email (admin uniquement)
	SMTPHost      *string `json:"smtp_host,omitempty"`
	SMTPPort      *int    `json:"smtp_port,omitempty"`
	SMTPUsername  *string `json:"smtp_username,omitempty"`
	SMTPPassword  *string `json:"smtp_password,omitempty"`
	SMTPFromEmail *string `json:"smtp_from_email,omitempty"`
	SMTPFromName  *string `json:"smtp_from_name,omitempty"`

	// Paiement (admin uniquement)
	KKiaPayPublicKey  *string `json:"kkiapay_public_key,omitempty"`
	KKiaPayPrivateKey *string `json:"kkiapay_private_key,omitempty"`
	KKiaPaySecret     *string `json:"kkiapay_secret,omitempty"`
	PaymentEnabled    *bool   `json:"payment_enabled,omitempty"`
}

// UpdateMaintenanceModeRequest représente la requête pour activer/désactiver la maintenance
type UpdateMaintenanceModeRequest struct {
	IsActive                 *bool      `json:"is_active,omitempty"`
	Title                    *string    `json:"title,omitempty"`
	Message                  *string    `json:"message,omitempty"`
	ScheduledStart           *time.Time `json:"scheduled_start,omitempty"`
	ScheduledEnd             *time.Time `json:"scheduled_end,omitempty"`
	EstimatedDurationMinutes *int       `json:"estimated_duration_minutes,omitempty"`
	NotifyUsers              *bool      `json:"notify_users,omitempty"`
	ShowCountdown            *bool      `json:"show_countdown,omitempty"`
	AllowAdminAccess         *bool      `json:"allow_admin_access,omitempty"`
	AllowedIPAddresses       []string   `json:"allowed_ip_addresses,omitempty"`
	EmergencyContactEmail    *string    `json:"emergency_contact_email,omitempty"`
	EmergencyContactPhone    *string    `json:"emergency_contact_phone,omitempty"`
	Reason                   *string    `json:"reason,omitempty"`
}
