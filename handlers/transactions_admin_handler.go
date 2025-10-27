package handlers

import (
	"database/sql"
	"encoding/json"
	"kivendi-backend/config"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// --- Structures ---

// KKiaPayTransactionResponse représente une transaction KKiaPay
type KKiaPayTransactionResponse struct {
	ID            int             `json:"id"`
	TransactionID string          `json:"transaction_id"`
	BoostID       *int            `json:"boost_id"`
	AdID          *int            `json:"ad_id"`
	UserID        *int            `json:"user_id"`
	Amount        float64         `json:"amount"`
	Status        string          `json:"status"`
	State         *string         `json:"state"`
	RawResponse   json.RawMessage `json:"raw_response,omitempty"`
	VerifiedAt    *time.Time      `json:"verified_at"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	// Champs enrichis depuis d'autres tables
	UserEmail  *string `json:"user_email,omitempty"`
	UserName   *string `json:"user_name,omitempty"`
	AdTitle    *string `json:"ad_title,omitempty"`
	BoostOffer *string `json:"boost_offer,omitempty"`
}

// TransactionStatsResponse représente les statistiques des transactions
type TransactionStatsResponse struct {
	TotalTransactions      int     `json:"total_transactions"`
	SuccessfulTransactions int     `json:"successful_transactions"`
	PendingTransactions    int     `json:"pending_transactions"`
	FailedTransactions     int     `json:"failed_transactions"`
	TotalAmount            float64 `json:"total_amount"`
	SuccessfulAmount       float64 `json:"successful_amount"`
	AverageAmount          float64 `json:"average_amount"`
}

// --- Handlers ---

// GetAllTransactionsHandler récupère toutes les transactions KKiaPay (admin uniquement)
func GetAllTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Vérifier que l'utilisateur est bien un admin (pas modérateur)
	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins ont accès
	if requestingAdminRole != "admin" {
		httpError(w, "Accès réservé aux administrateurs uniquement", http.StatusForbidden, nil)
		return
	}

	// Récupérer les paramètres de pagination et filtrage
	page := 1
	limit := 20
	status := r.URL.Query().Get("status") // "completed", "pending", "failed", etc.

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	// Construire la requête SQL avec jointures
	query := `
		SELECT 
			kt.id, kt.transaction_id, kt.boost_id, kt.ad_id, kt.user_id, 
			kt.amount, kt.status, kt.state, kt.verified_at, 
			kt.created_at, kt.updated_at,
			u.email as user_email,
			CONCAT(u.first_name, ' ', u.last_name) as user_name,
			a.title as ad_title,
			bo.name as boost_offer
		FROM kkiapay_transactions kt
		LEFT JOIN users u ON kt.user_id = u.id
		LEFT JOIN ads a ON kt.ad_id = a.id
		LEFT JOIN ad_boosts ab ON kt.boost_id = ab.id
		LEFT JOIN boost_offers bo ON ab.boost_offer_id = bo.id
	`

	countQuery := "SELECT COUNT(*) FROM kkiapay_transactions"

	// Appliquer le filtre de statut si fourni
	var args []interface{}
	argIndex := 1

	if status != "" {
		query += " WHERE kt.status = $" + strconv.Itoa(argIndex)
		countQuery += " WHERE status = $1"
		args = append(args, status)
		argIndex++
	}

	// Compter le total
	var totalCount int
	err = config.DB.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		httpError(w, "Erreur lors du comptage des transactions", http.StatusInternalServerError, err)
		return
	}

	// Ajouter l'ordre et la pagination
	query += " ORDER BY kt.created_at DESC LIMIT $" + strconv.Itoa(argIndex) + " OFFSET $" + strconv.Itoa(argIndex+1)
	args = append(args, limit, offset)

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		httpError(w, "Erreur lors de la récupération des transactions", http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var transactions []KKiaPayTransactionResponse
	for rows.Next() {
		var t KKiaPayTransactionResponse
		var boostID, adID, userID sql.NullInt64
		var state, userEmail, userName, adTitle, boostOffer sql.NullString
		var verifiedAt sql.NullTime

		err := rows.Scan(
			&t.ID, &t.TransactionID, &boostID, &adID, &userID,
			&t.Amount, &t.Status, &state, &verifiedAt,
			&t.CreatedAt, &t.UpdatedAt,
			&userEmail, &userName, &adTitle, &boostOffer,
		)
		if err != nil {
			httpError(w, "Erreur lors de la lecture des transactions", http.StatusInternalServerError, err)
			return
		}

		// Convertir les NullXXX en pointeurs
		if boostID.Valid {
			val := int(boostID.Int64)
			t.BoostID = &val
		}
		if adID.Valid {
			val := int(adID.Int64)
			t.AdID = &val
		}
		if userID.Valid {
			val := int(userID.Int64)
			t.UserID = &val
		}
		if state.Valid {
			t.State = &state.String
		}
		if verifiedAt.Valid {
			t.VerifiedAt = &verifiedAt.Time
		}
		if userEmail.Valid {
			t.UserEmail = &userEmail.String
		}
		if userName.Valid {
			t.UserName = &userName.String
		}
		if adTitle.Valid {
			t.AdTitle = &adTitle.String
		}
		if boostOffer.Valid {
			t.BoostOffer = &boostOffer.String
		}

		transactions = append(transactions, t)
	}

	if transactions == nil {
		transactions = []KKiaPayTransactionResponse{}
	}

	// Calculer la pagination
	totalPages := (totalCount + limit - 1) / limit

	log.Printf("Admin %d a consulté les transactions (page %d, total: %d)", requestingAdminID, page, totalCount)

	// Réponse
	response := map[string]interface{}{
		"transactions": transactions,
		"pagination": map[string]interface{}{
			"currentPage":  page,
			"totalItems":   totalCount,
			"itemsPerPage": limit,
			"totalPages":   totalPages,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// GetTransactionByIDHandler récupère une transaction spécifique (admin uniquement)
func GetTransactionByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	if requestingAdminRole != "admin" {
		httpError(w, "Accès réservé aux administrateurs uniquement", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	transactionID, err := strconv.Atoi(vars["transactionID"])
	if err != nil {
		httpError(w, "ID de transaction invalide", http.StatusBadRequest, err)
		return
	}

	var t KKiaPayTransactionResponse
	var boostID, adID, userID sql.NullInt64
	var state, userEmail, userName, adTitle, boostOffer sql.NullString
	var verifiedAt sql.NullTime
	var rawResponse sql.NullString

	query := `
		SELECT 
			kt.id, kt.transaction_id, kt.boost_id, kt.ad_id, kt.user_id, 
			kt.amount, kt.status, kt.state, kt.raw_response, kt.verified_at, 
			kt.created_at, kt.updated_at,
			u.email as user_email,
			CONCAT(u.first_name, ' ', u.last_name) as user_name,
			a.title as ad_title,
			bo.name as boost_offer
		FROM kkiapay_transactions kt
		LEFT JOIN users u ON kt.user_id = u.id
		LEFT JOIN ads a ON kt.ad_id = a.id
		LEFT JOIN ad_boosts ab ON kt.boost_id = ab.id
		LEFT JOIN boost_offers bo ON ab.boost_offer_id = bo.id
		WHERE kt.id = $1
	`

	err = config.DB.QueryRow(query, transactionID).Scan(
		&t.ID, &t.TransactionID, &boostID, &adID, &userID,
		&t.Amount, &t.Status, &state, &rawResponse, &verifiedAt,
		&t.CreatedAt, &t.UpdatedAt,
		&userEmail, &userName, &adTitle, &boostOffer,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Transaction non trouvée", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la récupération de la transaction", http.StatusInternalServerError, err)
		return
	}

	// Convertir les NullXXX
	if boostID.Valid {
		val := int(boostID.Int64)
		t.BoostID = &val
	}
	if adID.Valid {
		val := int(adID.Int64)
		t.AdID = &val
	}
	if userID.Valid {
		val := int(userID.Int64)
		t.UserID = &val
	}
	if state.Valid {
		t.State = &state.String
	}
	if verifiedAt.Valid {
		t.VerifiedAt = &verifiedAt.Time
	}
	if userEmail.Valid {
		t.UserEmail = &userEmail.String
	}
	if userName.Valid {
		t.UserName = &userName.String
	}
	if adTitle.Valid {
		t.AdTitle = &adTitle.String
	}
	if boostOffer.Valid {
		t.BoostOffer = &boostOffer.String
	}
	if rawResponse.Valid {
		t.RawResponse = json.RawMessage(rawResponse.String)
	}

	json.NewEncoder(w).Encode(t)
}

// GetTransactionStatsHandler récupère les statistiques des transactions (admin uniquement)
func GetTransactionStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	if requestingAdminRole != "admin" {
		httpError(w, "Accès réservé aux administrateurs uniquement", http.StatusForbidden, nil)
		return
	}

	var stats TransactionStatsResponse

	query := `
		SELECT 
			COUNT(*) as total_transactions,
			COUNT(CASE WHEN status = 'completed' OR status = 'SUCCESS' THEN 1 END) as successful_transactions,
			COUNT(CASE WHEN status = 'pending' OR status = 'PENDING' THEN 1 END) as pending_transactions,
			COUNT(CASE WHEN status = 'failed' OR status = 'FAILED' THEN 1 END) as failed_transactions,
			COALESCE(SUM(amount), 0) as total_amount,
			COALESCE(SUM(CASE WHEN status = 'completed' OR status = 'SUCCESS' THEN amount ELSE 0 END), 0) as successful_amount,
			COALESCE(AVG(amount), 0) as average_amount
		FROM kkiapay_transactions
	`

	err = config.DB.QueryRow(query).Scan(
		&stats.TotalTransactions,
		&stats.SuccessfulTransactions,
		&stats.PendingTransactions,
		&stats.FailedTransactions,
		&stats.TotalAmount,
		&stats.SuccessfulAmount,
		&stats.AverageAmount,
	)

	if err != nil {
		httpError(w, "Erreur lors de la récupération des statistiques", http.StatusInternalServerError, err)
		return
	}

	json.NewEncoder(w).Encode(stats)
}
