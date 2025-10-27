package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"kivendi-backend/config"

	"github.com/gorilla/mux"
)

// TermsAndConditions représente une version des termes et conditions
type TermsAndConditions struct {
	ID            int       `json:"id"`
	Version       string    `json:"version"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	IsActive      bool      `json:"is_active"`
	EffectiveDate time.Time `json:"effective_date"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GetActiveTermsHandler récupère les termes et conditions actuellement actifs
func GetActiveTermsHandler(w http.ResponseWriter, r *http.Request) {
	var terms TermsAndConditions

	err := config.DB.QueryRow(`
		SELECT id, version, title, content, is_active, effective_date, created_at, updated_at
		FROM terms_and_conditions
		WHERE is_active = true
		LIMIT 1
	`).Scan(
		&terms.ID,
		&terms.Version,
		&terms.Title,
		&terms.Content,
		&terms.IsActive,
		&terms.EffectiveDate,
		&terms.CreatedAt,
		&terms.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Aucune version active des termes et conditions", http.StatusNotFound)
			return
		}
		log.Printf("Erreur lors de la récupération des termes actifs : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(terms); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
}

// GetAllTermsVersionsHandler récupère toutes les versions des termes et conditions
// (Pour administration ou historique)
func GetAllTermsVersionsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := config.DB.Query(`
		SELECT id, version, title, content, is_active, effective_date, created_at, updated_at
		FROM terms_and_conditions
		ORDER BY effective_date DESC
	`)
	if err != nil {
		log.Printf("Erreur lors de la récupération de toutes les versions : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var termsVersions []TermsAndConditions

	for rows.Next() {
		var terms TermsAndConditions
		err := rows.Scan(
			&terms.ID,
			&terms.Version,
			&terms.Title,
			&terms.Content,
			&terms.IsActive,
			&terms.EffectiveDate,
			&terms.CreatedAt,
			&terms.UpdatedAt,
		)
		if err != nil {
			log.Printf("Erreur lors de la lecture des lignes : %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}
		termsVersions = append(termsVersions, terms)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur lors de l'itération des lignes : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Si aucune version n'existe, retourner un tableau vide
	if len(termsVersions) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(termsVersions); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
}

// GetTermsByVersionHandler récupère une version spécifique des termes et conditions
func GetTermsByVersionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	version := vars["version"]

	if version == "" {
		http.Error(w, "Version manquante", http.StatusBadRequest)
		return
	}

	var terms TermsAndConditions

	err := config.DB.QueryRow(`
		SELECT id, version, title, content, is_active, effective_date, created_at, updated_at
		FROM terms_and_conditions
		WHERE version = $1
	`, version).Scan(
		&terms.ID,
		&terms.Version,
		&terms.Title,
		&terms.Content,
		&terms.IsActive,
		&terms.EffectiveDate,
		&terms.CreatedAt,
		&terms.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Version des termes non trouvée", http.StatusNotFound)
			return
		}
		log.Printf("Erreur lors de la récupération de la version %s : %v", version, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(terms); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
}

// GetTermsByIDHandler récupère une version spécifique des termes et conditions par ID
func GetTermsByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	termsIDStr := vars["termsID"]

	termsID, err := strconv.Atoi(termsIDStr)
	if err != nil {
		http.Error(w, "ID des termes invalide", http.StatusBadRequest)
		return
	}

	var terms TermsAndConditions

	err = config.DB.QueryRow(`
		SELECT id, version, title, content, is_active, effective_date, created_at, updated_at
		FROM terms_and_conditions
		WHERE id = $1
	`, termsID).Scan(
		&terms.ID,
		&terms.Version,
		&terms.Title,
		&terms.Content,
		&terms.IsActive,
		&terms.EffectiveDate,
		&terms.CreatedAt,
		&terms.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Termes non trouvés", http.StatusNotFound)
			return
		}
		log.Printf("Erreur lors de la récupération des termes ID %d : %v", termsID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(terms); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
}

// CreateTermsHandler crée une nouvelle version des termes et conditions
// (Réservé aux administrateurs - vous pouvez ajouter un middleware admin)
/*func CreateTermsHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Version       string    `json:"version"`
		Title         string    `json:"title"`
		Content       string    `json:"content"`
		IsActive      bool      `json:"is_active"`
		EffectiveDate time.Time `json:"effective_date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// Validation
	if payload.Version == "" || payload.Title == "" || payload.Content == "" {
		http.Error(w, "Les champs version, title et content sont obligatoires", http.StatusBadRequest)
		return
	}

	var termsID int
	err := config.DB.QueryRow(`
		INSERT INTO terms_and_conditions (version, title, content, is_active, effective_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, payload.Version, payload.Title, payload.Content, payload.IsActive, payload.EffectiveDate).Scan(&termsID)

	if err != nil {
		log.Printf("Erreur lors de la création des termes : %v", err)
		http.Error(w, "Erreur lors de la création des termes. La version existe peut-être déjà.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Termes et conditions créés avec succès",
		"id":      termsID,
	})
}

// UpdateTermsHandler met à jour une version existante des termes et conditions
// (Réservé aux administrateurs)
func UpdateTermsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	termsIDStr := vars["termsID"]

	termsID, err := strconv.Atoi(termsIDStr)
	if err != nil {
		http.Error(w, "ID des termes invalide", http.StatusBadRequest)
		return
	}

	var payload struct {
		Title         string    `json:"title"`
		Content       string    `json:"content"`
		IsActive      bool      `json:"is_active"`
		EffectiveDate time.Time `json:"effective_date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	result, err := config.DB.Exec(`
		UPDATE terms_and_conditions
		SET title = $1, content = $2, is_active = $3, effective_date = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5
	`, payload.Title, payload.Content, payload.IsActive, payload.EffectiveDate, termsID)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour des termes : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Termes non trouvés", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Termes et conditions mis à jour avec succès",
	})
}

// DeleteTermsHandler supprime une version des termes et conditions
// (Réservé aux administrateurs - attention à ne pas supprimer la version active)
func DeleteTermsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	termsIDStr := vars["termsID"]

	termsID, err := strconv.Atoi(termsIDStr)
	if err != nil {
		http.Error(w, "ID des termes invalide", http.StatusBadRequest)
		return
	}

	// Vérifier si c'est la version active
	var isActive bool
	err = config.DB.QueryRow("SELECT is_active FROM terms_and_conditions WHERE id = $1", termsID).Scan(&isActive)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Termes non trouvés", http.StatusNotFound)
			return
		}
		log.Printf("Erreur lors de la vérification des termes : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	if isActive {
		http.Error(w, "Impossible de supprimer la version active des termes et conditions", http.StatusBadRequest)
		return
	}

	result, err := config.DB.Exec("DELETE FROM terms_and_conditions WHERE id = $1", termsID)
	if err != nil {
		log.Printf("Erreur lors de la suppression des termes : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Termes non trouvés", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Termes et conditions supprimés avec succès"))
}*/
