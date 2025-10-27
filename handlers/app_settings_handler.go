package handlers

import (
	"database/sql"
	"encoding/json"
	"kivendi-backend/config"
	"kivendi-backend/models"
	"log"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

// ==============================================================
// ROUTES PUBLIQUES - PARAMÈTRES D'APPLICATION
// ==============================================================

// GetPublicAppSettingsHandler retourne les paramètres publics de l'application
func GetPublicAppSettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := `
		SELECT 
			app_name, app_tagline, app_description,
			logo_url, favicon_url, logo_dark_url,
			support_email, contact_phone, whatsapp_number,
			facebook_url, instagram_url, twitter_url, linkedin_url, youtube_url, tiktok_url,
			physical_address, city, country,
			default_language, currency, payment_enabled
		FROM app_settings
		ORDER BY id DESC
		LIMIT 1
	`

	var settings models.AppSettingsPublic
	var (
		appTagline, appDescription                                                sql.NullString
		logoURL, faviconURL, logoDarkURL                                          sql.NullString
		supportEmail, contactPhone, whatsappNumber                                sql.NullString
		facebookURL, instagramURL, twitterURL, linkedinURL, youtubeURL, tiktokURL sql.NullString
		physicalAddress, city, country                                            sql.NullString
	)

	err := config.DB.QueryRow(query).Scan(
		&settings.AppName, &appTagline, &appDescription,
		&logoURL, &faviconURL, &logoDarkURL,
		&supportEmail, &contactPhone, &whatsappNumber,
		&facebookURL, &instagramURL, &twitterURL,
		&linkedinURL, &youtubeURL, &tiktokURL,
		&physicalAddress, &city, &country,
		&settings.DefaultLanguage, &settings.Currency, &settings.PaymentEnabled,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Utiliser httpError même pour les routes publiques est une bonne pratique
			// si httpError est conçu pour gérer les erreurs publiques.
			// Sinon, http.Error est acceptable ici. Pour la cohérence, utilisons httpError.
			httpError(w, "Paramètres non trouvés", http.StatusNotFound, err)
		} else {
			log.Printf("Erreur lors de la récupération des paramètres publics: %v", err)
			httpError(w, "Erreur interne du serveur", http.StatusInternalServerError, err)
		}
		return
	}

	// Convertir les NullString en string
	if appTagline.Valid {
		settings.AppTagline = appTagline.String
	}
	if appDescription.Valid {
		settings.AppDescription = appDescription.String
	}
	if logoURL.Valid {
		settings.LogoURL = logoURL.String
	}
	if faviconURL.Valid {
		settings.FaviconURL = faviconURL.String
	}
	if logoDarkURL.Valid {
		settings.LogoDarkURL = logoDarkURL.String
	}
	if supportEmail.Valid {
		settings.SupportEmail = supportEmail.String
	}
	if contactPhone.Valid {
		settings.ContactPhone = contactPhone.String
	}
	if whatsappNumber.Valid {
		settings.WhatsAppNumber = whatsappNumber.String
	}
	if facebookURL.Valid {
		settings.FacebookURL = facebookURL.String
	}
	if instagramURL.Valid {
		settings.InstagramURL = instagramURL.String
	}
	if twitterURL.Valid {
		settings.TwitterURL = twitterURL.String
	}
	if linkedinURL.Valid {
		settings.LinkedInURL = linkedinURL.String
	}
	if youtubeURL.Valid {
		settings.YouTubeURL = youtubeURL.String
	}
	if tiktokURL.Valid {
		settings.TikTokURL = tiktokURL.String
	}
	if physicalAddress.Valid {
		settings.PhysicalAddress = physicalAddress.String
	}
	if city.Valid {
		settings.City = city.String
	}
	if country.Valid {
		settings.Country = country.String
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(settings)
}

