package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"kivendi-backend/config"
	"kivendi-backend/models"
	"kivendi-backend/services"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

// GetAllUsersHandler récupère tous les utilisateurs pour le panel admin avec pagination et filtres.
func GetAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des utilisateurs pour le panel admin.")

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

	// --- Filtres et Recherche ---
	query := r.URL.Query().Get("query")
	status := r.URL.Query().Get("status")
	userType := r.URL.Query().Get("type")

	var whereClauses []string
	var args []interface{}
	argID := 1

	if query != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d OR email ILIKE $%d OR shop_name ILIKE $%d)", argID, argID, argID, argID))
		args = append(args, "%"+query+"%")
		argID++
	}

	switch status {
	case "verified":
		whereClauses = append(whereClauses, "is_verified = TRUE AND is_blocked = FALSE")
	case "unverified":
		whereClauses = append(whereClauses, "is_verified = FALSE AND is_blocked = FALSE")
	case "blocked":
		whereClauses = append(whereClauses, "is_blocked = TRUE")
	}

	if userType == "Personnel" || userType == "Professionnel" {
		whereClauses = append(whereClauses, fmt.Sprintf("account_type = $%d", argID))
		args = append(args, userType)
		argID++
	}

	whereQuery := ""
	if len(whereClauses) > 0 {
		whereQuery = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// --- Récupération des données ---
	var users []models.UserResponse // ✅ Changé de []models.User
	var totalUsers int

	// 1. Compter le total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereQuery)
	err = config.DB.QueryRow(countQuery, args...).Scan(&totalUsers)
	if err != nil {
		log.Printf("Erreur lors du comptage des utilisateurs admin: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 2. Récupérer les utilisateurs
	limitOffsetQuery := fmt.Sprintf("ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	dataQuery := fmt.Sprintf(`
		SELECT id, first_name, last_name, email, account_type, 
		       shop_name, avatar_url, is_verified, is_blocked, created_at, updated_at
		FROM users
		%s
		%s
	`, whereQuery, limitOffsetQuery)

	rows, err := config.DB.Query(dataQuery, args...)
	if err != nil {
		log.Printf("Erreur lors de la récupération des utilisateurs admin: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.AccountType,
			&user.ShopName, &user.AvatarURL, &user.IsVerified, &user.IsBlocked,
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			log.Printf("Erreur lors du scan d'un utilisateur admin: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}

		// Calculer les champs dérivés
		user.ComputeFields()

		// ✅ Convertir en UserResponse pour envoyer au frontend
		users = append(users, user.ToResponse())
	}
	if err = rows.Err(); err != nil {
		log.Printf("Erreur lors de l'itération des utilisateurs admin: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// --- Préparation de la réponse ---
	response := struct {
		Users      []models.UserResponse `json:"users"` // ✅ Changé de []models.User
		Pagination struct {
			CurrentPage  int `json:"currentPage"`
			TotalItems   int `json:"totalItems"`
			ItemsPerPage int `json:"itemsPerPage"`
			TotalPages   int `json:"totalPages"`
		} `json:"pagination"`
	}{
		Users: users,
	}
	response.Pagination.CurrentPage = page
	response.Pagination.TotalItems = totalUsers
	response.Pagination.ItemsPerPage = limit
	response.Pagination.TotalPages = (totalUsers + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
	}
	log.Println("Utilisateurs pour le panel admin récupérés avec succès.")
}

// AdminUpdateUserRequest définit la structure pour la mise à jour d'un utilisateur par un admin.
type AdminUpdateUserRequest struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	AccountType string `json:"account_type"`
	ShopName    string `json:"shop_name"`
	IsVerified  bool   `json:"is_verified"`
}

// UpdateUserForAdminHandler gère la modification des informations d'un utilisateur par un admin.
func UpdateUserForAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["userID"])
	if err != nil {
		http.Error(w, "ID utilisateur invalide", http.StatusBadRequest)
		return
	}

	var req AdminUpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour MAJ utilisateur %d: %v", userID, err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	if req.FirstName == "" || req.LastName == "" || req.Email == "" {
		http.Error(w, "Prénom, nom et email sont requis", http.StatusBadRequest)
		return
	}

	if req.AccountType != "Personnel" && req.AccountType != "Professionnel" {
		log.Printf("Erreur admin: Type de compte invalide pour utilisateur %d: %s", userID, req.AccountType)
		http.Error(w, "Type de compte invalide. Doit être 'Personnel' ou 'Professionnel'", http.StatusBadRequest)
		return
	}

	if req.AccountType == "Professionnel" && strings.TrimSpace(req.ShopName) == "" {
		http.Error(w, "Le nom de la boutique est obligatoire pour un compte Professionnel", http.StatusBadRequest)
		return
	}

	var shopName sql.NullString
	if req.AccountType == "Professionnel" {
		shopName = sql.NullString{String: strings.TrimSpace(req.ShopName), Valid: true}
	}

	var exists bool
	err = config.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id != $2)",
		req.Email, userID,
	).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification email pour MAJ utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Cet email est déjà utilisé par un autre compte", http.StatusConflict)
		return
	}

	query := `
		UPDATE users 
		SET first_name = $1, last_name = $2, email = $3, account_type = $4, 
		    shop_name = $5, is_verified = $6, updated_at = NOW()
		WHERE id = $7
	`
	result, err := config.DB.Exec(query,
		req.FirstName, req.LastName, req.Email, req.AccountType,
		shopName, req.IsVerified, userID,
	)
	if err != nil {
		log.Printf("Erreur admin: MAJ utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur lors de la mise à jour de l'utilisateur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		return
	}

	log.Printf("Utilisateur %d mis à jour avec succès par l'admin (type: %s, shop: %v)",
		userID, req.AccountType, shopName.Valid)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Utilisateur mis à jour avec succès",
	})
}

// ToggleUserBlockStatusHandler (dé)bloque un utilisateur.
func ToggleUserBlockStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["userID"])
	if err != nil {
		http.Error(w, "ID utilisateur invalide", http.StatusBadRequest)
		return
	}

	var isBlocked bool
	query := `
		UPDATE users 
		SET is_blocked = NOT is_blocked, updated_at = NOW() 
		WHERE id = $1
		RETURNING is_blocked
	`
	err = config.DB.QueryRow(query, userID).Scan(&isBlocked)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		} else {
			log.Printf("Erreur admin: Toggle block utilisateur %d: %v", userID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	var message string
	if isBlocked {
		message = "Utilisateur bloqué avec succès"
		log.Printf("Utilisateur %d bloqué par l'admin.", userID)
	} else {
		message = "Utilisateur débloqué avec succès"
		log.Printf("Utilisateur %d débloqué par l'admin.", userID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   message,
		"isBlocked": isBlocked,
	})
}

// GetUserDetailsForAdminHandler récupère les détails complets d'un utilisateur avec toutes ses statistiques.
func GetUserDetailsForAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["userID"])
	if err != nil {
		http.Error(w, "ID utilisateur invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Admin demande les détails de l'utilisateur ID: %d", userID)

	var user models.User
	query := `
		SELECT id, first_name, last_name, email, account_type, 
		       shop_name, avatar_url, is_verified, is_blocked, 
		       created_at, updated_at
		FROM users
		WHERE id = $1
	`
	err = config.DB.QueryRow(query, userID).Scan(
		&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.AccountType,
		&user.ShopName, &user.AvatarURL, &user.IsVerified, &user.IsBlocked,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
			return
		}
		log.Printf("Erreur admin: Récupération utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	user.ComputeFields()

	// Statistiques des annonces
	var adsStats struct {
		TotalAds       int `json:"totalAds"`
		ValidatedAds   int `json:"validatedAds"`
		PendingAds     int `json:"pendingAds"`
		RejectedAds    int `json:"rejectedAds"`
		DeactivatedAds int `json:"deactivatedAds"`
		SoldAds        int `json:"soldAds"`
		ActiveAds      int `json:"activeAds"`
	}

	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1", userID).Scan(&adsStats.TotalAds)
	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_validated = TRUE AND is_rejected = FALSE", userID).Scan(&adsStats.ValidatedAds)
	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_validated = FALSE AND is_rejected = FALSE AND is_deactivated = FALSE", userID).Scan(&adsStats.PendingAds)
	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_rejected = TRUE", userID).Scan(&adsStats.RejectedAds)
	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_deactivated = TRUE", userID).Scan(&adsStats.DeactivatedAds)
	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_sold = TRUE", userID).Scan(&adsStats.SoldAds)
	config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_validated = TRUE AND is_sold = FALSE AND is_deactivated = FALSE", userID).Scan(&adsStats.ActiveAds)

	// Statistiques des boosts
	var boostStats struct {
		TotalBoosts      int     `json:"totalBoosts"`
		ActiveBoosts     int     `json:"activeBoosts"`
		CompletedBoosts  int     `json:"completedBoosts"`
		TotalAmountSpent float64 `json:"totalAmountSpent"`
		PendingPayments  int     `json:"pendingPayments"`
	}

	config.DB.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE user_id = $1", userID).Scan(&boostStats.TotalBoosts)
	config.DB.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE user_id = $1 AND is_active = TRUE AND end_date > NOW()", userID).Scan(&boostStats.ActiveBoosts)
	config.DB.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE user_id = $1 AND (is_active = FALSE OR end_date <= NOW()) AND payment_status = 'completed'", userID).Scan(&boostStats.CompletedBoosts)
	config.DB.QueryRow("SELECT COALESCE(SUM(amount_paid), 0) FROM ad_boosts WHERE user_id = $1 AND payment_status = 'completed'", userID).Scan(&boostStats.TotalAmountSpent)
	config.DB.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE user_id = $1 AND payment_status = 'pending'", userID).Scan(&boostStats.PendingPayments)

	// Liste des boosts actifs
	type ActiveBoost struct {
		ID         int     `json:"id"`
		AdID       int     `json:"adId"`
		AdTitle    string  `json:"adTitle"`
		OfferName  string  `json:"offerName"`
		StartDate  string  `json:"startDate"`
		EndDate    string  `json:"endDate"`
		AmountPaid float64 `json:"amountPaid"`
	}

	var activeBoosts []ActiveBoost
	activeBoostsQuery := `
		SELECT ab.id, ab.ad_id, a.title, bo.name, ab.start_date, ab.end_date, ab.amount_paid
		FROM ad_boosts ab
		JOIN ads a ON ab.ad_id = a.id
		JOIN boost_offers bo ON ab.boost_offer_id = bo.id
		WHERE ab.user_id = $1 AND ab.is_active = TRUE AND ab.end_date > NOW()
		ORDER BY ab.end_date DESC
	`
	rows, err := config.DB.Query(activeBoostsQuery, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var boost ActiveBoost
			var startDate, endDate sql.NullTime
			if err := rows.Scan(&boost.ID, &boost.AdID, &boost.AdTitle, &boost.OfferName, &startDate, &endDate, &boost.AmountPaid); err == nil {
				if startDate.Valid {
					boost.StartDate = startDate.Time.Format("2006-01-02 15:04:05")
				}
				if endDate.Valid {
					boost.EndDate = endDate.Time.Format("2006-01-02 15:04:05")
				}
				activeBoosts = append(activeBoosts, boost)
			}
		}
	}

	// Statistiques des transactions
	var transactionStats struct {
		TotalTransactions int     `json:"totalTransactions"`
		SuccessfulTxns    int     `json:"successfulTransactions"`
		FailedTxns        int     `json:"failedTransactions"`
		PendingTxns       int     `json:"pendingTransactions"`
		TotalAmount       float64 `json:"totalAmount"`
	}

	config.DB.QueryRow("SELECT COUNT(*) FROM kkiapay_transactions WHERE user_id = $1", userID).Scan(&transactionStats.TotalTransactions)
	config.DB.QueryRow("SELECT COUNT(*) FROM kkiapay_transactions WHERE user_id = $1 AND status = 'SUCCESS'", userID).Scan(&transactionStats.SuccessfulTxns)
	config.DB.QueryRow("SELECT COUNT(*) FROM kkiapay_transactions WHERE user_id = $1 AND status = 'FAILED'", userID).Scan(&transactionStats.FailedTxns)
	config.DB.QueryRow("SELECT COUNT(*) FROM kkiapay_transactions WHERE user_id = $1 AND status = 'PENDING'", userID).Scan(&transactionStats.PendingTxns)
	config.DB.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM kkiapay_transactions WHERE user_id = $1 AND status = 'SUCCESS'", userID).Scan(&transactionStats.TotalAmount)

	// Autres statistiques
	var otherStats struct {
		TotalFavorites     int `json:"totalFavorites"`
		TotalConversations int `json:"totalConversations"`
		BlockedUsers       int `json:"blockedUsers"`
		ReportsReceived    int `json:"reportsReceived"`
		ReportsMade        int `json:"reportsMade"`
	}

	config.DB.QueryRow("SELECT COUNT(*) FROM favorites WHERE user_id = $1", userID).Scan(&otherStats.TotalFavorites)
	config.DB.QueryRow("SELECT COUNT(*) FROM conversations WHERE seller_id = $1 OR buyer_id = $1", userID).Scan(&otherStats.TotalConversations)
	config.DB.QueryRow("SELECT COUNT(*) FROM user_blocks WHERE blocker_id = $1", userID).Scan(&otherStats.BlockedUsers)
	config.DB.QueryRow("SELECT COUNT(*) FROM user_reports WHERE reported_id = $1", userID).Scan(&otherStats.ReportsReceived)
	config.DB.QueryRow("SELECT COUNT(*) FROM user_reports WHERE reporter_id = $1", userID).Scan(&otherStats.ReportsMade)

	// Construction de la réponse
	response := struct {
		User             models.UserResponse `json:"user"` // ✅ Changé de models.User
		AdsStats         interface{}         `json:"adsStats"`
		BoostStats       interface{}         `json:"boostStats"`
		ActiveBoosts     []ActiveBoost       `json:"activeBoosts"`
		TransactionStats interface{}         `json:"transactionStats"`
		OtherStats       interface{}         `json:"otherStats"`
	}{
		User:             user.ToResponse(), // ✅ Ajout de .ToResponse()
		AdsStats:         adsStats,
		BoostStats:       boostStats,
		ActiveBoosts:     activeBoosts,
		TransactionStats: transactionStats,
		OtherStats:       otherStats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur encodage réponse détails utilisateur %d: %v", userID, err)
	}

	log.Printf("Détails utilisateur %d récupérés avec succès par l'admin.", userID)
}

// DeleteUserForAdminHandler supprime un utilisateur et toutes ses données associées.
func DeleteUserForAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["userID"])
	if err != nil {
		http.Error(w, "ID utilisateur invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Admin initie la suppression de l'utilisateur ID: %d", userID)

	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur admin: Début de transaction pour suppression utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var avatarURL sql.NullString
	err = tx.QueryRow("SELECT avatar_url FROM users WHERE id = $1", userID).Scan(&avatarURL)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
			return
		}
		log.Printf("Erreur admin: Récupération avatar pour suppression utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	var allAdImages []string
	rows, err := tx.Query("SELECT images FROM ads WHERE user_id = $1", userID)
	if err != nil {
		log.Printf("Erreur admin: Récupération images d'annonces pour suppression utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var images pq.StringArray
		if err := rows.Scan(&images); err != nil {
			log.Printf("Erreur admin: Scan images d'annonces pour suppression utilisateur %d: %v", userID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}
		allAdImages = append(allAdImages, []string(images)...)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Erreur admin: Itération images d'annonces pour suppression utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	result, err := tx.Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		log.Printf("Erreur admin: Suppression utilisateur %d: %v", userID, err)
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			log.Println("ERREUR: La suppression a échoué. Assurez-vous que 'ON DELETE CASCADE' est configuré pour les tables liées.")
			http.Error(w, "Impossible de supprimer l'utilisateur en raison de données liées. 'ON DELETE CASCADE' est requis.", http.StatusConflict)
			return
		}
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Erreur admin: Commit transaction pour suppression utilisateur %d: %v", userID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Utilisateur %d supprimé de la BDD. Lancement du nettoyage S3...", userID)

	go func() {
		awsService, err := services.NewAWSService()
		if err != nil {
			log.Printf("CRITIQUE: Impossible d'initier le service AWS pour suppression images utilisateur %d: %v", userID, err)
			return
		}

		if avatarURL.Valid && avatarURL.String != "" {
			if err := awsService.DeleteImages([]string{avatarURL.String}); err != nil {
				log.Printf("ERREUR S3: Echec suppression avatar %s pour utilisateur %d: %v", avatarURL.String, userID, err)
			} else {
				log.Printf("Avatar S3 pour utilisateur %d supprimé.", userID)
			}
		}

		if len(allAdImages) > 0 {
			if err := awsService.DeleteImages(allAdImages); err != nil {
				log.Printf("ERREUR S3: Echec suppression %d images d'annonces pour utilisateur %d: %v", len(allAdImages), userID, err)
			} else {
				log.Printf("%d images d'annonces S3 pour utilisateur %d supprimées.", len(allAdImages), userID)
			}
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}
