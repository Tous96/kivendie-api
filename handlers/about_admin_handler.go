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

// AboutPageResponse représente une page "À propos"
type AboutPageResponse struct {
	ID              int       `json:"id"`
	Title           string    `json:"title"`
	Subtitle        *string   `json:"subtitle"`
	Content         string    `json:"content"`
	MissionTitle    *string   `json:"mission_title"`
	MissionContent  *string   `json:"mission_content"`
	VisionTitle     *string   `json:"vision_title"`
	VisionContent   *string   `json:"vision_content"`
	ValuesTitle     *string   `json:"values_title"`
	ValuesContent   *string   `json:"values_content"`
	TeamTitle       *string   `json:"team_title"`
	TeamContent     *string   `json:"team_content"`
	HeroImageURL    *string   `json:"hero_image_url"`
	MissionImageURL *string   `json:"mission_image_url"`
	VisionImageURL  *string   `json:"vision_image_url"`
	IsActive        bool      `json:"is_active"`
	CreatedBy       *int      `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateAboutPageRequest représente la requête de création
type CreateAboutPageRequest struct {
	Title           string  `json:"title"`
	Subtitle        *string `json:"subtitle"`
	Content         string  `json:"content"`
	MissionTitle    *string `json:"mission_title"`
	MissionContent  *string `json:"mission_content"`
	VisionTitle     *string `json:"vision_title"`
	VisionContent   *string `json:"vision_content"`
	ValuesTitle     *string `json:"values_title"`
	ValuesContent   *string `json:"values_content"`
	TeamTitle       *string `json:"team_title"`
	TeamContent     *string `json:"team_content"`
	HeroImageURL    *string `json:"hero_image_url"`
	MissionImageURL *string `json:"mission_image_url"`
	VisionImageURL  *string `json:"vision_image_url"`
	IsActive        bool    `json:"is_active"`
}

// UpdateAboutPageRequest représente la requête de mise à jour
type UpdateAboutPageRequest struct {
	Title           string  `json:"title"`
	Subtitle        *string `json:"subtitle"`
	Content         string  `json:"content"`
	MissionTitle    *string `json:"mission_title"`
	MissionContent  *string `json:"mission_content"`
	VisionTitle     *string `json:"vision_title"`
	VisionContent   *string `json:"vision_content"`
	ValuesTitle     *string `json:"values_title"`
	ValuesContent   *string `json:"values_content"`
	TeamTitle       *string `json:"team_title"`
	TeamContent     *string `json:"team_content"`
	HeroImageURL    *string `json:"hero_image_url"`
	MissionImageURL *string `json:"mission_image_url"`
	VisionImageURL  *string `json:"vision_image_url"`
}

// --- Handlers ---

// GetAllAboutPagesForAdminHandler récupère toutes les pages "À propos" (admin uniquement)
func GetAllAboutPagesForAdminHandler(w http.ResponseWriter, r *http.Request) {
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
		SELECT id, title, subtitle, content, mission_title, mission_content, 
		       vision_title, vision_content, values_title, values_content,
		       team_title, team_content, hero_image_url, mission_image_url, 
		       vision_image_url, is_active, created_by, created_at, updated_at
		FROM about_pages
	`

	switch status {
	case "active":
		query += " WHERE is_active = true"
	case "inactive":
		query += " WHERE is_active = false"
	}

	query += " ORDER BY created_at DESC"

	rows, err := config.DB.Query(query)
	if err != nil {
		httpError(w, "Erreur lors de la récupération des pages", http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var pages []AboutPageResponse
	for rows.Next() {
		var page AboutPageResponse
		var createdBy sql.NullInt64

		err := rows.Scan(
			&page.ID, &page.Title, &page.Subtitle, &page.Content,
			&page.MissionTitle, &page.MissionContent, &page.VisionTitle, &page.VisionContent,
			&page.ValuesTitle, &page.ValuesContent, &page.TeamTitle, &page.TeamContent,
			&page.HeroImageURL, &page.MissionImageURL, &page.VisionImageURL,
			&page.IsActive, &createdBy, &page.CreatedAt, &page.UpdatedAt,
		)
		if err != nil {
			httpError(w, "Erreur lors de la lecture des pages", http.StatusInternalServerError, err)
			return
		}

		if createdBy.Valid {
			val := int(createdBy.Int64)
			page.CreatedBy = &val
		}

		pages = append(pages, page)
	}

	if pages == nil {
		pages = []AboutPageResponse{}
	}

	json.NewEncoder(w).Encode(pages)
}

// GetAboutPageByIDForAdminHandler récupère une page "À propos" spécifique
func GetAboutPageByIDForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	pageID, err := strconv.Atoi(vars["pageID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	var page AboutPageResponse
	var createdBy sql.NullInt64

	query := `
		SELECT id, title, subtitle, content, mission_title, mission_content, 
		       vision_title, vision_content, values_title, values_content,
		       team_title, team_content, hero_image_url, mission_image_url, 
		       vision_image_url, is_active, created_by, created_at, updated_at
		FROM about_pages
		WHERE id = $1
	`

	err = config.DB.QueryRow(query, pageID).Scan(
		&page.ID, &page.Title, &page.Subtitle, &page.Content,
		&page.MissionTitle, &page.MissionContent, &page.VisionTitle, &page.VisionContent,
		&page.ValuesTitle, &page.ValuesContent, &page.TeamTitle, &page.TeamContent,
		&page.HeroImageURL, &page.MissionImageURL, &page.VisionImageURL,
		&page.IsActive, &createdBy, &page.CreatedAt, &page.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Page non trouvée", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la récupération de la page", http.StatusInternalServerError, err)
		return
	}

	if createdBy.Valid {
		val := int(createdBy.Int64)
		page.CreatedBy = &val
	}

	json.NewEncoder(w).Encode(page)
}

// CreateAboutPageHandler crée une nouvelle page "À propos"
func CreateAboutPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins peuvent créer des pages
	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent créer des pages", http.StatusForbidden, nil)
		return
	}

	var req CreateAboutPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// Validation des données
	if req.Title == "" || req.Content == "" {
		httpError(w, "Le titre et le contenu sont requis", http.StatusBadRequest, nil)
		return
	}

	if len(req.Content) < 50 {
		httpError(w, "Le contenu doit contenir au moins 50 caractères", http.StatusBadRequest, nil)
		return
	}

	// Si on active cette page, désactiver toutes les autres
	if req.IsActive {
		_, err = config.DB.Exec("UPDATE about_pages SET is_active = false WHERE is_active = true")
		if err != nil {
			httpError(w, "Erreur lors de la désactivation des anciennes pages", http.StatusInternalServerError, err)
			return
		}
	}

	// Insérer la nouvelle page
	var newPage AboutPageResponse
	var createdBy sql.NullInt64

	query := `
		INSERT INTO about_pages (
			title, subtitle, content, mission_title, mission_content, 
			vision_title, vision_content, values_title, values_content,
			team_title, team_content, hero_image_url, mission_image_url, 
			vision_image_url, is_active, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, title, subtitle, content, mission_title, mission_content, 
		          vision_title, vision_content, values_title, values_content,
		          team_title, team_content, hero_image_url, mission_image_url, 
		          vision_image_url, is_active, created_by, created_at, updated_at
	`

	err = config.DB.QueryRow(
		query,
		req.Title, req.Subtitle, req.Content, req.MissionTitle, req.MissionContent,
		req.VisionTitle, req.VisionContent, req.ValuesTitle, req.ValuesContent,
		req.TeamTitle, req.TeamContent, req.HeroImageURL, req.MissionImageURL,
		req.VisionImageURL, req.IsActive, requestingAdminID,
	).Scan(
		&newPage.ID, &newPage.Title, &newPage.Subtitle, &newPage.Content,
		&newPage.MissionTitle, &newPage.MissionContent, &newPage.VisionTitle, &newPage.VisionContent,
		&newPage.ValuesTitle, &newPage.ValuesContent, &newPage.TeamTitle, &newPage.TeamContent,
		&newPage.HeroImageURL, &newPage.MissionImageURL, &newPage.VisionImageURL,
		&newPage.IsActive, &createdBy, &newPage.CreatedAt, &newPage.UpdatedAt,
	)

	if err != nil {
		httpError(w, "Erreur lors de la création de la page", http.StatusInternalServerError, err)
		return
	}

	if createdBy.Valid {
		val := int(createdBy.Int64)
		newPage.CreatedBy = &val
	}

	log.Printf("Admin %d a créé une nouvelle page 'À propos' (ID: %d)", requestingAdminID, newPage.ID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newPage)
}