// GetMaintenanceStatusHandler retourne le statut de maintenance (public)
func GetMaintenanceStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := `
		SELECT 
			is_active, title, message, scheduled_end, 
			estimated_duration_minutes, show_countdown,
			emergency_contact_email, emergency_contact_phone
		FROM maintenance_mode
		ORDER BY id DESC
		LIMIT 1
	`

	var status models.MaintenanceStatus
	var (
		title, message                               sql.NullString
		scheduledEnd                                 sql.NullTime
		estimatedDurationMinutes                     sql.NullInt64
		emergencyContactEmail, emergencyContactPhone sql.NullString
	)

	err := config.DB.QueryRow(query).Scan(
		&status.IsActive, &title, &message,
		&scheduledEnd, &estimatedDurationMinutes,
		&status.ShowCountdown, &emergencyContactEmail,
		&emergencyContactPhone,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Retourner un statut par défaut (pas en maintenance)
			status.IsActive = false
			status.Title = "Service disponible"
			status.Message = ""
		} else {
			log.Printf("Erreur lors de la récupération du statut de maintenance: %v", err)
			httpError(w, "Erreur interne du serveur", http.StatusInternalServerError, err)
			return
		}
	} else {
		// Convertir les NullString et NullInt64
		if title.Valid {
			status.Title = title.String
		}
		if message.Valid {
			status.Message = message.String
		}
		if scheduledEnd.Valid {
			status.ScheduledEnd = &scheduledEnd.Time
		}
		if estimatedDurationMinutes.Valid {
			duration := int(estimatedDurationMinutes.Int64)
			status.EstimatedDurationMinutes = duration
		}
		if emergencyContactEmail.Valid {
			status.EmergencyContactEmail = emergencyContactEmail.String
		}
		if emergencyContactPhone.Valid {
			status.EmergencyContactPhone = emergencyContactPhone.String
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ==============================================================
// ROUTES ADMIN - PARAMÈTRES D'APPLICATION
// ==============================================================

// GetAppSettingsForAdminHandler retourne tous les paramètres (y compris sensibles) pour l'admin
func GetAppSettingsForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Vérifier que l'utilisateur est bien un admin ou modérateur
	_, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	query := `
		SELECT 
			id, app_name, app_tagline, app_description,
			logo_url, favicon_url, logo_dark_url,
			support_email, contact_phone, whatsapp_number,
			facebook_url, instagram_url, twitter_url, linkedin_url, youtube_url, tiktok_url,
			physical_address, city, country,
			company_name, registration_number, tax_id,
			meta_title, meta_description, meta_keywords,
			default_language, currency, timezone,
			auto_validate_ads, require_phone_verification, max_images_per_ad, max_ad_duration_days,
			smtp_host, smtp_port, smtp_username, smtp_password, smtp_from_email, smtp_from_name,
			kkiapay_public_key, kkiapay_private_key, kkiapay_secret, payment_enabled,
			created_at, updated_at, updated_by
		FROM app_settings
		ORDER BY id DESC
		LIMIT 1
	`

	var settings models.AppSettings
	var (
		appTagline, appDescription                                                sql.NullString
		logoURL, faviconURL, logoDarkURL                                          sql.NullString
		supportEmail, contactPhone, whatsappNumber                                sql.NullString
		facebookURL, instagramURL, twitterURL, linkedinURL, youtubeURL, tiktokURL sql.NullString
		physicalAddress, city, country                                            sql.NullString
		companyName, registrationNumber, taxID                                    sql.NullString
		metaTitle, metaDescription, metaKeywords                                  sql.NullString
		timezone                                                                  sql.NullString
		smtpHost, smtpUsername, smtpPassword, smtpFromEmail, smtpFromName         sql.NullString
		smtpPort                                                                  sql.NullInt64
		kkiapayPublicKey, kkiapayPrivateKey, kkiapaySecret                        sql.NullString
	)

	err = config.DB.QueryRow(query).Scan(
		&settings.ID, &settings.AppName, &appTagline, &appDescription,
		&logoURL, &faviconURL, &logoDarkURL,
		&supportEmail, &contactPhone, &whatsappNumber,
		&facebookURL, &instagramURL, &twitterURL,
		&linkedinURL, &youtubeURL, &tiktokURL,
		&physicalAddress, &city, &country,
		&companyName, &registrationNumber, &taxID,
		&metaTitle, &metaDescription, &metaKeywords,
		&settings.DefaultLanguage, &settings.Currency, &timezone,
		&settings.AutoValidateAds, &settings.RequirePhoneVerification,
		&settings.MaxImagesPerAd, &settings.MaxAdDurationDays,
		&smtpHost, &smtpPort, &smtpUsername,
		&smtpPassword, &smtpFromEmail, &smtpFromName,
		&kkiapayPublicKey, &kkiapayPrivateKey, &kkiapaySecret, &settings.PaymentEnabled,
		&settings.CreatedAt, &settings.UpdatedAt, &settings.UpdatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			httpError(w, "Paramètres non trouvés", http.StatusNotFound, err)
		} else {
			// httpError se charge de logger l'erreur
			httpError(w, "Erreur interne du serveur", http.StatusInternalServerError, err)
		}
		return
	}

	// Convertir les NullString et NullInt64
	if appTagline.Valid {
		settings.AppTagline = appTagline.String
	}
	if appDescription.Valid {
		settings.AppDescription = appDescription.String
	}
	if logoURL.Valid {
		settings.LogoURL = logoURL.String
	}
	if faviconURL.Valid {
		settings.FaviconURL = faviconURL.String
	}
	if logoDarkURL.Valid {
		settings.LogoDarkURL = logoDarkURL.String
	}
	if supportEmail.Valid {
		settings.SupportEmail = supportEmail.String
	}
	if contactPhone.Valid {
		settings.ContactPhone = contactPhone.String
	}
	if whatsappNumber.Valid {
		settings.WhatsAppNumber = whatsappNumber.String
	}
	if facebookURL.Valid {
		settings.FacebookURL = facebookURL.String
	}
	if instagramURL.Valid {
		settings.InstagramURL = instagramURL.String
	}
	if twitterURL.Valid {
		settings.TwitterURL = twitterURL.String
	}
	if linkedinURL.Valid {
		settings.LinkedInURL = linkedinURL.String
	}
	if youtubeURL.Valid {
		settings.YouTubeURL = youtubeURL.String
	}
	if tiktokURL.Valid {
		settings.TikTokURL = tiktokURL.String
	}
	if physicalAddress.Valid {
		settings.PhysicalAddress = physicalAddress.String
	}
	if city.Valid {
		settings.City = city.String
	}
	if country.Valid {
		settings.Country = country.String
	}
	if companyName.Valid {
		settings.CompanyName = companyName.String
	}
	if registrationNumber.Valid {
		settings.RegistrationNumber = registrationNumber.String
	}
	if taxID.Valid {
		settings.TaxID = taxID.String
	}
	if metaTitle.Valid {
		settings.MetaTitle = metaTitle.String
	}
	if metaDescription.Valid {
		settings.MetaDescription = metaDescription.String
	}
	if metaKeywords.Valid {
		settings.MetaKeywords = metaKeywords.String
	}
	if timezone.Valid {
		settings.Timezone = timezone.String
	}
	if smtpHost.Valid {
		settings.SMTPHost = smtpHost.String
	}
	if smtpPort.Valid {
		settings.SMTPPort = int(smtpPort.Int64)
	}
	if smtpUsername.Valid {
		settings.SMTPUsername = smtpUsername.String
	}
	if smtpFromEmail.Valid {
		settings.SMTPFromEmail = smtpFromEmail.String
	}
	if smtpFromName.Valid {
		settings.SMTPFromName = smtpFromName.String
	}
	if kkiapayPublicKey.Valid {
		settings.KKiaPayPublicKey = kkiapayPublicKey.String
	}

	// Masquer partiellement les données sensibles
	if smtpPassword.Valid && smtpPassword.String != "" {
		settings.SMTPPassword = "••••••••"
	}
	if kkiapayPrivateKey.Valid && kkiapayPrivateKey.String != "" {
		settings.KKiaPayPrivateKey = "••••••••"
	}
	if kkiapaySecret.Valid && kkiapaySecret.String != "" {
		settings.KKiaPaySecret = "••••••••"
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(settings)
}

// UpdateAppSettingsHandler met à jour les paramètres de l'application
func UpdateAppSettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Récupérer l'ID de l'admin depuis le token
	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins peuvent modifier les paramètres
	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent modifier les paramètres", http.StatusForbidden, nil)
		return
	}

	var req models.UpdateAppSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Données invalides", http.StatusBadRequest, err)
		return
	}

	// Construire la requête de mise à jour dynamique
	updateFields := []string{}
	args := []interface{}{}
	argID := 1

	// Helper pour ajouter un champ
	addField := func(fieldName string, value interface{}) {
		if value != nil {
			updateFields = append(updateFields, fieldName+" = $"+string(rune('0'+argID)))
			args = append(args, value)
			argID++
		}
	}

	// Informations générales
	if req.AppName != nil {
		addField("app_name", *req.AppName)
	}
	if req.AppTagline != nil {
		addField("app_tagline", *req.AppTagline)
	}
	if req.AppDescription != nil {
		addField("app_description", *req.AppDescription)
	}

	// Logos
	if req.LogoURL != nil {
		addField("logo_url", *req.LogoURL)
	}
	if req.FaviconURL != nil {
		addField("favicon_url", *req.FaviconURL)
	}
	if req.LogoDarkURL != nil {
		addField("logo_dark_url", *req.LogoDarkURL)
	}

	// Contact
	if req.SupportEmail != nil {
		addField("support_email", *req.SupportEmail)
	}
	if req.ContactPhone != nil {
		addField("contact_phone", *req.ContactPhone)
	}
	if req.WhatsAppNumber != nil {
		addField("whatsapp_number", *req.WhatsAppNumber)
	}

	// Réseaux sociaux
	if req.FacebookURL != nil {
		addField("facebook_url", *req.FacebookURL)
	}
	if req.InstagramURL != nil {
		addField("instagram_url", *req.InstagramURL)
	}
	if req.TwitterURL != nil {
		addField("twitter_url", *req.TwitterURL)
	}
	if req.LinkedInURL != nil {
		addField("linkedin_url", *req.LinkedInURL)
	}
	if req.YouTubeURL != nil {
		addField("youtube_url", *req.YouTubeURL)
	}
	if req.TikTokURL != nil {
		addField("tiktok_url", *req.TikTokURL)
	}

	// Adresse
	if req.PhysicalAddress != nil {
		addField("physical_address", *req.PhysicalAddress)
	}
	if req.City != nil {
		addField("city", *req.City)
	}
	if req.Country != nil {
		addField("country", *req.Country)
	}

	// Légal
	if req.CompanyName != nil {
		addField("company_name", *req.CompanyName)
	}
	if req.RegistrationNumber != nil {
		addField("registration_number", *req.RegistrationNumber)
	}
	if req.TaxID != nil {
		addField("tax_id", *req.TaxID)
	}

	// SEO
	if req.MetaTitle != nil {
		addField("meta_title", *req.MetaTitle)
	}
	if req.MetaDescription != nil {
		addField("meta_description", *req.MetaDescription)
	}
	if req.MetaKeywords != nil {
		addField("meta_keywords", *req.MetaKeywords)
	}

	// Paramètres
	if req.DefaultLanguage != nil {
		addField("default_language", *req.DefaultLanguage)
	}
	if req.Currency != nil {
		addField("currency", *req.Currency)
	}
	if req.Timezone != nil {
		addField("timezone", *req.Timezone)
	}
	if req.AutoValidateAds != nil {
		addField("auto_validate_ads", *req.AutoValidateAds)
	}
	if req.RequirePhoneVerification != nil {
		addField("require_phone_verification", *req.RequirePhoneVerification)
	}
	if req.MaxImagesPerAd != nil {
		addField("max_images_per_ad", *req.MaxImagesPerAd)
	}
	if req.MaxAdDurationDays != nil {
		addField("max_ad_duration_days", *req.MaxAdDurationDays)
	}

	// Email
	if req.SMTPHost != nil {
		addField("smtp_host", *req.SMTPHost)
	}
	if req.SMTPPort != nil {
		addField("smtp_port", *req.SMTPPort)
	}
	if req.SMTPUsername != nil {
		addField("smtp_username", *req.SMTPUsername)
	}
	if req.SMTPPassword != nil && *req.SMTPPassword != "••••••••" {
		addField("smtp_password", *req.SMTPPassword)
	}
	if req.SMTPFromEmail != nil {
		addField("smtp_from_email", *req.SMTPFromEmail)
	}
	if req.SMTPFromName != nil {
		addField("smtp_from_name", *req.SMTPFromName)
	}

	// Paiement
	if req.KKiaPayPublicKey != nil {
		addField("kkiapay_public_key", *req.KKiaPayPublicKey)
	}
	if req.KKiaPayPrivateKey != nil && *req.KKiaPayPrivateKey != "••••••••" {
		addField("kkiapay_private_key", *req.KKiaPayPrivateKey)
	}
	if req.KKiaPaySecret != nil && *req.KKiaPaySecret != "••••••••" {
		addField("kkiapay_secret", *req.KKiaPaySecret)
	}
	if req.PaymentEnabled != nil {
		addField("payment_enabled", *req.PaymentEnabled)
	}

	// Métadonnées
	addField("updated_by", requestingAdminID)
	addField("updated_at", "NOW()")

	if len(updateFields) <= 2 { // Seuls updated_by et updated_at
		httpError(w, "Aucune donnée à mettre à jour", http.StatusBadRequest, nil)
		return
	}

	query := `UPDATE app_settings SET ` + strings.Join(updateFields, ", ") + ` WHERE id = (SELECT id FROM app_settings ORDER BY id DESC LIMIT 1)`

	_, err = config.DB.Exec(query, args...)
	if err != nil {
		httpError(w, "Erreur lors de la mise à jour", http.StatusInternalServerError, err)
		return
	}

	log.Printf("Paramètres de l'application mis à jour par l'admin %d", requestingAdminID)

	response := map[string]string{
		"message": "Paramètres mis à jour avec succès",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ==============================================================
// ROUTES ADMIN - MODE MAINTENANCE
// ==============================================================

// GetMaintenanceModeForAdminHandler retourne les détails complets du mode maintenance
func GetMaintenanceModeForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Vérifier que l'utilisateur est bien un admin ou modérateur
	_, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	query := `
		SELECT 
			id, is_active, title, message,
			scheduled_start, scheduled_end, estimated_duration_minutes,
			notify_users, show_countdown, allow_admin_access, allowed_ip_addresses,
			emergency_contact_email, emergency_contact_phone, reason,
			created_at, updated_at, activated_by, activated_at, deactivated_at
		FROM maintenance_mode
		ORDER BY id DESC
		LIMIT 1
	`

	var maintenance models.MaintenanceMode
	var (
		title, message                                       sql.NullString
		scheduledStart, scheduledEnd                         sql.NullTime
		estimatedDurationMinutes                             sql.NullInt64
		allowedIPs                                           pq.StringArray
		emergencyContactEmail, emergencyContactPhone, reason sql.NullString
		activatedBy                                          sql.NullInt64
		activatedAt, deactivatedAt                           sql.NullTime
	)

	err = config.DB.QueryRow(query).Scan(
		&maintenance.ID, &maintenance.IsActive, &title, &message,
		&scheduledStart, &scheduledEnd, &estimatedDurationMinutes,
		&maintenance.NotifyUsers, &maintenance.ShowCountdown, &maintenance.AllowAdminAccess, &allowedIPs,
		&emergencyContactEmail, &emergencyContactPhone, &reason,
		&maintenance.CreatedAt, &maintenance.UpdatedAt, &activatedBy,
		&activatedAt, &deactivatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			httpError(w, "Mode maintenance non configuré", http.StatusNotFound, err)
		} else {
			httpError(w, "Erreur interne du serveur", http.StatusInternalServerError, err)
		}
		return
	}

	// Convertir les NullString, NullTime, NullInt64
	if title.Valid {
		maintenance.Title = title.String
	}
	if message.Valid {
		maintenance.Message = message.String
	}
	if scheduledStart.Valid {
		maintenance.ScheduledStart = &scheduledStart.Time
	}
	if scheduledEnd.Valid {
		maintenance.ScheduledEnd = &scheduledEnd.Time
	}
	if estimatedDurationMinutes.Valid {
		duration := int(estimatedDurationMinutes.Int64)
		maintenance.EstimatedDurationMinutes = duration
	}
	if emergencyContactEmail.Valid {
		maintenance.EmergencyContactEmail = emergencyContactEmail.String
	}
	if emergencyContactPhone.Valid {
		maintenance.EmergencyContactPhone = emergencyContactPhone.String
	}
	if reason.Valid {
		maintenance.Reason = reason.String
	}
	if activatedBy.Valid {
		maintenance.ActivatedBy = sql.NullInt64{Int64: activatedBy.Int64, Valid: true}
	}
	if activatedAt.Valid {
		maintenance.ActivatedAt = &activatedAt.Time
	}
	if deactivatedAt.Valid {
		maintenance.DeactivatedAt = &deactivatedAt.Time
	}

	maintenance.AllowedIPAddresses = []string(allowedIPs)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(maintenance)
}

