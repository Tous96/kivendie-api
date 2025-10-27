package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"kivendi-backend/config"
	"kivendi-backend/models"
	"kivendi-backend/services"
)

// generateResetToken génère un token de réinitialisation unique.
func generateResetToken() (string, error) {
	token, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return token.String(), nil
}

// ForgotPasswordHandler gère la demande de réinitialisation de mot de passe.
func ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la demande de mot de passe oublié")

	var requestData struct {
		Email string `json:"email"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		log.Printf("Erreur de décodage: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	// 1. Trouver l'utilisateur par email
	var user models.User
	err = config.DB.QueryRow(`
		SELECT id, email, is_verified, is_blocked
		FROM users WHERE email = $1
	`, requestData.Email).Scan(
		&user.ID,
		&user.Email,
		&user.IsVerified,
		&user.IsBlocked,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Tentative de réinitialisation pour un email non trouvé: %s", requestData.Email)
			// Répondre positivement pour ne pas révéler quels emails existent
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Si un compte est associé à cet email, un lien de réinitialisation a été envoyé.",
			})
			return
		}
		log.Printf("Erreur de base de données: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// 2. Vérifier si le compte est actif (non bloqué et vérifié)
	if user.IsBlocked {
		log.Printf("Tentative de réinitialisation pour un compte bloqué: %s", user.Email)
		w.WriteHeader(http.StatusOK) // Réponse positive pour la sécurité
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Si un compte est associé à cet email, un lien de réinitialisation a été envoyé.",
		})
		return
	}

	if !user.IsVerified {
		log.Printf("Tentative de réinitialisation pour un compte non vérifié: %s", user.Email)
		w.WriteHeader(http.StatusOK) // Réponse positive pour la sécurité
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Si un compte est associé à cet email, un lien de réinitialisation a été envoyé.",
		})
		return
	}

	// 3. Générer et stocker le token de réinitialisation
	token, err := generateResetToken()
	if err != nil {
		log.Printf("Erreur lors de la génération du token de réinitialisation: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Expiration dans 1 heure
	expiresAt := time.Now().Add(1 * time.Hour)

	_, err = config.DB.Exec(`
		UPDATE users 
		SET reset_token = $1, reset_token_expires_at = $2, updated_at = $3
		WHERE id = $4
	`, token, expiresAt, time.Now(), user.ID)

	if err != nil {
		log.Printf("Erreur lors du stockage du token de réinitialisation: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// 4. Envoyer l'email de réinitialisation
	// Vous devez créer cette fonction dans votre package services
	err = services.SendPasswordResetEmail(user.Email, token)
	if err != nil {
		log.Printf("Erreur lors de l'envoi de l'email de réinitialisation: %v", err)
		// Ne pas informer l'utilisateur de l'échec de l'email
	}

	log.Printf("Token de réinitialisation généré pour: %s", user.Email)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Si un compte est associé à cet email, un lien de réinitialisation a été envoyé.",
	})
}

// ResetPasswordHandler gère la réinitialisation effective du mot de passe.
func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la réinitialisation du mot de passe")

	var requestData struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		log.Printf("Erreur de décodage: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	if requestData.Token == "" || requestData.NewPassword == "" {
		http.Error(w, "Token et nouveau mot de passe sont requis", http.StatusBadRequest)
		return
	}

	// 1. Trouver l'utilisateur par le token de réinitialisation
	var user models.User
	var resetToken sql.NullString
	var expiresAt sql.NullTime

	err = config.DB.QueryRow(`
		SELECT id, email, reset_token, reset_token_expires_at
		FROM users WHERE reset_token = $1
	`, requestData.Token).Scan(
		&user.ID,
		&user.Email,
		&resetToken,
		&expiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Tentative de réinitialisation avec un token invalide: %s", requestData.Token)
			http.Error(w, "Token de réinitialisation invalide ou expiré", http.StatusUnauthorized)
			return
		}
		log.Printf("Erreur de base de données: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// 2. Vérifier si le token est valide et non expiré
	if !resetToken.Valid || !expiresAt.Valid || time.Now().After(expiresAt.Time) {
		log.Printf("Tentative de réinitialisation avec un token expiré pour: %s", user.Email)
		// Nettoyer le token expiré
		config.DB.Exec("UPDATE users SET reset_token = NULL, reset_token_expires_at = NULL WHERE id = $1", user.ID)
		http.Error(w, "Token de réinitialisation invalide ou expiré", http.StatusUnauthorized)
		return
	}

	// 3. Hacher le nouveau mot de passe
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erreur lors du hachage du nouveau mot de passe: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 4. Mettre à jour le mot de passe et effacer le token
	_, err = config.DB.Exec(`
		UPDATE users 
		SET password_hash = $1, reset_token = NULL, reset_token_expires_at = NULL, updated_at = $2
		WHERE id = $3
	`, string(hashedPassword), time.Now(), user.ID)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour du mot de passe: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	log.Printf("Mot de passe réinitialisé avec succès pour: %s", user.Email)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Mot de passe réinitialisé avec succès.",
	})
}
