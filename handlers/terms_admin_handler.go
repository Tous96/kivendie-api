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

// TermsAndConditionsResponse représente un terme et condition
type TermsAndConditionsResponse struct {
	ID            int       `json:"id"`
	Version       string    `json:"version"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	IsActive      bool      `json:"is_active"`
	EffectiveDate time.Time `json:"effective_date"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateTermsRequest représente la requête de création
type CreateTermsRequest struct {
	Version       string `json:"version"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	IsActive      bool   `json:"is_active"`
	EffectiveDate string `json:"effective_date"` // Format ISO 8601
}

// UpdateTermsRequest représente la requête de mise à jour
type UpdateTermsRequest struct {
	Version       string `json:"version"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	EffectiveDate string `json:"effective_date"`
}

// --- Handlers ---

// GetAllTermsForAdminHandler récupère tous les termes et conditions (admin uniquement)
func GetAllTermsForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Vérifier que l'utilisateur est bien un admin ou modérateur
	_, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Récupérer les paramètres de filtrage optionnels
	status := r.URL.Query().Get("status") // "active", "inactive", ou vide pour tous

	query := `
		SELECT id, version, title, content, is_active, effective_date, created_at, updated_at
		FROM terms_and_conditions
	`

	var args []interface{}
	switch status {
	case "active":
		query += " WHERE is_active = true"
	case "inactive":
		query += " WHERE is_active = false"
	}

	query += " ORDER BY created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		httpError(w, "Erreur lors de la récupération des termes", http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var termsList []TermsAndConditionsResponse
	for rows.Next() {
		var terms TermsAndConditionsResponse
		err := rows.Scan(
			&terms.ID, &terms.Version, &terms.Title, &terms.Content,
			&terms.IsActive, &terms.EffectiveDate, &terms.CreatedAt, &terms.UpdatedAt,
		)
		if err != nil {
			httpError(w, "Erreur lors de la lecture des termes", http.StatusInternalServerError, err)
			return
		}
		termsList = append(termsList, terms)
	}

	if termsList == nil {
		termsList = []TermsAndConditionsResponse{}
	}

	json.NewEncoder(w).Encode(termsList)
}

// GetTermsByIDForAdminHandler récupère un terme et condition spécifique
func GetTermsByIDForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	termsID, err := strconv.Atoi(vars["termsID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	var terms TermsAndConditionsResponse
	query := `
		SELECT id, version, title, content, is_active, effective_date, created_at, updated_at
		FROM terms_and_conditions
		WHERE id = $1
	`

	err = config.DB.QueryRow(query, termsID).Scan(
		&terms.ID, &terms.Version, &terms.Title, &terms.Content,
		&terms.IsActive, &terms.EffectiveDate, &terms.CreatedAt, &terms.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Termes et conditions non trouvés", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la récupération des termes", http.StatusInternalServerError, err)
		return
	}

	json.NewEncoder(w).Encode(terms)
}

// CreateTermsHandler crée un nouveau terme et condition
func CreateTermsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins peuvent créer des termes et conditions
	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent créer des termes et conditions", http.StatusForbidden, nil)
		return
	}

	var req CreateTermsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// Validation des données
	if req.Version == "" || req.Title == "" || req.Content == "" || req.EffectiveDate == "" {
		httpError(w, "Tous les champs sont requis", http.StatusBadRequest, nil)
		return
	}

	// Parser la date effective
	effectiveDate, err := time.Parse(time.RFC3339, req.EffectiveDate)
	if err != nil {
		httpError(w, "Format de date invalide. Utilisez le format ISO 8601", http.StatusBadRequest, err)
		return
	}

	// Si on active ce nouveau terme, désactiver tous les autres
	if req.IsActive {
		_, err = config.DB.Exec("UPDATE terms_and_conditions SET is_active = false WHERE is_active = true")
		if err != nil {
			httpError(w, "Erreur lors de la désactivation des anciens termes", http.StatusInternalServerError, err)
			return
		}
	}

	// Insérer le nouveau terme
	var newTerms TermsAndConditionsResponse
	query := `
		INSERT INTO terms_and_conditions (version, title, content, is_active, effective_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, version, title, content, is_active, effective_date, created_at, updated_at
	`

	err = config.DB.QueryRow(
		query,
		req.Version, req.Title, req.Content, req.IsActive, effectiveDate,
	).Scan(
		&newTerms.ID, &newTerms.Version, &newTerms.Title, &newTerms.Content,
		&newTerms.IsActive, &newTerms.EffectiveDate, &newTerms.CreatedAt, &newTerms.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"terms_and_conditions_version_key\"" {
			httpError(w, "Cette version existe déjà", http.StatusConflict, err)
		} else {
			httpError(w, "Erreur lors de la création des termes", http.StatusInternalServerError, err)
		}
		return
	}

	log.Printf("Admin %d a créé un nouveau terme et condition (ID: %d, Version: %s)",
		requestingAdminID, newTerms.ID, newTerms.Version)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newTerms)
}

