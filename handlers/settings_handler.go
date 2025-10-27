package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"kivendi-backend/config"

	"github.com/gorilla/mux"
)

// BlockedUserResponse représente un utilisateur bloqué dans la réponse
type BlockedUserResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
	BlockedAt time.Time `json:"blocked_at"`
}

// GetBlockedUsersHandler récupère la liste des utilisateurs bloqués par l'utilisateur connecté
func GetBlockedUsersHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des utilisateurs bloqués.")

	// Récupérer l'ID utilisateur du contexte
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Requête pour récupérer tous les utilisateurs bloqués par cet utilisateur
	query := `
		SELECT 
			u.id,
			CASE 
				WHEN u.account_type = 'Professionnel' THEN COALESCE(u.shop_name, u.first_name || ' ' || u.last_name)
				ELSE u.first_name || ' ' || u.last_name
			END AS name,
			u.avatar_url,
			ub.created_at AS blocked_at
		FROM user_blocks ub
		JOIN users u ON ub.blocked_id = u.id
		WHERE ub.blocker_id = $1
		ORDER BY ub.created_at DESC
	`

	rows, err := config.DB.QueryContext(r.Context(), query, userID)
	if err != nil {
		log.Printf("Erreur lors de la récupération des utilisateurs bloqués: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var blockedUsers []BlockedUserResponse

	for rows.Next() {
		var user BlockedUserResponse
		var avatarURL sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.Name,
			&avatarURL,
			&user.BlockedAt,
		)
		if err != nil {
			log.Printf("Erreur lors du scan d'un utilisateur bloqué: %v", err)
			continue
		}

		if avatarURL.Valid {
			user.AvatarURL = &avatarURL.String
		}

		blockedUsers = append(blockedUsers, user)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Si aucun utilisateur bloqué, retourner un tableau vide
	if blockedUsers == nil {
		blockedUsers = []BlockedUserResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blockedUsers); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
	}

	log.Printf("Liste des utilisateurs bloqués récupérée avec succès pour l'utilisateur %d. Total: %d", userID, len(blockedUsers))
}

// UnblockUserByIDHandler gère le déblocage d'un utilisateur par son ID
func UnblockUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de déblocage d'utilisateur.")

	unblockerID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	blockedUserIDStr := vars["userID"]

	var blockedUserID int
	// Essayer une conversion directe en entier
	if _, err := fmt.Sscanf(blockedUserIDStr, "%d", &blockedUserID); err != nil {
		http.Error(w, "ID utilisateur invalide", http.StatusBadRequest)
		return
	}

	if unblockerID == blockedUserID {
		http.Error(w, "Vous ne pouvez pas vous débloquer vous-même", http.StatusBadRequest)
		return
	}

	result, err := config.DB.ExecContext(r.Context(),
		`DELETE FROM user_blocks 
		 WHERE blocker_id = $1 AND blocked_id = $2`,
		unblockerID, blockedUserID)

	if err != nil {
		log.Printf("Erreur lors du déblocage de l'utilisateur: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Aucun blocage trouvé pour cet utilisateur", http.StatusNotFound)
		return
	}

	log.Printf("Utilisateur %d a débloqué l'utilisateur %d avec succès.", unblockerID, blockedUserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Utilisateur débloqué avec succès",
	})
}

// DeleteAccountHandler gère la suppression définitive du compte utilisateur
func DeleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de suppression de compte.")

	// Récupérer l'ID utilisateur du contexte
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Décoder la requête pour obtenir la confirmation
	var requestBody struct {
		Confirmation string `json:"confirmation"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// Vérifier la confirmation
	if requestBody.Confirmation != "SUPPRIMER" {
		http.Error(w, "Confirmation invalide. Veuillez taper 'SUPPRIMER'", http.StatusBadRequest)
		return
	}

	// Commencer une transaction pour la suppression
	tx, err := config.DB.BeginTx(context.Background(), nil)
	if err != nil {
		log.Printf("Erreur lors du début de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// La suppression en cascade devrait gérer automatiquement :
	// - Les annonces (ads) -> messages, favoris, conversations liés
	// - Les messages de l'utilisateur
	// - Les favoris
	// - Les blocages (user_blocks)
	// - Les signalements (user_reports)
	// - Les ventes (sold_ads)

	// Supprimer l'utilisateur (les contraintes ON DELETE CASCADE s'occuperont du reste)
	_, err = tx.ExecContext(context.Background(),
		`DELETE FROM users WHERE id = $1`,
		userID)

	if err != nil {
		log.Printf("Erreur lors de la suppression de l'utilisateur: %v", err)
		http.Error(w, "Erreur lors de la suppression du compte", http.StatusInternalServerError)
		return
	}

	// Valider la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur lors de la validation de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Compte utilisateur %d supprimé avec succès.", userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Compte supprimé avec succès",
	})
}

// GetUserSettingsHandler récupère les paramètres de l'utilisateur
func GetUserSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Utiliser userID pour récupérer les settings depuis la DB
	var settings map[string]interface{}
	err := config.DB.QueryRow(`
		SELECT settings FROM user_settings WHERE user_id = $1
	`, userID).Scan(&settings)

	if err == sql.ErrNoRows {
		// Retourner des paramètres par défaut
		settings = map[string]interface{}{
			"notifications_enabled": true,
			"email_notifications":   true,
			"push_notifications":    true,
		}
	} else if err != nil {
		log.Printf("Erreur lors de la récupération des paramètres: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// UpdateUserSettingsHandler met à jour les paramètres de l'utilisateur
func UpdateUserSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	var settings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// Ici, vous pouvez sauvegarder les paramètres dans une table settings
	// Pour l'instant, on retourne juste un succès
	log.Printf("Paramètres mis à jour pour l'utilisateur %d: %+v", userID, settings)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Paramètres mis à jour avec succès",
	})
}
