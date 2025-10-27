package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"kivendi-backend/config"
	"kivendi-backend/models"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// PushSvc est une instance globale du service de push, initialisée au démarrage.
var PushSvc *PushService

// PushService gère l'envoi de notifications push
type PushService struct {
	fcmClient *messaging.Client
}

// InitPushService initialise le client FCM global.
// Doit être appelé dans main.go au démarrage.
func InitPushService(ctx context.Context) error {
	var err error
	PushSvc, err = NewPushService(ctx)
	if err != nil {
		return fmt.Errorf("❌ échec de l'initialisation du service PushSvc: %w", err)
	}
	log.Println("✅ Service Push (PushSvc) initialisé globalement.")
	return nil
}

// NewPushService initialise le client FCM et retourne un nouveau service
func NewPushService(ctx context.Context) (*PushService, error) {
	// ✅ Vérifier que le fichier existe
	credPath := "./service-account.json"
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("❌ fichier service-account.json introuvable à %s", credPath)
	}

	log.Printf("📁 Chargement des credentials depuis: %s", credPath)

	opt := option.WithCredentialsFile(credPath)

	// =================================================================
	// Nous passons 'nil' pour la config.
	// Firebase lira le ProjectID directement depuis "service-account.json".
	// =================================================================
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("❌ erreur initialisation Firebase app: %w", err)
	}

	// Créer le client FCM
	fcmClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("❌ erreur initialisation FCM client: %w", err)
	}

	log.Println("✅ Service Push (FCM) initialisé avec succès")
	return &PushService{fcmClient: fcmClient}, nil
}

// SendChatMessagePush envoie une notification push pour un nouveau message de chat
func (s *PushService) SendChatMessagePush(
	ctx context.Context,
	recipientID int,
	senderName string,
	message *models.Message,
	conversationID int,
	adID int,
) error {
	// 1. Vérifier les préférences de notification du destinataire
	canSend, err := s.checkChatNotificationPreferences(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("erreur vérification préférences: %w", err)
	}

	if !canSend {
		log.Printf("[Push] L'utilisateur %d a désactivé les notifications push pour les messages.", recipientID)
		return nil
	}

	// 2. Récupérer les tokens de l'appareil du destinataire
	tokens, err := s.getDeviceTokens(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("erreur récupération tokens: %w", err)
	}

	if len(tokens) == 0 {
		log.Printf("[Push] Aucun token trouvé pour l'utilisateur %d.", recipientID)
		return nil
	}

	log.Printf("[Push] Envoi de notification à %d token(s) pour l'utilisateur %d", len(tokens), recipientID)

	// 3. Formater le message
	messageBody := s.formatMessageBody(message)

	// 4. ✅ SOLUTION : Envoyer message par message au lieu de multicast
	var successCount, failureCount int

	for _, token := range tokens {
		// Construire un message individuel avec notification heads-up
		notification := &messaging.Message{
			Token: token,
			Data: map[string]string{
				"type":            "chat_message",
				"conversation_id": fmt.Sprintf("%d", conversationID),
				"ad_id":           fmt.Sprintf("%d", adID),
				"sender_id":       message.SenderID,
				"sender_name":     senderName,
				"message_body":    messageBody,
				// "message_type":    message.Type, // Clé invalide renommée
				"chat_message_type": message.Type, // ✅ NOUVELLE CLE
				"title":             senderName,
				"body":              messageBody,
			},
			// Config pour iOS
			APNS: &messaging.APNSConfig{
				Headers: map[string]string{
					"apns-priority": "10",
				},
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Alert: &messaging.ApsAlert{
							Title: senderName,
							Body:  messageBody,
						},
						Sound:            "default",
						ContentAvailable: true,
						MutableContent:   true,
						Badge:            nil,
					},
				},
			},
			// Config pour Android - HEADS-UP NOTIFICATION
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Data: map[string]string{
					"type":            "chat_message",
					"conversation_id": fmt.Sprintf("%d", conversationID),
					"ad_id":           fmt.Sprintf("%d", adID),
					"sender_id":       message.SenderID,
					"sender_name":     senderName,
					"message_body":    messageBody,
					// "message_type":    message.Type, // Clé invalide renommée
					"chat_message_type": message.Type, // ✅ NOUVELLE CLE
				},
			},
		}

		// Envoyer le message
		_, err := s.fcmClient.Send(ctx, notification)
		if err != nil {
			failureCount++
			log.Printf("[Push] ❌ Échec envoi au token %s: %v", token[:20]+"...", err)

			// Vérifier si le token est invalide
			if messaging.IsRegistrationTokenNotRegistered(err) ||
				messaging.IsInvalidArgument(err) {
				log.Printf("[Push] Token invalide détecté, suppression...")
				s.deleteInvalidToken(context.Background(), token)
			}
		} else {
			successCount++
			log.Printf("[Push] ✅ Notification envoyée avec succès au token %s...", token[:20])
		}
	}

	// 5. Log du résumé
	if failureCount > 0 {
		log.Printf("[Push] ⚠️ %d notifications échouées sur %d", failureCount, len(tokens))
	}
	log.Printf("[Push] ✅ %d notifications envoyées avec succès à l'utilisateur %d", successCount, recipientID)
	if successCount == 0 && len(tokens) > 0 {
		return fmt.Errorf("toutes les notifications ont échoué (%d tokens)", len(tokens))
	}
	return nil
}