// UpdateTermsHandler met à jour un terme et condition existant
func UpdateTermsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins peuvent modifier des termes et conditions
	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent modifier des termes et conditions", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	termsID, err := strconv.Atoi(vars["termsID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	var req UpdateTermsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// Validation
	if req.Version == "" || req.Title == "" || req.Content == "" || req.EffectiveDate == "" {
		httpError(w, "Tous les champs sont requis", http.StatusBadRequest, nil)
		return
	}

	effectiveDate, err := time.Parse(time.RFC3339, req.EffectiveDate)
	if err != nil {
		httpError(w, "Format de date invalide", http.StatusBadRequest, err)
		return
	}

	query := `
		UPDATE terms_and_conditions
		SET version = $1, title = $2, content = $3, effective_date = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5
		RETURNING id, version, title, content, is_active, effective_date, created_at, updated_at
	`

	var updatedTerms TermsAndConditionsResponse
	err = config.DB.QueryRow(
		query,
		req.Version, req.Title, req.Content, effectiveDate, termsID,
	).Scan(
		&updatedTerms.ID, &updatedTerms.Version, &updatedTerms.Title, &updatedTerms.Content,
		&updatedTerms.IsActive, &updatedTerms.EffectiveDate, &updatedTerms.CreatedAt, &updatedTerms.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Termes et conditions non trouvés", http.StatusNotFound, err)
		return
	} else if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"terms_and_conditions_version_key\"" {
			httpError(w, "Cette version existe déjà", http.StatusConflict, err)
		} else {
			httpError(w, "Erreur lors de la mise à jour des termes", http.StatusInternalServerError, err)
		}
		return
	}

	log.Printf("Admin %d a mis à jour le terme et condition ID %d", requestingAdminID, termsID)
	json.NewEncoder(w).Encode(updatedTerms)
}

// ToggleTermsStatusHandler active ou désactive un terme et condition
func ToggleTermsStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent modifier le statut des termes", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	termsID, err := strconv.Atoi(vars["termsID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// Si on active ce terme, désactiver tous les autres
	if req.IsActive {
		_, err = config.DB.Exec("UPDATE terms_and_conditions SET is_active = false WHERE is_active = true AND id != $1", termsID)
		if err != nil {
			httpError(w, "Erreur lors de la désactivation des autres termes", http.StatusInternalServerError, err)
			return
		}
	}

	query := `
		UPDATE terms_and_conditions
		SET is_active = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		RETURNING id, version, title, content, is_active, effective_date, created_at, updated_at
	`

	var updatedTerms TermsAndConditionsResponse
	err = config.DB.QueryRow(query, req.IsActive, termsID).Scan(
		&updatedTerms.ID, &updatedTerms.Version, &updatedTerms.Title, &updatedTerms.Content,
		&updatedTerms.IsActive, &updatedTerms.EffectiveDate, &updatedTerms.CreatedAt, &updatedTerms.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Termes et conditions non trouvés", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la modification du statut", http.StatusInternalServerError, err)
		return
	}

	status := "désactivé"
	if req.IsActive {
		status = "activé"
	}

	log.Printf("Admin %d a %s le terme et condition ID %d", requestingAdminID, status, termsID)
	json.NewEncoder(w).Encode(updatedTerms)
}

// DeleteTermsHandler supprime un terme et condition
func DeleteTermsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent supprimer des termes et conditions", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	termsID, err := strconv.Atoi(vars["termsID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	// Vérifier si c'est le terme actif
	var isActive bool
	err = config.DB.QueryRow("SELECT is_active FROM terms_and_conditions WHERE id = $1", termsID).Scan(&isActive)
	if err == sql.ErrNoRows {
		httpError(w, "Termes et conditions non trouvés", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la vérification du terme", http.StatusInternalServerError, err)
		return
	}

	// Interdire la suppression du terme actif
	if isActive {
		httpError(w, "Impossible de supprimer les termes et conditions actifs. Veuillez d'abord activer une autre version", http.StatusForbidden, nil)
		return
	}

	query := "DELETE FROM terms_and_conditions WHERE id = $1"
	result, err := config.DB.Exec(query, termsID)
	if err != nil {
		httpError(w, "Erreur lors de la suppression des termes", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Termes et conditions non trouvés", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a supprimé le terme et condition ID %d", requestingAdminID, termsID)
	w.WriteHeader(http.StatusNoContent)
}
