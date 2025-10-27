package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"kivendi-backend/config"
)

// ContactFormRequest représente la structure des données attendues du formulaire de contact Flutter.
type ContactFormRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// ContactFormResponse représente la réponse renvoyée au client.
type ContactFormResponse struct {
	Message string `json:"message"`
	ID      int64  `json:"id,omitempty"`
}

func SubmitContactFormHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la soumission du formulaire de contact.")

	// 1. Décodage de la requête JSON
	var req ContactFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur: Corps de requête invalide: %v", err)
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// 2. Validation des champs (simple validation de présence)
	if req.Name == "" || req.Email == "" || req.Message == "" {
		log.Println("Erreur: Champs Nom, Email ou Message manquants.")
		http.Error(w, "Tous les champs (nom, email, message) sont requis", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO support_tickets (user_name, user_email, message, status, created_at)
		VALUES ($1, $2, $3, 'Nouveau', $4)
		RETURNING id
	`

	var ticketID int64
	err := config.DB.QueryRowContext(
		r.Context(),
		query,
		req.Name,
		req.Email,
		req.Message,
		time.Now(),
	).Scan(&ticketID)

	if err != nil {
		log.Printf("Erreur lors de l'insertion du ticket de support: %v", err)
		http.Error(w, "Erreur interne lors de l'enregistrement de la requête", http.StatusInternalServerError)
		return
	}

	log.Printf("Requête de support enregistrée avec succès. ID: %d", ticketID)

	// 4. Envoi de la réponse de succès
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // HTTP 201 Created pour une nouvelle ressource

	response := ContactFormResponse{
		Message: "Votre message a été envoyé avec succès. Nous vous répondrons dans les plus brefs délais.",
		ID:      ticketID,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		// Même si l'encodage échoue, l'action est réussie, donc loguer l'erreur seulement.
	}
}

// SupportContact représente la structure des informations de contact du site.
type SupportContact struct {
	SupportEmail   string    `json:"support_email"`
	ContactPhone   string    `json:"contact_phone"`
	WhatsappNumber string    `json:"whatsapp_number"`
	FacebookURL    string    `json:"facebook_url"`
	InstagramURL   string    `json:"instagram_url"`
	LastUpdated    time.Time `json:"last_updated"`
}

// GetSupportContactHandler gère la requête GET pour récupérer les informations de contact.
func GetSupportContactHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête pour les informations de contact.")

	// La requête SELECT pour récupérer les informations de la table.
	// Nous utilisons LIMIT 1 car cette table est censée contenir une seule ligne.
	query := `
		SELECT 
			support_email, 
			contact_phone, 
			whatsapp_number, 
			facebook_url, 
			instagram_url, 
			last_updated
		FROM support_contact
		LIMIT 1
	`

	var contact SupportContact

	// Utilisation de QueryRowContext pour s'attendre à une seule ligne de résultat
	err := config.DB.QueryRowContext(r.Context(), query).Scan(
		&contact.SupportEmail,
		&contact.ContactPhone,
		&contact.WhatsappNumber,
		&contact.FacebookURL,
		&contact.InstagramURL,
		&contact.LastUpdated,
	)

	if err != nil {
		// sql.ErrNoRows est l'erreur si aucune ligne n'est trouvée.
		if err.Error() == "sql: no rows in result set" {
			log.Println("Erreur: Aucune information de contact trouvée.")
			// On retourne un 404 (Not Found) ou un 200 avec un objet vide selon la préférence
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "Informations de contact non configurées"})
			return
		}

		log.Printf("Erreur lors de la récupération des informations de contact: %v", err)
		http.Error(w, "Erreur interne lors de la récupération des données", http.StatusInternalServerError)
		return
	}

	log.Println("Informations de contact récupérées avec succès.")

	// Envoi de la réponse JSON de succès
	w.Header().Set("Content-Type", "application/json")
	// On utilise http.StatusOK (200) pour une récupération réussie
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(contact); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
	}
}