// UpdateMaintenanceModeHandler active/désactive ou met à jour le mode maintenance
func UpdateMaintenanceModeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Récupérer l'ID de l'admin
	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins peuvent modifier le mode maintenance
	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent modifier le mode maintenance", http.StatusForbidden, nil)
		return
	}

	var req models.UpdateMaintenanceModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Données invalides", http.StatusBadRequest, err)
		return
	}

	// Construire la requête dynamique
	updateFields := []string{}
	args := []interface{}{}
	argID := 1

	if req.IsActive != nil {
		updateFields = append(updateFields, "is_active = $"+string(rune('0'+argID)))
		args = append(args, *req.IsActive)
		argID++

		// Mettre à jour activated_at ou deactivated_at
		if *req.IsActive {
			updateFields = append(updateFields, "activated_at = NOW()")
			updateFields = append(updateFields, "activated_by = $"+string(rune('0'+argID)))
			args = append(args, requestingAdminID)
			argID++
		} else {
			updateFields = append(updateFields, "deactivated_at = NOW()")
		}
	}

	if req.Title != nil {
		updateFields = append(updateFields, "title = $"+string(rune('0'+argID)))
		args = append(args, *req.Title)
		argID++
	}

	if req.Message != nil {
		updateFields = append(updateFields, "message = $"+string(rune('0'+argID)))
		args = append(args, *req.Message)
		argID++
	}

	if req.ScheduledStart != nil {
		updateFields = append(updateFields, "scheduled_start = $"+string(rune('0'+argID)))
		args = append(args, *req.ScheduledStart)
		argID++
	}

	if req.ScheduledEnd != nil {
		updateFields = append(updateFields, "scheduled_end = $"+string(rune('0'+argID)))
		args = append(args, *req.ScheduledEnd)
		argID++
	}

	if req.EstimatedDurationMinutes != nil {
		updateFields = append(updateFields, "estimated_duration_minutes = $"+string(rune('0'+argID)))
		args = append(args, *req.EstimatedDurationMinutes)
		argID++
	}

	if req.NotifyUsers != nil {
		updateFields = append(updateFields, "notify_users = $"+string(rune('0'+argID)))
		args = append(args, *req.NotifyUsers)
		argID++
	}

	if req.ShowCountdown != nil {
		updateFields = append(updateFields, "show_countdown = $"+string(rune('0'+argID)))
		args = append(args, *req.ShowCountdown)
		argID++
	}

	if req.AllowAdminAccess != nil {
		updateFields = append(updateFields, "allow_admin_access = $"+string(rune('0'+argID)))
		args = append(args, *req.AllowAdminAccess)
		argID++
	}

	if req.AllowedIPAddresses != nil {
		updateFields = append(updateFields, "allowed_ip_addresses = $"+string(rune('0'+argID)))
		args = append(args, pq.Array(req.AllowedIPAddresses))
		argID++
	}

	if req.EmergencyContactEmail != nil {
		updateFields = append(updateFields, "emergency_contact_email = $"+string(rune('0'+argID)))
		args = append(args, *req.EmergencyContactEmail)
		argID++
	}

	if req.EmergencyContactPhone != nil {
		updateFields = append(updateFields, "emergency_contact_phone = $"+string(rune('0'+argID)))
		args = append(args, *req.EmergencyContactPhone)
		argID++
	}

	if req.Reason != nil {
		updateFields = append(updateFields, "reason = $"+string(rune('0'+argID)))
		args = append(args, *req.Reason)
		argID++
	}

	if len(updateFields) == 0 {
		httpError(w, "Aucune donnée à mettre à jour", http.StatusBadRequest, nil)
		return
	}

	updateFields = append(updateFields, "updated_at = NOW()")

	query := `UPDATE maintenance_mode SET ` + strings.Join(updateFields, ", ") + ` WHERE id = (SELECT id FROM maintenance_mode ORDER BY id DESC LIMIT 1)`

	_, err = config.DB.Exec(query, args...)
	if err != nil {
		httpError(w, "Erreur lors de la mise à jour", http.StatusInternalServerError, err)
		return
	}

	log.Printf("Mode maintenance mis à jour par l'admin %d", requestingAdminID)

	response := map[string]string{
		"message": "Mode maintenance mis à jour avec succès",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
