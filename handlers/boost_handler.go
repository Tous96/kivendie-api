package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq"

	"kivendi-backend/config"
	"kivendi-backend/models"
)

// GetBoostOffersHandler récupère toutes les offres de boost disponibles
func GetBoostOffersHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des offres de boost.")

	query := `
		SELECT id, name, description, duration_days, price, position_priority, 
		       features, color, is_active, display_order, created_at, updated_at
		FROM boost_offers
		WHERE is_active = TRUE
		ORDER BY display_order ASC, price ASC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("Erreur lors de la récupération des offres de boost: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var offers []struct {
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

	for rows.Next() {
		var offer struct {
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
		var featuresJSON []byte

		err := rows.Scan(
			&offer.ID,
			&offer.Name,
			&offer.Description,
			&offer.DurationDays,
			&offer.Price,
			&offer.PositionPriority,
			&featuresJSON,
			&offer.Color,
			&offer.IsActive,
			&offer.DisplayOrder,
			&offer.CreatedAt,
			&offer.UpdatedAt,
		)
		if err != nil {
			log.Printf("Erreur lors du scan d'une offre de boost: %v", err)
			continue
		}

		if err := json.Unmarshal(featuresJSON, &offer.Features); err != nil {
			log.Printf("Erreur lors du parsing des features: %v", err)
			offer.Features = make(map[string]interface{})
		}

		offers = append(offers, offer)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des offres: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(offers); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
	}

	log.Printf("Récupération réussie de %d offres de boost.", len(offers))
}

// GetBoostOfferDetailsHandler récupère les détails d'une offre de boost spécifique
func GetBoostOfferDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	offerIDStr := vars["offerID"]
	offerID, err := strconv.Atoi(offerIDStr)
	if err != nil {
		http.Error(w, "ID d'offre invalide", http.StatusBadRequest)
		return
	}

	query := `
		SELECT id, name, description, duration_days, price, position_priority, 
		       features, color, is_active, display_order, created_at, updated_at
		FROM boost_offers
		WHERE id = $1 AND is_active = TRUE
	`

	var offer struct {
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
	var featuresJSON []byte

	err = config.DB.QueryRow(query, offerID).Scan(
		&offer.ID,
		&offer.Name,
		&offer.Description,
		&offer.DurationDays,
		&offer.Price,
		&offer.PositionPriority,
		&featuresJSON,
		&offer.Color,
		&offer.IsActive,
		&offer.DisplayOrder,
		&offer.CreatedAt,
		&offer.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Offre de boost non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la récupération de l'offre: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	if err := json.Unmarshal(featuresJSON, &offer.Features); err != nil {
		log.Printf("Erreur lors du parsing des features: %v", err)
		offer.Features = make(map[string]interface{})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(offer)
}

// Fonction helper pour créer une notification
func createNotification(userID int, notifType, title, message string, data map[string]interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Erreur lors de la sérialisation des données de notification: %v", err)
		dataJSON = []byte("{}")
	}

	query := `
		INSERT INTO notifications (user_id, type, title, message, data, is_read, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, false, NOW(), NOW())
	`
	_, err = config.DB.Exec(query, userID, notifType, title, message, dataJSON)
	if err != nil {
		log.Printf("Erreur lors de la création de la notification: %v", err)
		return err
	}
	log.Printf("✅ Notification créée pour l'utilisateur %d: %s", userID, title)
	return nil
}

// PurchaseBoostHandler gère l'achat d'un boost pour une annonce avec KKiaPay
func PurchaseBoostHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("=== DÉBUT PURCHASE BOOST ===")

	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	var req struct {
		BoostOfferID  int    `json:"boost_offer_id"`
		TransactionID string `json:"transaction_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur de décodage JSON: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	log.Printf("User ID: %d", userID)
	log.Printf("Ad ID: %d", adID)
	log.Printf("Boost Offer ID: %d", req.BoostOfferID)
	log.Printf("Transaction ID: %s", req.TransactionID)

	if req.TransactionID == "" {
		http.Error(w, "ID de transaction KKiaPay manquant", http.StatusBadRequest)
		return
	}

	// Vérifier la propriété de l'annonce
	var ownerID int
	var isValidated bool
	var adTitle string
	err = config.DB.QueryRow("SELECT user_id, is_validated, title FROM ads WHERE id = $1", adID).Scan(&ownerID, &isValidated, &adTitle)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la vérification de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	if ownerID != userID {
		http.Error(w, "Vous n'êtes pas autorisé à booster cette annonce", http.StatusForbidden)
		return
	}

	if !isValidated {
		http.Error(w, "L'annonce doit être validée avant de pouvoir être boostée", http.StatusBadRequest)
		return
	}

	log.Println("✅ Propriété de l'annonce vérifiée")

	// Récupérer les détails de l'offre
	var offerPrice float64
	var offerDurationDays int
	var offerPriority int
	var offerName string
	err = config.DB.QueryRow(`
		SELECT price, duration_days, position_priority, name
		FROM boost_offers 
		WHERE id = $1 AND is_active = TRUE
	`, req.BoostOfferID).Scan(&offerPrice, &offerDurationDays, &offerPriority, &offerName)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Offre de boost non trouvée ou inactive", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la récupération de l'offre: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("✅ Offre récupérée: %.2f FCFA pour %d jours", offerPrice, offerDurationDays)

	// Vérifier la transaction KKiaPay
	log.Printf("=== VÉRIFICATION KKIAPAY ===")
	log.Printf("Transaction ID à vérifier: %s", req.TransactionID)
	log.Printf("Montant attendu: %.2f FCFA", offerPrice)
	log.Printf("Mode: %s", map[bool]string{true: "SANDBOX", false: "PRODUCTION"}[config.KKiaPay.IsSandbox])

	transaction, err := config.KKiaPay.VerifyTransaction(req.TransactionID)
	if err != nil {
		log.Printf("❌ Erreur vérification KKiaPay: %v", err)

		// Créer une notification d'échec
		createNotification(
			userID,
			"boost_failure",
			"Échec du boost",
			fmt.Sprintf("Le paiement pour booster votre annonce \"%s\" n'a pas pu être vérifié. Veuillez réessayer.", adTitle),
			map[string]interface{}{
				"ad_id":          adID,
				"ad_title":       adTitle,
				"transaction_id": req.TransactionID,
				"error":          err.Error(),
			},
		)

		http.Error(w, fmt.Sprintf("Échec de la vérification du paiement: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("✅ Transaction KKiaPay récupérée:")
	log.Printf("  - Status: %s", transaction.Status)
	log.Printf("  - State: %s", transaction.State)
	log.Printf("  - Amount: %.2f", transaction.Amount)

	// Vérifier que la transaction est réussie
	if transaction.Status != "SUCCESS" || transaction.State != "RECEIVED" {
		log.Printf("❌ Transaction non valide: Status=%s, State=%s", transaction.Status, transaction.State)

		// Créer une notification d'échec
		createNotification(
			userID,
			"boost_failure",
			"Paiement non validé",
			fmt.Sprintf("Le paiement pour booster votre annonce \"%s\" n'a pas été validé par KKiaPay. Statut: %s", adTitle, transaction.Status),
			map[string]interface{}{
				"ad_id":          adID,
				"ad_title":       adTitle,
				"transaction_id": req.TransactionID,
				"status":         transaction.Status,
				"state":          transaction.State,
			},
		)

		http.Error(w, "Le paiement n'a pas été validé par KKiaPay", http.StatusPaymentRequired)
		return
	}

	log.Println("✅ Transaction confirmée avec succès")

	// Vérifier le montant (sauf en sandbox où l'API peut retourner 0)
	if config.KKiaPay.IsSandbox {
		log.Println("🧪 MODE SANDBOX: Vérification du montant ignorée")
		log.Printf("   (Montant attendu: %.2f FCFA, montant reçu: %.2f FCFA)", offerPrice, transaction.Amount)
	} else {
		// En production, vérification stricte du montant
		amountDiff := transaction.Amount - offerPrice
		if amountDiff < -0.01 || amountDiff > 0.01 {
			log.Printf("❌ Montant incorrect:")
			log.Printf("  - Attendu: %.2f FCFA", offerPrice)
			log.Printf("  - Reçu: %.2f FCFA", transaction.Amount)
			log.Printf("  - Différence: %.2f FCFA", amountDiff)

			// Créer une notification d'échec
			createNotification(
				userID,
				"boost_failure",
				"Montant incorrect",
				fmt.Sprintf("Le montant du paiement (%.0f FCFA) ne correspond pas au prix de l'offre (%.0f FCFA) pour booster \"%s\".", transaction.Amount, offerPrice, adTitle),
				map[string]interface{}{
					"ad_id":           adID,
					"ad_title":        adTitle,
					"transaction_id":  req.TransactionID,
					"expected_amount": offerPrice,
					"received_amount": transaction.Amount,
				},
			)

			http.Error(w, "Le montant du paiement ne correspond pas au prix de l'offre", http.StatusBadRequest)
			return
		}
		log.Printf("✅ Montant validé: %.2f FCFA", transaction.Amount)
	}

	// Vérifier que la transaction n'a pas déjà été utilisée
	log.Printf("Vérification unicité de la transaction...")
	var existingTransactionID int
	err = config.DB.QueryRow("SELECT id FROM kkiapay_transactions WHERE transaction_id = $1", req.TransactionID).Scan(&existingTransactionID)
	if err != sql.ErrNoRows {
		if err == nil {
			log.Printf("❌ Transaction déjà enregistrée: id=%d", existingTransactionID)

			// Créer une notification d'échec
			createNotification(
				userID,
				"boost_failure",
				"Transaction déjà utilisée",
				fmt.Sprintf("Cette transaction a déjà été utilisée. Votre annonce \"%s\" ne peut pas être boostée avec cette transaction.", adTitle),
				map[string]interface{}{
					"ad_id":          adID,
					"ad_title":       adTitle,
					"transaction_id": req.TransactionID,
				},
			)

			http.Error(w, "Cette transaction a déjà été utilisée", http.StatusConflict)
		} else {
			log.Printf("❌ Erreur vérification transaction: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("✅ Transaction unique, création du boost...")

	// Convertir la réponse KKiaPay en JSONB pour stockage
	rawResponseJSON, err := json.Marshal(transaction)
	if err != nil {
		log.Printf("⚠️ Erreur lors de la sérialisation de la réponse KKiaPay: %v", err)
		// On continue quand même, ce n'est pas critique
		rawResponseJSON = []byte("{}")
	}

	// Commencer une transaction DB
	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur lors du début de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	startDate := time.Now()
	endDate := startDate.AddDate(0, 0, offerDurationDays)

	// Insérer le boost
	var boostID int
	insertBoostQuery := `
		INSERT INTO ad_boosts 
		(ad_id, boost_offer_id, user_id, start_date, end_date, is_active, 
		 payment_status, payment_method, transaction_id, amount_paid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err = tx.QueryRow(
		insertBoostQuery,
		adID,
		req.BoostOfferID,
		userID,
		startDate,
		endDate,
		true,
		"completed",
		"kkiapay",
		req.TransactionID,
		offerPrice,
	).Scan(&boostID)

	if err != nil {
		log.Printf("Erreur lors de l'insertion du boost: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Boost inséré avec ID: %d", boostID)

	// Enregistrer la transaction KKiaPay
	// En mode sandbox, KKiaPay retourne souvent Amount=0, donc on utilise le prix réel de l'offre
	amountToStore := transaction.Amount
	if config.KKiaPay.IsSandbox && transaction.Amount == 0 {
		amountToStore = offerPrice
		log.Printf("🧪 MODE SANDBOX: Utilisation du prix de l'offre (%.2f FCFA) au lieu du montant KKiaPay (0)", offerPrice)
	}

	insertTransactionQuery := `
		INSERT INTO kkiapay_transactions 
		(transaction_id, boost_id, ad_id, user_id, amount, status, state, raw_response, verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`

	_, err = tx.Exec(
		insertTransactionQuery,
		req.TransactionID,
		boostID,
		adID,
		userID,
		amountToStore,
		transaction.Status,
		transaction.State,
		rawResponseJSON,
		time.Now(),
	)

	if err != nil {
		log.Printf("Erreur lors de l'insertion de la transaction KKiaPay: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Transaction KKiaPay enregistrée: %s", req.TransactionID)

	// Mettre à jour l'annonce
	updateQuery := `UPDATE ads SET is_boosted = TRUE, updated_at = NOW() WHERE id = $1`
	_, err = tx.Exec(updateQuery, adID)
	if err != nil {
		log.Printf("Erreur lors de la mise à jour de l'annonce: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Annonce mise à jour (is_boosted=TRUE)")

	// Valider la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur lors de la validation de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ SUCCÈS: Boost %d créé avec succès pour l'annonce %d par l'utilisateur %d", boostID, adID, userID)
	log.Printf("   Transaction KKiaPay: %s", req.TransactionID)
	log.Printf("   Montant: %.2f FCFA", offerPrice)
	log.Printf("   Durée: %d jours (du %s au %s)", offerDurationDays, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Créer une notification de succès
	createNotification(
		userID,
		"boost_success",
		"Boost activé avec succès !",
		fmt.Sprintf("Votre annonce \"%s\" a été boostée avec succès avec l'offre %s pour %d jours.", adTitle, offerName, offerDurationDays),
		map[string]interface{}{
			"ad_id":          adID,
			"ad_title":       adTitle,
			"boost_id":       boostID,
			"boost_name":     offerName,
			"duration_days":  offerDurationDays,
			"amount_paid":    offerPrice,
			"start_date":     startDate.Format("2006-01-02"),
			"end_date":       endDate.Format("2006-01-02"),
			"transaction_id": req.TransactionID,
		},
	)

	response := struct {
		Message   string    `json:"message"`
		BoostID   int       `json:"boost_id"`
		AdID      int       `json:"ad_id"`
		StartDate time.Time `json:"start_date"`
		EndDate   time.Time `json:"end_date"`
	}{
		Message:   "Boost activé avec succès",
		BoostID:   boostID,
		AdID:      adID,
		StartDate: startDate,
		EndDate:   endDate,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetBoostedAdsHandler récupère toutes les annonces boostées actives
func GetBoostedAdsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des annonces boostées.")

	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	query := `
		SELECT 
			a.id, a.title, a.description, a.price, a.images, a.city, 
			a.phone_number, a.is_phone_visible, a.is_delivery_available,
			a.latitude, a.longitude, a.created_at,
			u.id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
			sc.name as sub_category_name, c.name as category_name,
			ab.end_date, bo.position_priority
		FROM ads a
		JOIN ad_boosts ab ON a.id = ab.ad_id
		JOIN boost_offers bo ON ab.boost_offer_id = bo.id
		JOIN users u ON a.user_id = u.id
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.is_validated = TRUE 
		AND a.is_deactivated = FALSE 
		AND a.is_rejected = FALSE
		AND a.is_sold = FALSE
		AND a.is_boosted = TRUE
		AND ab.is_active = TRUE
		AND ab.end_date > NOW()
		ORDER BY bo.position_priority DESC, a.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := config.DB.Query(query, limit, offset)
	if err != nil {
		log.Printf("Erreur lors de la récupération des annonces boostées: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []models.Ad

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var shopName, avatarURL sql.NullString
		var latitude, longitude sql.NullFloat64
		var firstName, lastName, accountType string
		var subCategoryName, categoryName string
		var endDate time.Time
		var priority int

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &ad.City,
			&ad.PhoneNumber, &ad.IsPhoneVisible, &ad.IsDeliveryAvailable,
			&latitude, &longitude, &ad.CreatedAt,
			&ad.User.ID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
			&subCategoryName, &categoryName,
			&endDate, &priority,
		)
		if err != nil {
			log.Printf("Erreur lors du scan d'une annonce boostée: %v", err)
			continue
		}

		ad.Images = []string(images)
		if latitude.Valid {
			ad.Latitude = latitude
		}
		if longitude.Valid {
			ad.Longitude = longitude
		}

		ad.SubCategoryName = subCategoryName
		ad.CategoryName = categoryName
		ad.User.FirstName = firstName
		ad.User.LastName = lastName
		ad.User.ShopName = shopName
		ad.User.AvatarURL = avatarURL
		ad.User.IsProAccount = accountType == "Professionnel"

		if ad.User.IsProAccount && shopName.Valid {
			ad.User.DisplayName = shopName.String
		} else {
			ad.User.DisplayName = fmt.Sprintf("%s %s", firstName, lastName)
		}

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des annonces boostées: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	var totalAds int
	countQuery := `
		SELECT COUNT(*)
		FROM ads a
		JOIN ad_boosts ab ON a.id = ab.ad_id
		WHERE a.is_validated = TRUE 
		AND a.is_deactivated = FALSE 
		AND a.is_rejected = FALSE
		AND a.is_sold = FALSE
		AND a.is_boosted = TRUE
		AND ab.is_active = TRUE
		AND ab.end_date > NOW()
	`
	err = config.DB.QueryRow(countQuery).Scan(&totalAds)
	if err != nil {
		log.Printf("Erreur lors du comptage des annonces boostées: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	response := struct {
		Ads        []models.Ad `json:"ads"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
	}{
		Ads: ads,
	}

	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("Récupération réussie de %d annonces boostées sur %d total.", len(ads), totalAds)
}

// GetUserBoostHistoryHandler récupère l'historique des boosts d'un utilisateur
func GetUserBoostHistoryHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération de l'historique des boosts.")

	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	query := `
		SELECT 
			ab.id, ab.ad_id, ab.start_date, ab.end_date, ab.is_active,
			ab.payment_status, ab.payment_method, ab.amount_paid, ab.created_at,
			a.title, a.images,
			bo.name as offer_name, bo.duration_days, bo.color
		FROM ad_boosts ab
		JOIN ads a ON ab.ad_id = a.id
		JOIN boost_offers bo ON ab.boost_offer_id = bo.id
		WHERE ab.user_id = $1
		ORDER BY ab.created_at DESC
	`

	rows, err := config.DB.Query(query, userID)
	if err != nil {
		log.Printf("Erreur lors de la récupération de l'historique: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []struct {
		ID            int       `json:"id"`
		AdID          int       `json:"ad_id"`
		AdTitle       string    `json:"ad_title"`
		AdImage       string    `json:"ad_image"`
		StartDate     time.Time `json:"start_date"`
		EndDate       time.Time `json:"end_date"`
		IsActive      bool      `json:"is_active"`
		PaymentStatus string    `json:"payment_status"`
		PaymentMethod string    `json:"payment_method"`
		AmountPaid    float64   `json:"amount_paid"`
		CreatedAt     time.Time `json:"created_at"`
		OfferName     string    `json:"offer_name"`
		DurationDays  int       `json:"duration_days"`
		OfferColor    string    `json:"offer_color"`
	}

	for rows.Next() {
		var item struct {
			ID            int       `json:"id"`
			AdID          int       `json:"ad_id"`
			AdTitle       string    `json:"ad_title"`
			AdImage       string    `json:"ad_image"`
			StartDate     time.Time `json:"start_date"`
			EndDate       time.Time `json:"end_date"`
			IsActive      bool      `json:"is_active"`
			PaymentStatus string    `json:"payment_status"`
			PaymentMethod string    `json:"payment_method"`
			AmountPaid    float64   `json:"amount_paid"`
			CreatedAt     time.Time `json:"created_at"`
			OfferName     string    `json:"offer_name"`
			DurationDays  int       `json:"duration_days"`
			OfferColor    string    `json:"offer_color"`
		}
		var images pq.StringArray

		err := rows.Scan(
			&item.ID, &item.AdID, &item.StartDate, &item.EndDate, &item.IsActive,
			&item.PaymentStatus, &item.PaymentMethod, &item.AmountPaid, &item.CreatedAt,
			&item.AdTitle, &images,
			&item.OfferName, &item.DurationDays, &item.OfferColor,
		)
		if err != nil {
			log.Printf("Erreur lors du scan de l'historique: %v", err)
			continue
		}

		if len(images) > 0 {
			item.AdImage = images[0]
		}

		history = append(history, item)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération de l'historique: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)

	log.Printf("Historique des boosts récupéré avec succès pour l'utilisateur %d", userID)
}

// CheckAdBoostStatusHandler vérifie si une annonce est actuellement boostée
func CheckAdBoostStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	query := `
		SELECT 
			ab.id, ab.end_date, ab.is_active,
			bo.name, bo.color, bo.position_priority
		FROM ad_boosts ab
		JOIN boost_offers bo ON ab.boost_offer_id = bo.id
		WHERE ab.ad_id = $1 
		AND ab.is_active = TRUE 
		AND ab.end_date > NOW()
		ORDER BY ab.created_at DESC
		LIMIT 1
	`

	var response struct {
		IsBoosted        bool       `json:"is_boosted"`
		BoostID          *int       `json:"boost_id,omitempty"`
		EndDate          *time.Time `json:"end_date,omitempty"`
		OfferName        *string    `json:"offer_name,omitempty"`
		OfferColor       *string    `json:"offer_color,omitempty"`
		PositionPriority *int       `json:"position_priority,omitempty"`
	}

	var boostID int
	var endDate time.Time
	var offerName, offerColor string
	var priority int

	err = config.DB.QueryRow(query, adID).Scan(
		&boostID, &endDate, &response.IsBoosted,
		&offerName, &offerColor, &priority,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			response.IsBoosted = false
		} else {
			log.Printf("Erreur lors de la vérification du statut de boost: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}
	} else {
		response.IsBoosted = true
		response.BoostID = &boostID
		response.EndDate = &endDate
		response.OfferName = &offerName
		response.OfferColor = &offerColor
		response.PositionPriority = &priority
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
