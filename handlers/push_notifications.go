package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"kivendi-backend/config"
)

// RegisterTokenRequest structure pour la requête d'enregistrement du token
type RegisterTokenRequest struct {
	Token      string `json:"token"`
	DeviceType string `json:"device_type"` // 'android', 'ios', 'web'
}

// RegisterDeviceTokenHandler gère l'enregistrement d'un token de notification push
func RegisterDeviceTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer l'ID utilisateur du contexte (fourni par le middleware JWT)
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("❌ [PUSH] ID utilisateur non trouvé dans le contexte")
		http.Error(w, "Non autorisé", http.StatusUnauthorized)
		return
	}

	// 1. Décoder la requête
	var req RegisterTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ [PUSH] Erreur de décodage JSON: %v", err)
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	// 2. Valider les entrées
	token := strings.TrimSpace(req.Token)
	deviceType := strings.ToLower(strings.TrimSpace(req.DeviceType))

	if token == "" {
		log.Println("❌ [PUSH] Token manquant")
		http.Error(w, "Le champ 'token' est requis", http.StatusBadRequest)
		return
	}

	if deviceType != "android" && deviceType != "ios" && deviceType != "web" {
		log.Printf("❌ [PUSH] DeviceType invalide: %s", deviceType)
		http.Error(w, "Le champ 'device_type' doit être 'android', 'ios' ou 'web'", http.StatusBadRequest)
		return
	}

	log.Printf("🔵 [PUSH] Enregistrement du token pour UserID: %d, Type: %s", userID, deviceType)

	// 3. Insérer ou Mettre à jour le token dans la base de données
	// Nous utilisons "ON CONFLICT (token) DO UPDATE" pour gérer les cas où :
	// 1. Le token existe déjà pour cet utilisateur (met à jour updated_at).
	// 2. Le token existe mais pour un AUTRE utilisateur (ex: déconnexion/reconnexion).
	//    Dans ce cas, le token est ré-assigné au nouvel utilisateur.
	query := `
		INSERT INTO device_tokens (user_id, token, device_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (token) DO UPDATE
		SET user_id = EXCLUDED.user_id,
			device_type = EXCLUDED.device_type,
			updated_at = $4
	`

	_, err := config.DB.Exec(query, userID, token, deviceType, time.Now())
	if err != nil {
		log.Printf("❌ [PUSH] Erreur lors de l'enregistrement du token en BDD: %v", err)
		http.Error(w, "Erreur lors de l'enregistrement du token", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [PUSH] Token enregistré/mis à jour avec succès pour UserID: %d", userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Token enregistré avec succès",
	})
}