// checkChatNotificationPreferences vérifie si l'utilisateur souhaite recevoir des notifs de CHAT
func (s *PushService) checkChatNotificationPreferences(ctx context.Context, recipientID int) (bool, error) {
	var pushEnabled, messagesEnabled bool

	err := config.DB.QueryRowContext(ctx,
		`SELECT 
			COALESCE(push_notifications, true), 
			COALESCE(message_notifications, true)
		 FROM notification_preferences 
		 WHERE user_id = $1`,
		recipientID).Scan(&pushEnabled, &messagesEnabled)

	if err == sql.ErrNoRows {
		// Par défaut, les notifications sont activées
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return pushEnabled && messagesEnabled, nil
}

// ============== NOUVELLES FONCTIONS POUR LES NOTIFICATIONS D'ANNONCE ==============

// checkGeneralNotificationPreferences vérifie si l'utilisateur souhaite recevoir des notifs push (switch global).
func (s *PushService) checkGeneralNotificationPreferences(ctx context.Context, recipientID int) (bool, error) {
	var pushEnabled bool

	query := `SELECT COALESCE(push_notifications, true) 
			  FROM notification_preferences 
			  WHERE user_id = $1`

	// =================================================================
	// 👇 CORRECTION APPLIQUÉE ICI 👇
	// J'ai ajouté 'recipientID' comme argument pour le paramètre $1.
	// =================================================================
	err := config.DB.QueryRowContext(ctx, query, recipientID).Scan(&pushEnabled)

	if err == sql.ErrNoRows {
		// Par défaut, les notifications sont activées
		return true, nil
	}
	if err != nil {
		return false, err // Vraie erreur de DB
	}

	return pushEnabled, nil
}

// sendGenericPush envoie une notification push standard à un utilisateur.
// C'est la fonction de base pour les notifications d'annonces.
func (s *PushService) sendGenericPush(
	ctx context.Context,
	recipientID int,
	title string,
	body string,
	dataType string, // ex: "ad_validated", "ad_deactivated"
	data map[string]string,
) error {
	// 1. Vérifier les préférences (juste le switch global)
	canSend, err := s.checkGeneralNotificationPreferences(ctx, recipientID)
	if err != nil {
		// C'est ici que l'erreur se propageait
		return fmt.Errorf("erreur vérification préférences générales: %w", err)
	}
	if !canSend {
		log.Printf("[Push] L'utilisateur %d a désactivé les notifications push générales.", recipientID)
		return nil
	}

	// 2. Get tokens
	tokens, err := s.getDeviceTokens(ctx, recipientID)
	if err != nil {
		return fmt.Errorf("erreur récupération tokens: %w", err)
	}
	if len(tokens) == 0 {
		log.Printf("[Push] Aucun token trouvé pour l'utilisateur %d.", recipientID)
		return nil
	}

	log.Printf("[Push] Envoi de notification '%s' à %d token(s) for user %d", dataType, len(tokens), recipientID)

	// 3. Préparer la payload de données
	fullData := map[string]string{
		"type":  dataType,
		"title": title,
		"body":  body,
	}
	for k, v := range data {
		fullData[k] = v // Ajoute les données spécifiques (ex: adId)
	}

	// 4. Envoyer le message à tous les tokens
	var successCount, failureCount int
	for _, token := range tokens {
		msg := &messaging.Message{
			Token: token,
			Data:  fullData, // Data-only message pour la gestion en arrière-plan
			// Config iOS
			APNS: &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Alert: &messaging.ApsAlert{
							Title: title,
							Body:  body,
						},
						Sound:            "default",
						ContentAvailable: true, // Pour le traitement en arrière-plan
					},
				},
			},
			// Config Android (inclut une notif simple pour le cas où l'app est tuée)
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					Title: title,
					Body:  body,
					Sound: "default",
				},
			},
		}

		_, err := s.fcmClient.Send(ctx, msg)
		if err != nil {
			failureCount++
			log.Printf("[Push] ❌ Échec envoi '%s' au token %s: %v", dataType, token[:20]+"...", err)
			if messaging.IsRegistrationTokenNotRegistered(err) || messaging.IsInvalidArgument(err) {
				log.Printf("[Push] Token invalide détecté, suppression...")
				s.deleteInvalidToken(context.Background(), token)
			}
		} else {
			successCount++
		}
	}

	log.Printf("[Push] Résumé '%s' pour user %d: %d succès, %d échecs.", dataType, recipientID, successCount, failureCount)
	return nil
}