// UpdateAboutPageHandler met à jour une page "À propos" existante
func UpdateAboutPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	// Seuls les admins peuvent modifier des pages
	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent modifier des pages", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	pageID, err := strconv.Atoi(vars["pageID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	var req UpdateAboutPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// Validation
	if req.Title == "" || req.Content == "" {
		httpError(w, "Le titre et le contenu sont requis", http.StatusBadRequest, nil)
		return
	}

	if len(req.Content) < 50 {
		httpError(w, "Le contenu doit contenir au moins 50 caractères", http.StatusBadRequest, nil)
		return
	}

	query := `
		UPDATE about_pages
		SET title = $1, subtitle = $2, content = $3, mission_title = $4, mission_content = $5,
		    vision_title = $6, vision_content = $7, values_title = $8, values_content = $9,
		    team_title = $10, team_content = $11, hero_image_url = $12, mission_image_url = $13,
		    vision_image_url = $14, updated_at = CURRENT_TIMESTAMP
		WHERE id = $15
		RETURNING id, title, subtitle, content, mission_title, mission_content, 
		          vision_title, vision_content, values_title, values_content,
		          team_title, team_content, hero_image_url, mission_image_url, 
		          vision_image_url, is_active, created_by, created_at, updated_at
	`

	var updatedPage AboutPageResponse
	var createdBy sql.NullInt64

	err = config.DB.QueryRow(
		query,
		req.Title, req.Subtitle, req.Content, req.MissionTitle, req.MissionContent,
		req.VisionTitle, req.VisionContent, req.ValuesTitle, req.ValuesContent,
		req.TeamTitle, req.TeamContent, req.HeroImageURL, req.MissionImageURL,
		req.VisionImageURL, pageID,
	).Scan(
		&updatedPage.ID, &updatedPage.Title, &updatedPage.Subtitle, &updatedPage.Content,
		&updatedPage.MissionTitle, &updatedPage.MissionContent, &updatedPage.VisionTitle, &updatedPage.VisionContent,
		&updatedPage.ValuesTitle, &updatedPage.ValuesContent, &updatedPage.TeamTitle, &updatedPage.TeamContent,
		&updatedPage.HeroImageURL, &updatedPage.MissionImageURL, &updatedPage.VisionImageURL,
		&updatedPage.IsActive, &createdBy, &updatedPage.CreatedAt, &updatedPage.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Page non trouvée", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la mise à jour de la page", http.StatusInternalServerError, err)
		return
	}

	if createdBy.Valid {
		val := int(createdBy.Int64)
		updatedPage.CreatedBy = &val
	}

	log.Printf("Admin %d a mis à jour la page 'À propos' ID %d", requestingAdminID, pageID)
	json.NewEncoder(w).Encode(updatedPage)
}

