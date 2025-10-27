// Dans votre fichier services/notification_service.go (exemple)

package services

import (
	"encoding/json"
	"kivendi-backend/config"
	"log"
)

// CreateNotification enregistre une notification dans la base de données pour un utilisateur.
func CreateNotification(userID int, notifType, title, message string, data map[string]interface{}) {

	// Vérifier si l'utilisateur a activé ce type de notification
	// (Logique à implémenter si vous avez une table de préférences de notification)

	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Erreur lors de la sérialisation des données de notification pour l'utilisateur %d: %v", userID, err)
		// Utiliser une chaîne vide ou '{}' en cas d'erreur
		dataJSON = []byte("{}")
	}

	query := `
        INSERT INTO notifications (user_id, type, title, message, data)
        VALUES ($1, $2, $3, $4, $5)
    `
	_, err = config.DB.Exec(query, userID, notifType, title, message, dataJSON)
	if err != nil {
		log.Printf("Impossible de créer la notification pour l'utilisateur %d: %v", userID, err)
	} else {
		log.Printf("Notification de type '%s' créée pour l'utilisateur %d.", notifType, userID)
		// Ici, vous pourriez aussi déclencher une notification push si vous utilisez un service comme Firebase Cloud Messaging.
	}
}
