package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"kivendi-backend/config"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	// Requis pour NullableTime
	"github.com/gorilla/mux"
	"gopkg.in/gomail.v2"
)

// SupportTicket est la structure JSON pour un ticket de support
type SupportTicket struct {
	ID        int          `json:"id"`
	UserName  string       `json:"user_name"`
	UserEmail string       `json:"user_email"`
	Message   string       `json:"message"`
	Status    string       `json:"status"`
	CreatedAt NullableTime `json:"created_at"` // Réutilise NullableTime
}

// UpdateTicketRequest est la structure pour la mise à jour admin
type UpdateTicketRequest struct {
	Status string `json:"status"`
}

type ReplyTicketRequest struct {
	ReplyMessage string `json:"reply_message"`
}

// GetSupportTicketsHandler récupère tous les tickets de support (Admin)
// GET /api/v1/admin/support-tickets
func GetSupportTicketsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// L'authentification est déjà gérée par le middleware adminRoutes

	statusFilter := r.URL.Query().Get("status")

	query := `
		SELECT id, user_name, user_email, message, status, created_at
		FROM support_tickets
	`
	var args []interface{}

	if statusFilter != "" {
		query += " WHERE status = $1"
		args = append(args, statusFilter)
	}

	// Trie pour voir les "Nouveau" en premier, puis les plus récents
	query += " ORDER BY CASE status WHEN 'Nouveau' THEN 1 ELSE 2 END, created_at DESC"

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		httpError(w, "Erreur lors de la récupération des tickets", http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var tickets []SupportTicket
	for rows.Next() {
		var ticket SupportTicket
		err := rows.Scan(
			&ticket.ID, &ticket.UserName, &ticket.UserEmail,
			&ticket.Message, &ticket.Status, &ticket.CreatedAt,
		)
		if err != nil {
			httpError(w, "Erreur lors de la lecture des tickets", http.StatusInternalServerError, err)
			return
		}
		tickets = append(tickets, ticket)
	}

	if err = rows.Err(); err != nil {
		httpError(w, "Erreur lors de l'itération sur les tickets", http.StatusInternalServerError, err)
		return
	}

	// Assurer un tableau vide au lieu de 'null' si aucun ticket n'est trouvé
	if tickets == nil {
		tickets = make([]SupportTicket, 0)
	}

	json.NewEncoder(w).Encode(tickets)
}

// UpdateSupportTicketHandler met à jour le statut d'un ticket (Admin)
// PATCH /api/v1/admin/support-tickets/{ticketID}
func UpdateSupportTicketHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["ticketID"])
	if err != nil {
		httpError(w, "ID de ticket invalide", http.StatusBadRequest, err)
		return
	}

	var req UpdateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// Validation du statut
	status := strings.Title(strings.ToLower(req.Status)) // Met en majuscule "En cours", "Fermé", etc.
	if status == "" {
		httpError(w, "Le statut ne peut pas être vide", http.StatusBadRequest, nil)
		return
	}

	query := "UPDATE support_tickets SET status = $1 WHERE id = $2"
	result, err := config.DB.Exec(query, status, ticketID)
	if err != nil {
		httpError(w, "Erreur lors de la mise à jour du ticket", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Ticket non trouvé", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a mis à jour le statut du ticket %d (Statut: %s)",
		requestingAdminID, ticketID, status)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Statut du ticket mis à jour"})
}