// ToggleAboutPageStatusHandler active ou désactive une page "À propos"
func ToggleAboutPageStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent modifier le statut des pages", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	pageID, err := strconv.Atoi(vars["pageID"])
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

	// Si on active cette page, désactiver toutes les autres
	if req.IsActive {
		_, err = config.DB.Exec("UPDATE about_pages SET is_active = false WHERE is_active = true AND id != $1", pageID)
		if err != nil {
			httpError(w, "Erreur lors de la désactivation des autres pages", http.StatusInternalServerError, err)
			return
		}
	}

	query := `
		UPDATE about_pages
		SET is_active = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		RETURNING id, title, subtitle, content, mission_title, mission_content, 
		          vision_title, vision_content, values_title, values_content,
		          team_title, team_content, hero_image_url, mission_image_url, 
		          vision_image_url, is_active, created_by, created_at, updated_at
	`

	var updatedPage AboutPageResponse
	var createdBy sql.NullInt64

	err = config.DB.QueryRow(query, req.IsActive, pageID).Scan(
		&updatedPage.ID, &updatedPage.Title, &updatedPage.Subtitle, &updatedPage.Content,
		&updatedPage.MissionTitle, &updatedPage.MissionContent, &updatedPage.VisionTitle, &updatedPage.VisionContent,
		&updatedPage.ValuesTitle, &updatedPage.ValuesContent, &updatedPage.TeamTitle, &updatedPage.TeamContent,
		&updatedPage.HeroImageURL, &updatedPage.MissionImageURL, &updatedPage.VisionImageURL,
		&updatedPage.IsActive, &createdBy, &updatedPage.CreatedAt, &updatedPage.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Page non trouvée", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la modification du statut", http.StatusInternalServerError, err)
		return
	}

	if createdBy.Valid {
		val := int(createdBy.Int64)
		updatedPage.CreatedBy = &val
	}

	status := "désactivée"
	if req.IsActive {
		status = "activée"
	}

	log.Printf("Admin %d a %s la page 'À propos' ID %d", requestingAdminID, status, pageID)
	json.NewEncoder(w).Encode(updatedPage)
}

