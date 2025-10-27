package handlers

import (
	"database/sql"
	"encoding/json"
	"kivendi-backend/config"
	"log"
	"net/http"
	"strconv"
	"strings"

	// Requis pour la structure NullableTime (utilisée par ReportResponse)
	"github.com/gorilla/mux"
)

// NOTE : Ce fichier suppose que les éléments suivants sont définis dans
// d'autres fichiers du même package 'handlers' (ex: admin_management_handler.go):
// 1. func httpError(w http.ResponseWriter, message string, code int, err error)
// 2. func getRequestingAdmin(r *http.Request) (int, string, error)
// 3. type NullableTime struct { ... } et sa méthode MarshalJSON()
// 4. les constantes de contexte (adminIDContextKey, adminRoleContextKey)

// --- Structures (Signalements) ---

// ReportResponse est la structure JSON pour un signalement, incluant les infos de l'utilisateur
type ReportResponse struct {
	ID             int          `json:"id"`
	ReporterID     int          `json:"reporter_id"`
	ReportedID     int          `json:"reported_id"`
	ConversationID int          `json:"conversation_id"`
	Reason         string       `json:"reason"`
	Status         string       `json:"status"`
	AdminNotes     *string      `json:"admin_notes"` // <--- MODIFIÉ ICI
	CreatedAt      NullableTime `json:"created_at"`
	UpdatedAt      NullableTime `json:"updated_at"`

	// Infos jointes
	ReporterEmail string `json:"reporter_email,omitempty"`
	ReportedEmail string `json:"reported_email,omitempty"`
	ReporterName  string `json:"reporter_name,omitempty"`
	ReportedName  string `json:"reported_name,omitempty"`
}

// UpdateReportRequest est la structure pour la mise à jour d'un signalement
type UpdateReportRequest struct {
	Status     *string `json:"status"`
	AdminNotes *string `json:"admin_notes"`
}

// --- Handlers (Signalements) ---

// GetReportsHandler récupère tous les signalements, filtrables par statut
func GetReportsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// L'authentification est déjà gérée par le middleware adminRoutes

	statusFilter := r.URL.Query().Get("status")

	query := `
		SELECT 
			r.id, r.reporter_id, r.reported_id, r.conversation_id, 
			r.reason, r.status, r.admin_notes, r.created_at, r.updated_at,
			u_reporter.email AS reporter_email,
			(u_reporter.first_name || ' ' || u_reporter.last_name) AS reporter_name,
			u_reported.email AS reported_email,
			(u_reported.first_name || ' ' || u_reported.last_name) AS reported_name
		FROM user_reports r
		LEFT JOIN users u_reporter ON r.reporter_id = u_reporter.id
		LEFT JOIN users u_reported ON r.reported_id = u_reported.id
	`
	var args []interface{}

	if statusFilter != "" {
		query += " WHERE r.status = $1"
		args = append(args, statusFilter)
	}

	// Trie pour voir les "pending" en premier, puis les plus récents
	query += " ORDER BY CASE r.status WHEN 'pending' THEN 1 ELSE 2 END, r.created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		httpError(w, "Erreur lors de la récupération des signalements", http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var reports []ReportResponse
	for rows.Next() {
		var report ReportResponse
		var adminNotes sql.NullString // Variable temporaire pour le scan

		err := rows.Scan(
			&report.ID, &report.ReporterID, &report.ReportedID, &report.ConversationID,
			&report.Reason, &report.Status,
			&adminNotes, // Scan dans la variable temporaire
			&report.CreatedAt, &report.UpdatedAt,
			&report.ReporterEmail, &report.ReporterName,
			&report.ReportedEmail, &report.ReportedName,
		)
		if err != nil {
			httpError(w, "Erreur lors de la lecture des signalements", http.StatusInternalServerError, err)
			return
		}

		// Conversion de sql.NullString vers *string
		if adminNotes.Valid {
			report.AdminNotes = &adminNotes.String
		}
		// Si adminNotes.Valid est false, report.AdminNotes reste nil (JSON null)

		reports = append(reports, report)
	}

	if err = rows.Err(); err != nil {
		httpError(w, "Erreur lors de l'itération sur les signalements", http.StatusInternalServerError, err)
		return
	}

	json.NewEncoder(w).Encode(reports)
}

// UpdateReportHandler met à jour le statut et les notes d'un signalement (PATCH)
func UpdateReportHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// getRequestingAdmin est défini dans admin_management_handler.go
	requestingAdminID, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	reportID, err := strconv.Atoi(vars["reportID"])
	if err != nil {
		httpError(w, "ID de signalement invalide", http.StatusBadRequest, err)
		return
	}

	// Utilise UpdateReportRequest avec des pointeurs pour gérer les mises à jour partielles (PATCH)
	var req UpdateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// 1. Récupérer le signalement actuel
	var currentStatus string
	var currentNotes sql.NullString
	err = config.DB.QueryRow("SELECT status, admin_notes FROM user_reports WHERE id = $1", reportID).Scan(&currentStatus, &currentNotes)
	if err != nil {
		if err == sql.ErrNoRows {
			httpError(w, "Signalement non trouvé", http.StatusNotFound, err)
		} else {
			httpError(w, "Erreur lors de la récupération du signalement", http.StatusInternalServerError, err)
		}
		return
	}

	// 2. Appliquer les modifications si elles sont fournies dans le JSON
	if req.Status != nil {
		status := strings.ToLower(*req.Status)
		if status != "resolved" && status != "dismissed" && status != "pending" {
			httpError(w, "Statut invalide. Doit être 'pending', 'resolved' ou 'dismissed'.", http.StatusBadRequest, nil)
			return
		}
		currentStatus = status
	}

	if req.AdminNotes != nil {
		if *req.AdminNotes == "" {
			currentNotes = sql.NullString{Valid: false} // Mettre à NULL
		} else {
			currentNotes = sql.NullString{String: *req.AdminNotes, Valid: true} // Mettre à jour la note
		}
	}

	// 3. Exécuter la mise à jour
	query := `
		UPDATE user_reports
		SET status = $1, admin_notes = $2, updated_at = NOW()
		WHERE id = $3
	`

	result, err := config.DB.Exec(query, currentStatus, currentNotes, reportID)
	if err != nil {
		httpError(w, "Erreur lors de la mise à jour du signalement", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Signalement non trouvé (ou aucune modification)", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a mis à jour le signalement %d (Statut: %s)", requestingAdminID, reportID, currentStatus)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Signalement mis à jour avec succès"})
}