// ReplySupportTicketHandler envoie une réponse par email à l'utilisateur pour un ticket
// POST /api/v1/admin/support-tickets/{ticketID}/reply
func ReplySupportTicketHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["ticketID"])
	if err != nil {
		httpError(w, "ID de ticket invalide", http.StatusBadRequest, err)
		return
	}

	// 1. Parser le message de réponse de l'admin
	var req ReplyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}
	if req.ReplyMessage == "" {
		httpError(w, "Le message de réponse ne peut pas être vide", http.StatusBadRequest, nil)
		return
	}

	// 2. Récupérer les infos du ticket (email, nom, message original)
	var userName, userEmail, originalMessage string
	query := "SELECT user_name, user_email, message FROM support_tickets WHERE id = $1"
	err = config.DB.QueryRow(query, ticketID).Scan(&userName, &userEmail, &originalMessage)
	if err != nil {
		if err == sql.ErrNoRows {
			httpError(w, "Ticket non trouvé", http.StatusNotFound, err)
		} else {
			httpError(w, "Erreur lors de la récupération du ticket", http.StatusInternalServerError, err)
		}
		return
	}

	// 3. --- Logique d'envoi d'email (MODIFIÉE POUR HTML) ---

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := os.Getenv("SMTP_FROM")

	if smtpHost == "" || smtpPortStr == "" || smtpUser == "" || smtpPass == "" {
		log.Printf("ERREUR: Configuration SMTP manquante (HOST, PORT, USER, PASS) pour ticket %d", ticketID)
		httpError(w, "Configuration du service d'email manquante côté serveur", http.StatusInternalServerError, nil)
		return
	}

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		log.Printf("ERREUR: SMTP_PORT invalide: %v", err)
		httpError(w, "Configuration SMTP invalide (Port)", http.StatusInternalServerError, err)
		return
	}

	// Créer le message
	m := gomail.NewMessage()
	m.SetHeader("From", smtpFrom)
	m.SetHeader("To", userEmail)
	m.SetHeader("Subject", "Re: Votre ticket de support #"+strconv.Itoa(ticketID))

	// NOUVEAU: Échapper les données en texte brut pour les insérer dans l'HTML
	// Note: req.ReplyMessage n'est PAS échappé car c'est du HTML
	escapedUserName := html.EscapeString(userName)
	escapedOriginalMessage := html.EscapeString(originalMessage)

	// NOUVEAU: Construire le corps de l'email au format HTML
	// Les styles sont "inline" pour une compatibilité maximale avec les clients email
	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="fr">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
	</head>
	<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0;">
		<div style="width: 90%%; max-width: 600px; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 8px;">
			
			<p style="font-size: 1.1em;">Bonjour %s,</p>
			<p>Merci de nous avoir contactés. Voici la réponse de notre équipe :</p>
			
			<div style="margin-top: 20px; padding: 15px; background-color: #fdfdfd; border-radius: 5px; border: 1px solid #eee;">
				%s
			</div>
			
			<div style="margin: 25px 0; border-top: 1px solid #eee;"></div>
			
			<p>Cordialement,<br>L'équipe Support Kivendie</p>
			
			<div style="background-color: #f9f9f9; border-left: 4px solid #ccc; padding: 10px; margin-top: 25px; font-style: italic; color: #555;">
				<p style="margin: 0 0 10px 0; font-style: normal; font-weight: bold;">Votre message original :</p>
				<p style="margin: 0;">%s</p>
			</div>

		</div>
	</body>
	</html>
	`, escapedUserName, req.ReplyMessage, escapedOriginalMessage)

	// NOUVEAU: Définir le corps de l'email comme "text/html"
	m.SetBody("text/html", htmlBody)

	// (Optionnel mais recommandé) Vous pourriez ajouter une version texte brut
	// en "strippant" le HTML de req.ReplyMessage, mais pour l'instant,
	// le HTML est la version principale.
	// m.AddAlternative("text/plain", "...")

	// Configurer le Dialer SMTP
	d := gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass)

	// Envoyer l'email
	if err := d.DialAndSend(m); err != nil {
		log.Printf("ERREUR: Échec d'envoi d'email pour ticket %d: %v", ticketID, err)
		httpError(w, "Erreur lors de l'envoi de l'email", http.StatusInternalServerError, err)
		return
	}

	// --- Fin de la logique d'envoi ---

	// 4. Mettre à jour le statut du ticket (passer à "En cours")
	updateQuery := "UPDATE support_tickets SET status = $1 WHERE id = $2"
	_, err = config.DB.Exec(updateQuery, "En cours", ticketID)
	if err != nil {
		log.Printf("ERREUR: Échec de la mise à jour du statut pour le ticket %d après réponse: %v", ticketID, err)
	}

	// 5. Envoyer la réponse de succès
	log.Printf("Admin %d a répondu au ticket %d (Email: %s)", requestingAdminID, ticketID, userEmail)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Réponse envoyée avec succès à " + userEmail,
	})
}

// DeleteSupportTicketHandler supprime un ticket de support (Admin)
// DELETE /api/v1/admin/support-tickets/{ticketID}
func DeleteSupportTicketHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	ticketID, err := strconv.Atoi(vars["ticketID"])
	if err != nil {
		httpError(w, "ID de ticket invalide", http.StatusBadRequest, err)
		return
	}

	query := "DELETE FROM support_tickets WHERE id = $1"
	result, err := config.DB.Exec(query, ticketID)
	if err != nil {
		httpError(w, "Erreur lors de la suppression du ticket", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Ticket non trouvé", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a SUPPRIMÉ le ticket %d", requestingAdminID, ticketID)
	w.WriteHeader(http.StatusNoContent)
}