// SendAdValidatedPush envoie une notif push pour une annonce validée.
func (s *PushService) SendAdValidatedPush(ctx context.Context, recipientID int, adTitle string, adID int) {
	title := "Votre annonce a été approuvée !"
	body := fmt.Sprintf("Bonne nouvelle ! Votre annonce « %s » est maintenant visible par tous.", adTitle)
	data := map[string]string{
		"adId": fmt.Sprintf("%d", adID),
	}

	// Lancer en goroutine pour ne pas bloquer la requête HTTP
	go func() {
		err := s.sendGenericPush(context.Background(), recipientID, title, body, "ad_validated", data)
		if err != nil {
			log.Printf("[Push] Erreur envoi notif 'ad_validated' pour user %d: %v", recipientID, err)
		}
	}()
}

// SendAdRejectedPush envoie une notif push pour une annonce rejetée.
func (s *PushService) SendAdRejectedPush(ctx context.Context, recipientID int, adTitle string, adID int, reason string) {
	title := "Votre annonce a été rejetée"
	body := fmt.Sprintf("Votre annonce « %s » n'a pas pu être validée. Raison : %s", adTitle, reason)
	data := map[string]string{
		"adId": fmt.Sprintf("%d", adID),
	}

	go func() {
		err := s.sendGenericPush(context.Background(), recipientID, title, body, "ad_rejected", data)
		if err != nil {
			log.Printf("[Push] Erreur envoi notif 'ad_rejected' pour user %d: %v", recipientID, err)
		}
	}()
}

// SendAdDeactivatedPush envoie une notif push pour une annonce désactivée.
func (s *PushService) SendAdDeactivatedPush(ctx context.Context, recipientID int, adTitle string, adID int) {
	title := "Votre annonce a été désactivée"
	body := fmt.Sprintf("Votre annonce « %s » a été désactivée par un administrateur.", adTitle)
	data := map[string]string{
		"adId": fmt.Sprintf("%d", adID),
	}

	go func() {
		err := s.sendGenericPush(context.Background(), recipientID, title, body, "ad_deactivated", data)
		if err != nil {
			log.Printf("[Push] Erreur envoi notif 'ad_deactivated' pour user %d: %v", recipientID, err)
		}
	}()
}

// SendAdDeletedPush envoie une notif push pour une annonce supprimée.
func (s *PushService) SendAdDeletedPush(ctx context.Context, recipientID int, adTitle string) {
	title := "Votre annonce a été supprimée"
	body := fmt.Sprintf("Votre annonce « %s » a été supprimée par un administrateur.", adTitle)

	go func() {
		// nil data, car l'adID n'existe plus
		err := s.sendGenericPush(context.Background(), recipientID, title, body, "ad_deleted", nil)
		if err != nil {
			log.Printf("[Push] Erreur envoi notif 'ad_deleted' pour user %d: %v", recipientID, err)
		}
	}()
}

// ============================================================================

// getDeviceTokens récupère tous les tokens actifs pour un utilisateur
func (s *PushService) getDeviceTokens(ctx context.Context, recipientID int) ([]string, error) {
	rows, err := config.DB.QueryContext(ctx,
		`SELECT token FROM device_tokens WHERE user_id = $1`,
		recipientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

// deleteInvalidToken supprime un token de la BDD
func (s *PushService) deleteInvalidToken(ctx context.Context, token string) {
	_, err := config.DB.ExecContext(ctx, `DELETE FROM device_tokens WHERE token = $1`, token)
	if err != nil {
		log.Printf("[Push] Erreur suppression token invalide %s: %v", token[:20]+"...", err)
	} else {
		log.Printf("[Push] ✅ Token invalide %s... supprimé de la BDD", token[:20])
	}
}

// formatMessageBody crée un texte simple pour la notification
func (s *PushService) formatMessageBody(message *models.Message) string {
	switch message.Type {
	case "text":
		return message.Text
	case "image":
		return "📷 Image partagée"
	case "offer":
		if message.OfferAmount != nil {
			return fmt.Sprintf("Nouvelle offre : %.0f FCFA", *message.OfferAmount)
		}
		return "Nouvelle offre"
	default:
		return "Nouveau message"
	}
}
