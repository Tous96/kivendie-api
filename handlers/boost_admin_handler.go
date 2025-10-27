package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"kivendi-backend/config"
	"kivendi-backend/models"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

// BoostedAdAdminView définit la structure pour visualiser les annonces boostées dans le panneau d'administration.
type BoostedAdAdminView struct {
	BoostID         int       `json:"boost_id"`
	AdID            int       `json:"ad_id"`
	AdTitle         string    `json:"ad_title"`
	AdImage         string    `json:"ad_image"`
	UserID          int       `json:"user_id"`
	UserDisplayName string    `json:"user_display_name"`
	OfferName       string    `json:"offer_name"`
	AmountPaid      float64   `json:"amount_paid"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	IsActive        bool      `json:"is_active"`
	PaymentStatus   string    `json:"payment_status"`
	TransactionID   string    `json:"transaction_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// BoostableAdAdminView définit la structure pour les annonces non-boostées
type BoostableAdAdminView struct {
	AdID            int    `json:"ad_id"`
	AdTitle         string `json:"ad_title"`
	AdImage         string `json:"ad_image"`
	UserID          int    `json:"user_id"`
	UserDisplayName string `json:"user_display_name"`
	CreatedAt       string `json:"created_at"`
}

// GetNonBoostedAdsForAdminHandler récupère les annonces validées qui n'ont pas de boost actif.
func GetNonBoostedAdsForAdminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Admin: Début de la récupération des annonces non-boostées (boostables).")

	// --- Pagination ---
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 10 {
		limit = 10
	}
	offset := (page - 1) * limit

	// --- Requête de données ---
	var boostableAds []BoostableAdAdminView
	query := `
		SELECT
			a.id as ad_id,
			a.title as ad_title,
			a.images as ad_images,
			u.id as user_id,
			CASE
				WHEN u.account_type = 'Professionnel' AND u.shop_name IS NOT NULL AND u.shop_name != '' THEN u.shop_name
				ELSE u.first_name || ' ' || u.last_name
			END as user_display_name,
			a.created_at
		FROM ads a
		JOIN users u ON a.user_id = u.id
		WHERE
			a.is_validated = TRUE
			AND a.is_deactivated = FALSE
			AND a.is_rejected = FALSE
			AND a.is_sold = FALSE
			AND NOT EXISTS (
				SELECT 1
				FROM ad_boosts ab
				WHERE ab.ad_id = a.id
				AND ab.is_active = TRUE
				AND ab.end_date > NOW()
			)
		ORDER BY a.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := config.DB.Query(query, limit, offset)
	if err != nil {
		log.Printf("Erreur admin (boostables): récupération des données: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ad BoostableAdAdminView
		var images pq.StringArray
		err := rows.Scan(
			&ad.AdID, &ad.AdTitle, &images, &ad.UserID, &ad.UserDisplayName, &ad.CreatedAt,
		)
		if err != nil {
			log.Printf("Erreur admin (boostables): scan des données: %v", err)
			continue
		}
		if len(images) > 0 {
			ad.AdImage = images[0]
		}
		boostableAds = append(boostableAds, ad)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Erreur admin (boostables): itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// --- Comptage total ---
	var totalAds int
	countQuery := `
		SELECT COUNT(*)
		FROM ads a
		WHERE
			a.is_validated = TRUE
			AND a.is_deactivated = FALSE
			AND a.is_rejected = FALSE
			AND a.is_sold = FALSE
			AND NOT EXISTS (
				SELECT 1
				FROM ad_boosts ab
				WHERE ab.ad_id = a.id
				AND ab.is_active = TRUE
				AND ab.end_date > NOW()
			)
	`
	err = config.DB.QueryRow(countQuery).Scan(&totalAds)
	if err != nil {
		log.Printf("Erreur admin (boostables): comptage total: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// --- Préparation de la réponse ---
	response := struct {
		BoostableAds []BoostableAdAdminView `json:"boostableAds"`
		Pagination   struct {
			CurrentPage  int `json:"currentPage"`
			TotalItems   int `json:"totalItems"`
			ItemsPerPage int `json:"itemsPerPage"`
			TotalPages   int `json:"totalPages"`
		} `json:"pagination"`
	}{
		BoostableAds: boostableAds,
	}
	response.Pagination.CurrentPage = page
	response.Pagination.TotalItems = totalAds
	response.Pagination.ItemsPerPage = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur admin (boostables): encodage JSON: %v", err)
	}
	log.Printf("Admin: %d annonces boostables récupérées avec succès.", len(boostableAds))
}

// CreateBoostAdminHandler permet à un admin de booster une annonce manuellement
func CreateBoostAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		AdID         int    `json:"ad_id"`
		BoostOfferID int    `json:"boost_offer_id"`
		Reason       string `json:"reason"` // Optionnel: "Offre commerciale", "Compensation", etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	if req.AdID <= 0 || req.BoostOfferID <= 0 {
		http.Error(w, "Les 'ad_id' et 'boost_offer_id' sont requis", http.StatusBadRequest)
		return
	}

	log.Printf("Admin: Tentative de boost manuel pour l'annonce ID: %d avec l'offre ID: %d", req.AdID, req.BoostOfferID)

	// Démarrer la transaction
	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur admin (boost manuel): début transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 1. Récupérer les détails de l'annonce (propriétaire, statut, titre)
	var adOwnerID int
	var adTitle string
	var isValidated bool
	err = tx.QueryRow("SELECT user_id, title, is_validated FROM ads WHERE id = $1", req.AdID).Scan(&adOwnerID, &adTitle, &isValidated)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur admin (boost manuel): récupération annonce %d: %v", req.AdID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	if !isValidated {
		http.Error(w, "L'annonce doit être validée avant de pouvoir être boostée", http.StatusBadRequest)
		return
	}

	// 2. Récupérer les détails de l'offre (durée, nom)
	var offerDurationDays int
	var offerName string
	err = tx.QueryRow("SELECT duration_days, name FROM boost_offers WHERE id = $1", req.BoostOfferID).Scan(&offerDurationDays, &offerName)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Offre de boost non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur admin (boost manuel): récupération offre %d: %v", req.BoostOfferID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// 3. Vérifier qu'il n'y a pas déjà un boost actif
	var activeBoostCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE ad_id = $1 AND is_active = TRUE AND end_date > NOW()", req.AdID).Scan(&activeBoostCount)
	if err != nil {
		log.Printf("Erreur admin (boost manuel): vérification boost actif pour annonce %d: %v", req.AdID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if activeBoostCount > 0 {
		http.Error(w, "Cette annonce est déjà activement boostée", http.StatusConflict)
		return
	}

	// 4. Créer le boost
	startDate := time.Now()
	endDate := startDate.AddDate(0, 0, offerDurationDays)

	transactionID := fmt.Sprintf("ADMIN_GRANT_%d", time.Now().UnixNano())
	if req.Reason != "" {
		transactionID = req.Reason // Utiliser la raison comme "transaction_id" pour info
	}

	var boostID int
	insertBoostQuery := `
		INSERT INTO ad_boosts 
		(ad_id, boost_offer_id, user_id, start_date, end_date, is_active, 
		 payment_status, payment_method, transaction_id, amount_paid)
		VALUES ($1, $2, $3, $4, $5, TRUE, 'admin_granted', 'admin', $6, 0.0)
		RETURNING id
	`
	err = tx.QueryRow(
		insertBoostQuery,
		req.AdID,
		req.BoostOfferID,
		adOwnerID,
		startDate,
		endDate,
		transactionID,
	).Scan(&boostID)

	if err != nil {
		log.Printf("Erreur admin (boost manuel): insertion du boost: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 5. Mettre à jour l'annonce
	_, err = tx.Exec("UPDATE ads SET is_boosted = TRUE, updated_at = NOW() WHERE id = $1", req.AdID)
	if err != nil {
		log.Printf("Erreur admin (boost manuel): mise à jour de l'annonce %d: %v", req.AdID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 6. Commit de la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur admin (boost manuel): commit transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 7. Créer une notification pour l'utilisateur
	// Note: 'createNotification' est défini dans 'boost_handler.go' mais est accessible
	// car les two fichiers sont dans le même package 'handlers'.
	go createNotification(
		adOwnerID,
		"boost_success",
		"Boost activé par un administrateur !",
		fmt.Sprintf("Votre annonce \"%s\" a été boostée par notre équipe avec l'offre %s pour %d jours.", adTitle, offerName, offerDurationDays),
		map[string]interface{}{
			"ad_id":         req.AdID,
			"ad_title":      adTitle,
			"boost_id":      boostID,
			"boost_name":    offerName,
			"duration_days": offerDurationDays,
			"start_date":    startDate.Format("2006-01-02"),
			"end_date":      endDate.Format("2006-01-02"),
		},
	)

	log.Printf("Admin: Boost manuel %d créé avec succès pour l'annonce %d.", boostID, req.AdID)

	// 8. Réponse
	response := map[string]interface{}{
		"message":  "Annonce boostée avec succès par l'administrateur",
		"boost_id": boostID,
		"ad_id":    req.AdID,
		"user_id":  adOwnerID,
		"end_date": endDate,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetAllBoostedAdsAdminHandler récupère toutes les annonces boostées pour le panneau d'administration avec pagination.
func GetAllBoostedAdsAdminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Admin: Début de la récupération de toutes les annonces boostées.")

	// --- Pagination ---
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 10 {
		limit = 10 // Limite par défaut
	}
	offset := (page - 1) * limit

	// --- Récupération des données ---
	var boostedAds []BoostedAdAdminView
	var totalBoosts int

	// Requête pour obtenir les annonces boostées
	query := `
		SELECT
			ab.id as boost_id,
			a.id as ad_id,
			a.title as ad_title,
			a.images as ad_images,
			u.id as user_id,
			CASE
				WHEN u.account_type = 'Professionnel' AND u.shop_name IS NOT NULL AND u.shop_name != '' THEN u.shop_name
				ELSE u.first_name || ' ' || u.last_name
			END as user_display_name,
			bo.name as offer_name,
			ab.amount_paid,
			ab.start_date,
			ab.end_date,
			ab.is_active,
			ab.payment_status,
			ab.transaction_id,
			ab.created_at
		FROM ad_boosts ab
		JOIN ads a ON ab.ad_id = a.id
		JOIN users u ON ab.user_id = u.id
		JOIN boost_offers bo ON ab.boost_offer_id = bo.id
		ORDER BY ab.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := config.DB.Query(query, limit, offset)
	if err != nil {
		log.Printf("Erreur admin (boosts): récupération des données: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ad BoostedAdAdminView
		var images pq.StringArray
		err := rows.Scan(
			&ad.BoostID, &ad.AdID, &ad.AdTitle, &images, &ad.UserID, &ad.UserDisplayName,
			&ad.OfferName, &ad.AmountPaid, &ad.StartDate, &ad.EndDate,
			&ad.IsActive, &ad.PaymentStatus, &ad.TransactionID, &ad.CreatedAt,
		)
		if err != nil {
			log.Printf("Erreur admin (boosts): scan des données: %v", err)
			continue
		}
		if len(images) > 0 {
			ad.AdImage = images[0]
		}
		boostedAds = append(boostedAds, ad)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Erreur admin (boosts): itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Compter le total des boosts
	err = config.DB.QueryRow("SELECT COUNT(*) FROM ad_boosts").Scan(&totalBoosts)
	if err != nil {
		log.Printf("Erreur admin (boosts): comptage total: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// --- Préparation de la réponse ---
	response := struct {
		BoostedAds []BoostedAdAdminView `json:"boostedAds"`
		Pagination struct {
			CurrentPage  int `json:"currentPage"`
			TotalItems   int `json:"totalItems"`
			ItemsPerPage int `json:"itemsPerPage"`
			TotalPages   int `json:"totalPages"`
		} `json:"pagination"`
	}{
		BoostedAds: boostedAds,
	}
	response.Pagination.CurrentPage = page
	response.Pagination.TotalItems = totalBoosts
	response.Pagination.ItemsPerPage = limit
	response.Pagination.TotalPages = (totalBoosts + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur admin (boosts): encodage JSON: %v", err)
	}
	log.Printf("Admin: %d annonces boostées récupérées avec succès.", len(boostedAds))
}

// DeactivateBoostAdminHandler permet à un administrateur de désactiver manuellement un boost.
func DeactivateBoostAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boostIDStr, ok := vars["boostID"]
	if !ok {
		http.Error(w, "ID de boost manquant", http.StatusBadRequest)
		return
	}
	boostID, err := strconv.Atoi(boostIDStr)
	if err != nil {
		http.Error(w, "ID de boost invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Admin: Tentative de désactivation du boost ID: %d", boostID)

	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur admin (désactivation boost): début transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 1. Désactiver le boost et récupérer l'ID de l'annonce associée
	var adID int
	err = tx.QueryRow("UPDATE ad_boosts SET is_active = FALSE WHERE id = $1 RETURNING ad_id", boostID).Scan(&adID)
	if err != nil {
		log.Printf("Erreur admin (désactivation boost): mise à jour du boost %d: %v", boostID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 2. Vérifier s'il existe d'autres boosts actifs pour la même annonce
	var activeBoostCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE ad_id = $1 AND is_active = TRUE AND end_date > NOW()", adID).Scan(&activeBoostCount)
	if err != nil {
		log.Printf("Erreur admin (désactivation boost): comptage des boosts actifs pour l'annonce %d: %v", adID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 3. S'il n'y a pas d'autres boosts actifs, mettre à jour le statut 'is_boosted' de l'annonce
	if activeBoostCount == 0 {
		_, err = tx.Exec("UPDATE ads SET is_boosted = FALSE WHERE id = $1", adID)
		if err != nil {
			log.Printf("Erreur admin (désactivation boost): mise à jour de l'annonce %d: %v", adID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}
		log.Printf("Admin: Statut 'is_boosted' de l'annonce %d mis à FALSE.", adID)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Erreur admin (désactivation boost): commit transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Admin: Boost %d désactivé avec succès.", boostID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Boost désactivé avec succès"})
}

// ============== GESTION DES OFFRES DE BOOST (ADMIN) ==============

// GetAllBoostOffersForAdminHandler récupère toutes les offres de boost (actives et inactives) pour l'admin
func GetAllBoostOffersForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := `
		SELECT 
			id, name, description, duration_days, price, 
			position_priority, features, color, is_active, 
			display_order, created_at, updated_at
		FROM boost_offers
		ORDER BY display_order ASC, created_at DESC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("Erreur admin: Récupération des offres de boost: %v", err)
		http.Error(w, "Erreur lors de la récupération des offres", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	offers := []models.BoostOffer{}

	for rows.Next() {
		var offer models.BoostOffer
		var featuresJSON []byte
		var description, color sql.NullString

		err := rows.Scan(
			&offer.ID,
			&offer.Name,
			&description,
			&offer.DurationDays,
			&offer.Price,
			&offer.PositionPriority,
			&featuresJSON,
			&color,
			&offer.IsActive,
			&offer.DisplayOrder,
			&offer.CreatedAt,
			&offer.UpdatedAt,
		)
		if err != nil {
			log.Printf("Erreur admin: Scan des offres de boost: %v", err)
			http.Error(w, "Erreur lors de la lecture des offres", http.StatusInternalServerError)
			return
		}

		// Gérer les champs NULL
		if description.Valid {
			offer.Description = description.String
		}
		if color.Valid {
			offer.Color = color.String
		}

		// Décoder les features JSON
		if len(featuresJSON) > 0 {
			if err := json.Unmarshal(featuresJSON, &offer.Features); err != nil {
				log.Printf("Avertissement: Erreur lors du décodage des features pour l'offre %d: %v", offer.ID, err)
			}
		}

		offers = append(offers, offer)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur admin: Itération des offres de boost: %v", err)
		http.Error(w, "Erreur lors de l'itération des offres", http.StatusInternalServerError)
		return
	}

	log.Printf("Récupération de %d offres de boost pour l'admin", len(offers))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"offers": offers,
		"total":  len(offers),
	})
}

// CreateBoostOfferHandler crée une nouvelle offre de boost
func CreateBoostOfferHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		DurationDays     int      `json:"duration_days"`
		Price            float64  `json:"price"`
		PositionPriority int      `json:"position_priority"`
		Features         []string `json:"features"`
		Color            string   `json:"color"`
		IsActive         bool     `json:"is_active"`
		DisplayOrder     int      `json:"display_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour création d'offre de boost: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Validation
	if req.Name == "" {
		http.Error(w, "Le nom de l'offre est requis", http.StatusBadRequest)
		return
	}
	if req.DurationDays <= 0 {
		http.Error(w, "La durée doit être supérieure à 0", http.StatusBadRequest)
		return
	}
	if req.Price < 0 {
		http.Error(w, "Le prix ne peut pas être négatif", http.StatusBadRequest)
		return
	}

	// Vérifier si une offre avec ce nom existe déjà
	var exists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM boost_offers WHERE name = $1)", req.Name).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de l'offre: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Une offre avec ce nom existe déjà", http.StatusConflict)
		return
	}

	// Convertir les features en JSON
	featuresJSON, err := json.Marshal(req.Features)
	if err != nil {
		log.Printf("Erreur admin: Marshalling des features: %v", err)
		http.Error(w, "Erreur lors du traitement des fonctionnalités", http.StatusInternalServerError)
		return
	}

	// Insertion
	var offerID int
	query := `
		INSERT INTO boost_offers 
		(name, description, duration_days, price, position_priority, features, color, is_active, display_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	err = config.DB.QueryRow(
		query,
		req.Name,
		req.Description,
		req.DurationDays,
		req.Price,
		req.PositionPriority,
		featuresJSON,
		req.Color,
		req.IsActive,
		req.DisplayOrder,
	).Scan(&offerID)

	if err != nil {
		log.Printf("Erreur admin: Insertion de l'offre de boost: %v", err)
		http.Error(w, "Erreur lors de la création de l'offre", http.StatusInternalServerError)
		return
	}

	log.Printf("Offre de boost créée avec succès: ID=%d, Name=%s", offerID, req.Name)

	response := map[string]interface{}{
		"message": "Offre de boost créée avec succès",
		"offer": map[string]interface{}{
			"id":                offerID,
			"name":              req.Name,
			"description":       req.Description,
			"duration_days":     req.DurationDays,
			"price":             req.Price,
			"position_priority": req.PositionPriority,
			"features":          req.Features,
			"color":             req.Color,
			"is_active":         req.IsActive,
			"display_order":     req.DisplayOrder,
		},
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateBoostOfferHandler modifie une offre de boost existante
func UpdateBoostOfferHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	offerID, err := strconv.Atoi(vars["offerID"])
	if err != nil {
		http.Error(w, "ID d'offre invalide", http.StatusBadRequest)
		return
	}

	var req struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		DurationDays     int      `json:"duration_days"`
		Price            float64  `json:"price"`
		PositionPriority int      `json:"position_priority"`
		Features         []string `json:"features"`
		Color            string   `json:"color"`
		IsActive         bool     `json:"is_active"`
		DisplayOrder     int      `json:"display_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour modification d'offre de boost: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Validation
	if req.Name == "" {
		http.Error(w, "Le nom de l'offre est requis", http.StatusBadRequest)
		return
	}
	if req.DurationDays <= 0 {
		http.Error(w, "La durée doit être supérieure à 0", http.StatusBadRequest)
		return
	}
	if req.Price < 0 {
		http.Error(w, "Le prix ne peut pas être négatif", http.StatusBadRequest)
		return
	}

	// Vérifier si une autre offre avec ce nom existe
	var exists bool
	err = config.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM boost_offers WHERE name = $1 AND id != $2)",
		req.Name, offerID,
	).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de l'offre: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Une offre avec ce nom existe déjà", http.StatusConflict)
		return
	}

	// Convertir les features en JSON
	featuresJSON, err := json.Marshal(req.Features)
	if err != nil {
		log.Printf("Erreur admin: Marshalling des features: %v", err)
		http.Error(w, "Erreur lors du traitement des fonctionnalités", http.StatusInternalServerError)
		return
	}

	// Mise à jour
	query := `
		UPDATE boost_offers 
		SET name = $1, description = $2, duration_days = $3, price = $4, 
		    position_priority = $5, features = $6, color = $7, is_active = $8, 
		    display_order = $9, updated_at = NOW()
		WHERE id = $10
	`
	result, err := config.DB.Exec(
		query,
		req.Name,
		req.Description,
		req.DurationDays,
		req.Price,
		req.PositionPriority,
		featuresJSON,
		req.Color,
		req.IsActive,
		req.DisplayOrder,
		offerID,
	)

	if err != nil {
		log.Printf("Erreur admin: Mise à jour de l'offre de boost %d: %v", offerID, err)
		http.Error(w, "Erreur lors de la mise à jour de l'offre", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Offre non trouvée", http.StatusNotFound)
		return
	}

	log.Printf("Offre de boost %d mise à jour avec succès", offerID)

	response := map[string]interface{}{
		"message": "Offre de boost mise à jour avec succès",
		"offer": map[string]interface{}{
			"id":                offerID,
			"name":              req.Name,
			"description":       req.Description,
			"duration_days":     req.DurationDays,
			"price":             req.Price,
			"position_priority": req.PositionPriority,
			"features":          req.Features,
			"color":             req.Color,
			"is_active":         req.IsActive,
			"display_order":     req.DisplayOrder,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ToggleBoostOfferStatusHandler active ou désactive une offre de boost
func ToggleBoostOfferStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	offerID, err := strconv.Atoi(vars["offerID"])
	if err != nil {
		http.Error(w, "ID d'offre invalide", http.StatusBadRequest)
		return
	}

	// Récupérer le statut actuel
	var currentStatus bool
	err = config.DB.QueryRow("SELECT is_active FROM boost_offers WHERE id = $1", offerID).Scan(&currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Offre non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur admin: Récupération du statut de l'offre %d: %v", offerID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// Inverser le statut
	newStatus := !currentStatus
	query := "UPDATE boost_offers SET is_active = $1, updated_at = NOW() WHERE id = $2"
	_, err = config.DB.Exec(query, newStatus, offerID)
	if err != nil {
		log.Printf("Erreur admin: Mise à jour du statut de l'offre %d: %v", offerID, err)
		http.Error(w, "Erreur lors de la mise à jour du statut", http.StatusInternalServerError)
		return
	}

	statusText := "désactivée"
	if newStatus {
		statusText = "activée"
	}

	log.Printf("Offre de boost %d %s avec succès", offerID, statusText)

	response := map[string]interface{}{
		"message":   fmt.Sprintf("Offre %s avec succès", statusText),
		"is_active": newStatus,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeleteBoostOfferHandler supprime une offre de boost
func DeleteBoostOfferHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	offerID, err := strconv.Atoi(vars["offerID"])
	if err != nil {
		http.Error(w, "ID d'offre invalide", http.StatusBadRequest)
		return
	}

	// Vérifier s'il y a des boosts actifs liés à cette offre
	var activeBoostCount int
	err = config.DB.QueryRow(
		"SELECT COUNT(*) FROM ad_boosts WHERE boost_offer_id = $1 AND is_active = true AND end_date > NOW()",
		offerID,
	).Scan(&activeBoostCount)
	if err != nil {
		log.Printf("Erreur admin: Vérification des boosts actifs pour l'offre %d: %v", offerID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	if activeBoostCount > 0 {
		http.Error(w, fmt.Sprintf("Impossible de supprimer l'offre: %d boost(s) actif(s) l'utilisent", activeBoostCount), http.StatusConflict)
		return
	}

	// Suppression
	query := "DELETE FROM boost_offers WHERE id = $1"
	result, err := config.DB.Exec(query, offerID)
	if err != nil {
		log.Printf("Erreur admin: Suppression de l'offre de boost %d: %v", offerID, err)
		http.Error(w, "Erreur lors de la suppression de l'offre", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Offre non trouvée", http.StatusNotFound)
		return
	}

	log.Printf("Offre de boost %d supprimée avec succès", offerID)

	response := map[string]string{
		"message": "Offre de boost supprimée avec succès",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetBoostOfferStatsHandler récupère les statistiques d'utilisation d'une offre
func GetBoostOfferStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	offerID, err := strconv.Atoi(vars["offerID"])
	if err != nil {
		http.Error(w, "ID d'offre invalide", http.StatusBadRequest)
		return
	}

	// Vérifier que l'offre existe
	var offerExists bool
	err = config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM boost_offers WHERE id = $1)", offerID).Scan(&offerExists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de l'offre %d: %v", offerID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if !offerExists {
		http.Error(w, "Offre non trouvée", http.StatusNotFound)
		return
	}

	// Récupérer les statistiques
	var stats struct {
		TotalPurchases  int     `json:"total_purchases"`
		ActiveBoosts    int     `json:"active_boosts"`
		CompletedBoosts int     `json:"completed_boosts"`
		TotalRevenue    float64 `json:"total_revenue"`
	}

	query := `
		SELECT 
			COUNT(*) as total_purchases,
			SUM(CASE WHEN is_active = true AND end_date > NOW() THEN 1 ELSE 0 END) as active_boosts,
			SUM(CASE WHEN end_date <= NOW() THEN 1 ELSE 0 END) as completed_boosts,
			COALESCE(SUM(amount_paid), 0) as total_revenue
		FROM ad_boosts
		WHERE offer_id = $1
	`

	err = config.DB.QueryRow(query, offerID).Scan(
		&stats.TotalPurchases,
		&stats.ActiveBoosts,
		&stats.CompletedBoosts,
		&stats.TotalRevenue,
	)
	if err != nil {
		log.Printf("Erreur admin: Récupération des statistiques de l'offre %d: %v", offerID, err)
		http.Error(w, "Erreur lors de la récupération des statistiques", http.StatusInternalServerError)
		return
	}

	log.Printf("Statistiques de l'offre %d récupérées avec succès", offerID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}