// DeleteAboutPageHandler supprime une page "À propos"
func DeleteAboutPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	if requestingAdminRole != "admin" {
		httpError(w, "Seuls les administrateurs peuvent supprimer des pages", http.StatusForbidden, nil)
		return
	}

	vars := mux.Vars(r)
	pageID, err := strconv.Atoi(vars["pageID"])
	if err != nil {
		httpError(w, "ID invalide", http.StatusBadRequest, err)
		return
	}

	// Vérifier si c'est la page active
	var isActive bool
	err = config.DB.QueryRow("SELECT is_active FROM about_pages WHERE id = $1", pageID).Scan(&isActive)
	if err == sql.ErrNoRows {
		httpError(w, "Page non trouvée", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la vérification de la page", http.StatusInternalServerError, err)
		return
	}

	// Interdire la suppression de la page active
	if isActive {
		httpError(w, "Impossible de supprimer la page active. Veuillez d'abord activer une autre page", http.StatusForbidden, nil)
		return
	}

	query := "DELETE FROM about_pages WHERE id = $1"
	result, err := config.DB.Exec(query, pageID)
	if err != nil {
		httpError(w, "Erreur lors de la suppression de la page", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Page non trouvée", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a supprimé la page 'À propos' ID %d", requestingAdminID, pageID)
	w.WriteHeader(http.StatusNoContent)
}

// GetActiveAboutPageHandler récupère la page "À propos" active (PUBLIC - pas d'auth requise)
func GetActiveAboutPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var page AboutPageResponse
	var createdBy sql.NullInt64

	query := `
		SELECT id, title, subtitle, content, mission_title, mission_content, 
		       vision_title, vision_content, values_title, values_content,
		       team_title, team_content, hero_image_url, mission_image_url, 
		       vision_image_url, is_active, created_by, created_at, updated_at
		FROM about_pages
		WHERE is_active = true
		LIMIT 1
	`

	err := config.DB.QueryRow(query).Scan(
		&page.ID, &page.Title, &page.Subtitle, &page.Content,
		&page.MissionTitle, &page.MissionContent, &page.VisionTitle, &page.VisionContent,
		&page.ValuesTitle, &page.ValuesContent, &page.TeamTitle, &page.TeamContent,
		&page.HeroImageURL, &page.MissionImageURL, &page.VisionImageURL,
		&page.IsActive, &createdBy, &page.CreatedAt, &page.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		httpError(w, "Aucune page 'À propos' active n'a été trouvée", http.StatusNotFound, err)
		return
	} else if err != nil {
		httpError(w, "Erreur lors de la récupération de la page", http.StatusInternalServerError, err)
		return
	}

	if createdBy.Valid {
		val := int(createdBy.Int64)
		page.CreatedBy = &val
	}

	json.NewEncoder(w).Encode(page)
}
