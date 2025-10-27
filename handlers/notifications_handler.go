package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"kivendi-backend/config"

	"github.com/gorilla/mux"
)

// NotificationPreferences représente les préférences de notification d'un utilisateur
type NotificationPreferences struct {
	UserID                int  `json:"user_id"`
	NotificationsEnabled  bool `json:"notifications_enabled"`
	EmailNotifications    bool `json:"email_notifications"`
	PushNotifications     bool `json:"push_notifications"`
	MessageNotifications  bool `json:"message_notifications"`
	AdNotifications       bool `json:"ad_notifications"`
	FavoriteNotifications bool `json:"favorite_notifications"`
}

// Notification représente une notification utilisateur
type Notification struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Data      string    `json:"data,omitempty"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// GetNotificationPreferencesHandler récupère les préférences de notification de l'utilisateur
func GetNotificationPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	var prefs NotificationPreferences
	err := config.DB.QueryRowContext(r.Context(),
		`SELECT user_id, notifications_enabled, email_notifications, push_notifications,
		        message_notifications, ad_notifications, favorite_notifications
		 FROM notification_preferences WHERE user_id = $1`,
		userID).Scan(
		&prefs.UserID,
		&prefs.NotificationsEnabled,
		&prefs.EmailNotifications,
		&prefs.PushNotifications,
		&prefs.MessageNotifications,
		&prefs.AdNotifications,
		&prefs.FavoriteNotifications,
	)

	if err == sql.ErrNoRows {
		// Créer des préférences par défaut si elles n'existent pas
		prefs = NotificationPreferences{
			UserID:                userID,
			NotificationsEnabled:  true,
			EmailNotifications:    true,
			PushNotifications:     true,
			MessageNotifications:  true,
			AdNotifications:       true,
			FavoriteNotifications: true,
		}

		// Insérer les préférences par défaut
		_, err = config.DB.ExecContext(r.Context(),
			`INSERT INTO notification_preferences 
			 (user_id, notifications_enabled, email_notifications, push_notifications,
			  message_notifications, ad_notifications, favorite_notifications)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			prefs.UserID, prefs.NotificationsEnabled, prefs.EmailNotifications,
			prefs.PushNotifications, prefs.MessageNotifications,
			prefs.AdNotifications, prefs.FavoriteNotifications,
		)
		if err != nil {
			log.Printf("Erreur lors de la création des préférences par défaut: %v", err)
		}
	} else if err != nil {
		log.Printf("Erreur lors de la récupération des préférences: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

// UpdateNotificationPreferencesHandler met à jour les préférences de notification
func UpdateNotificationPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	var prefs NotificationPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// Mettre à jour ou insérer les préférences
	_, err := config.DB.ExecContext(r.Context(),
		`INSERT INTO notification_preferences 
		 (user_id, notifications_enabled, email_notifications, push_notifications,
		  message_notifications, ad_notifications, favorite_notifications)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (user_id) DO UPDATE SET
		   notifications_enabled = EXCLUDED.notifications_enabled,
		   email_notifications = EXCLUDED.email_notifications,
		   push_notifications = EXCLUDED.push_notifications,
		   message_notifications = EXCLUDED.message_notifications,
		   ad_notifications = EXCLUDED.ad_notifications,
		   favorite_notifications = EXCLUDED.favorite_notifications,
		   updated_at = CURRENT_TIMESTAMP`,
		userID, prefs.NotificationsEnabled, prefs.EmailNotifications,
		prefs.PushNotifications, prefs.MessageNotifications,
		prefs.AdNotifications, prefs.FavoriteNotifications,
	)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour des préférences: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Préférences mises à jour avec succès",
	})
}

// GetNotificationsHandler récupère toutes les notifications de l'utilisateur
func GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Paramètres de pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	unreadOnly := r.URL.Query().Get("unread_only") == "true"

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Construire la requête SQL
	query := `SELECT id, user_id, type, title, message, data, is_read, created_at
	          FROM notifications WHERE user_id = $1`

	if unreadOnly {
		query += " AND is_read = false"
	}

	query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"

	rows, err := config.DB.QueryContext(r.Context(), query, userID, limit, offset)
	if err != nil {
		log.Printf("Erreur lors de la récupération des notifications: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var notif Notification
		var data sql.NullString

		err := rows.Scan(
			&notif.ID,
			&notif.UserID,
			&notif.Type,
			&notif.Title,
			&notif.Message,
			&data,
			&notif.IsRead,
			&notif.CreatedAt,
		)
		if err != nil {
			log.Printf("Erreur lors du scan d'une notification: %v", err)
			continue
		}

		if data.Valid {
			notif.Data = data.String
		}

		notifications = append(notifications, notif)
	}

	if notifications == nil {
		notifications = []Notification{}
	}

	// Compter le nombre total de notifications
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	if unreadOnly {
		countQuery += " AND is_read = false"
	}
	config.DB.QueryRowContext(r.Context(), countQuery, userID).Scan(&totalCount)

	response := map[string]interface{}{
		"notifications": notifications,
		"pagination": map[string]interface{}{
			"current_page": page,
			"total_pages":  (totalCount + limit - 1) / limit,
			"total_count":  totalCount,
			"limit":        limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// MarkNotificationAsReadHandler marque une notification comme lue
func MarkNotificationAsReadHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	notifIDStr := vars["notificationID"]
	notifID, err := strconv.Atoi(notifIDStr)
	if err != nil {
		http.Error(w, "ID de notification invalide", http.StatusBadRequest)
		return
	}

	result, err := config.DB.ExecContext(r.Context(),
		`UPDATE notifications SET is_read = true, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND user_id = $2`,
		notifID, userID)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour de la notification: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Notification non trouvée", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Notification marquée comme lue",
	})
}

// MarkAllNotificationsAsReadHandler marque toutes les notifications comme lues
func MarkAllNotificationsAsReadHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	result, err := config.DB.ExecContext(r.Context(),
		`UPDATE notifications SET is_read = true, updated_at = CURRENT_TIMESTAMP
		 WHERE user_id = $1 AND is_read = false`,
		userID)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour des notifications: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Toutes les notifications ont été marquées comme lues",
		"rows_affected": rowsAffected,
	})
}

// DeleteNotificationHandler supprime une notification
func DeleteNotificationHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	notifIDStr := vars["notificationID"]
	notifID, err := strconv.Atoi(notifIDStr)
	if err != nil {
		http.Error(w, "ID de notification invalide", http.StatusBadRequest)
		return
	}

	result, err := config.DB.ExecContext(r.Context(),
		`DELETE FROM notifications WHERE id = $1 AND user_id = $2`,
		notifID, userID)

	if err != nil {
		log.Printf("Erreur lors de la suppression de la notification: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Notification non trouvée", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUnreadNotificationCountHandler récupère le nombre de notifications non lues
func GetUnreadNotificationCountHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	var count int
	err := config.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`,
		userID).Scan(&count)

	if err != nil {
		log.Printf("Erreur lors du comptage des notifications: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"unread_count": count,
	})
}

// CreateNotification est une fonction utilitaire pour créer une notification
func CreateNotification(ctx context.Context, userID int, notifType, title, message string, data map[string]interface{}) error {
	dataJSON, _ := json.Marshal(data)

	_, err := config.DB.ExecContext(ctx,
		`INSERT INTO notifications (user_id, type, title, message, data, is_read, created_at)
		 VALUES ($1, $2, $3, $4, $5, false, CURRENT_TIMESTAMP)`,
		userID, notifType, title, message, string(dataJSON))

	return err
}
