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
	"kivendi-backend/models"
	"kivendi-backend/services"
	localwebsocket "kivendi-backend/websocket"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lib/pq"
)

// Définition de la clé de contexte pour l'ID utilisateur
var wsManager = localwebsocket.NewManager()
var notificationManager = localwebsocket.NewNotificationManager()
var awsService *services.AWSService

func InitAWSService() {
	var err error
	awsService, err = services.NewAWSService()
	if err != nil {
		log.Fatalf("Erreur d'initialisation du service AWS: %v", err)
	}
	log.Println("Service AWS initialisé avec succès.")
}

// InitPushService initialise le service de push et l'affecte au singleton exporté du package services
// À appeler depuis main.go au démarrage
func InitPushService() {
	var err error
	services.PushSvc, err = services.NewPushService(context.Background())
	if err != nil {
		log.Fatalf("Erreur d'initialisation du service Push (FCM): %v", err)
	}
	log.Println("Service Push (FCM) initialisé avec succès.")
}

// GetUserIDFromContext extrait l'ID de l'utilisateur du contexte de la requête.
func GetUserIDFromContext(r *http.Request) (int, bool) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	return userID, ok
}

// GetOrCreateConversation gère la création ou la récupération d'une conversation.
// GetOrCreateConversation - CORRECTION: Empêcher la création de conversations si blocage global
func GetOrCreateConversation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID, err := strconv.Atoi(vars["adID"])
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	buyerID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	var sellerID int
	err = config.DB.QueryRowContext(r.Context(),
		"SELECT user_id FROM ads WHERE id = $1", adID).Scan(&sellerID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur de base de données lors de la récupération de l'ID du vendeur: %v", err)
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		}
		return
	}

	if buyerID == sellerID {
		http.Error(w, "Vous ne pouvez pas créer une conversation avec votre propre annonce.", http.StatusBadRequest)
		return
	}

	// CORRECTION: Vérification de blocage GLOBAL (pas lié à une conversation spécifique)
	var blockCount int
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM user_blocks 
		 WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)`,
		buyerID, sellerID).Scan(&blockCount)

	if err != nil {
		log.Printf("Erreur de base de données lors de la vérification du statut de blocage: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	if blockCount > 0 {
		http.Error(w, "Impossible de créer une conversation : un blocage existe entre ces utilisateurs.", http.StatusForbidden)
		return
	}

	// Chercher une conversation existante entre ces deux utilisateurs pour cette annonce
	var conversationID int
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT id FROM conversations 
		 WHERE ad_id = $1 AND ((seller_id = $2 AND buyer_id = $3) OR (seller_id = $3 AND buyer_id = $2))`,
		adID, sellerID, buyerID).Scan(&conversationID)

	if err == sql.ErrNoRows {
		// Créer une nouvelle conversation
		err = config.DB.QueryRowContext(r.Context(),
			`INSERT INTO conversations (ad_id, seller_id, buyer_id) VALUES ($1, $2, $3) RETURNING id`,
			adID, sellerID, buyerID).Scan(&conversationID)
		if err != nil {
			log.Printf("Erreur de base de données lors de la création d'une nouvelle conversation: %v", err)
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		log.Printf("Erreur de base de données lors de la récupération de la conversation: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]int{"conversation_id": conversationID})
}

// GetConversationHistory gère la récupération de l'historique des messages d'une conversation.
func GetConversationHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	userID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	var count int
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM conversations WHERE id = $1 AND (seller_id = $2 OR buyer_id = $2)`,
		conversationID, userID).Scan(&count)
	if err != nil || count == 0 {
		http.Error(w, "Accès à la conversation non autorisé", http.StatusForbidden)
		return
	}

	rows, err := config.DB.QueryContext(r.Context(),
		`SELECT id, conversation_id, sender_id, text, offer_amount, type, created_at, is_read, image_urls
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC`,
		conversationID)
	if err != nil {
		log.Printf("Erreur de base de données lors de la récupération des messages: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		var imageURLs pq.StringArray

		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Text,
			&msg.OfferAmount, &msg.Type, &msg.CreatedAt, &msg.IsRead, &imageURLs); err != nil {
			log.Printf("Erreur de scan des messages: %v", err)
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		msg.ImageURLs = models.StringArray(imageURLs)
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur lors de l'itération des lignes de messages: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

// GetConversationListHandler gère la récupération de la liste des conversations pour un utilisateur.
func GetConversationListHandler(w http.ResponseWriter, r *http.Request) {
	userID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	// CORRECTION: Mettre à jour la requête SQL pour afficher le nom de la boutique si le compte est 'pro'
	// Sinon, afficher le nom et prénom de l'utilisateur
	rows, err := config.DB.QueryContext(r.Context(),
		`SELECT
			c.id,
			c.ad_id,
			CASE
				WHEN c.seller_id = $1 THEN
					CASE
						WHEN u2.account_type = 'Professionnel' THEN COALESCE(u2.shop_name, 'Nom de boutique inconnu')
						ELSE u2.first_name || ' ' || u2.last_name
					END
				ELSE
					CASE
						WHEN u1.account_type = 'Professionnel' THEN COALESCE(u1.shop_name, 'Nom de boutique inconnu')
						ELSE u1.first_name || ' ' || u1.last_name
					END
			END AS other_user_name,
			a.title AS ad_title,
			a.images[1] AS ad_image_url,
			c.created_at,
			CASE
				WHEN m.type = 'image' THEN 'Image partagée' -- Correction
				WHEN m.type = 'offer' THEN 'Offre: ' || m.offer_amount || ' FCFA' -- Correction: Ajouter l'offre ici
				ELSE m.text
			END AS last_message_text,
			m.offer_amount AS last_message_offer_amount,
			m.type AS last_message_type,
			m.created_at AS last_message_timestamp,
			(SELECT COUNT(*) FROM messages WHERE conversation_id = c.id AND sender_id != $1 AND is_read = false) AS unread_messages_count
		FROM conversations c
		JOIN ads a ON c.ad_id = a.id
		JOIN users u1 ON c.seller_id = u1.id
		JOIN users u2 ON c.buyer_id = u2.id
		LEFT JOIN messages m ON m.conversation_id = c.id AND m.created_at = (
			SELECT MAX(created_at) FROM messages WHERE conversation_id = c.id
		)
		WHERE (c.seller_id = $1 OR c.buyer_id = $1)
		AND NOT EXISTS (
			SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = $1 AND ub.blocked_id = CASE WHEN c.seller_id = $1 THEN c.buyer_id ELSE c.seller_id END)
			OR (ub.blocked_id = $1 AND ub.blocker_id = CASE WHEN c.seller_id = $1 THEN c.buyer_id ELSE c.seller_id END)
		)
		ORDER BY COALESCE(m.created_at, c.created_at) DESC`,
		userID)

	if err != nil {
		log.Printf("Erreur de base de données lors de la récupération de la liste des conversations: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var conversations []map[string]interface{}
	for rows.Next() {
		var convID, adID int
		var otherUserName, adTitle, adImageURL string
		var convCreatedAt time.Time
		var lastMessageText sql.NullString
		var lastMessageOfferAmount sql.NullFloat64
		var lastMessageType sql.NullString
		var lastMessageTimestamp sql.NullTime
		var unreadCount int

		if err := rows.Scan(
			&convID,
			&adID,
			&otherUserName,
			&adTitle,
			&adImageURL,
			&convCreatedAt,
			&lastMessageText,
			&lastMessageOfferAmount,
			&lastMessageType,
			&lastMessageTimestamp,
			&unreadCount); err != nil {
			log.Printf("Erreur de scan de la liste des conversations: %v", err)
			continue
		}

		convMap := map[string]interface{}{
			"id":                        convID,
			"ad_id":                     adID,
			"other_user_name":           otherUserName,
			"ad_title":                  adTitle,
			"ad_image_url":              adImageURL,
			"created_at":                convCreatedAt,
			"last_message_text":         lastMessageText.String,
			"last_message_offer_amount": lastMessageOfferAmount.Float64,
			"last_message_type":         lastMessageType.String,
			"last_message_timestamp":    lastMessageTimestamp.Time,
			"unread_messages_count":     unreadCount,
		}
		conversations = append(conversations, convMap)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur lors de l'itération des lignes de conversations: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(conversations)
}

// MarkMessagesAsReadHandler gère le marquage de tous les messages non lus d'une conversation comme lus.
func MarkMessagesAsReadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	userID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	result, err := config.DB.ExecContext(r.Context(),
		`UPDATE messages
		SET is_read = true
		WHERE conversation_id = $1 AND sender_id != $2`,
		conversationID, userID)
	if err != nil {
		log.Printf("Erreur de base de données lors de la mise à jour des messages: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("%d messages marqués comme lus dans la conversation %d.", rowsAffected, conversationID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Messages marqués comme lus."})
}

// HandleNotificationsWebSocket gère les connexions WebSocket pour les notifications génériques.
func HandleNotificationsWebSocket(w http.ResponseWriter, r *http.Request) {
	userID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	conn, err := notificationManager.GetUpgrader().Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Échec de la mise à niveau de la connexion WebSocket pour les notifications: %v", err)
		return
	}

	client := &localwebsocket.Client{Conn: conn}
	notificationManager.Register(userID, client)
	log.Printf("Connexion WebSocket pour les notifications établie pour l'utilisateur %d.", userID)

	defer func() {
		conn.Close()
		notificationManager.Unregister(userID)
		log.Printf("Déconnexion du client de notification pour l'utilisateur %d.", userID)
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Le client a fermé la connexion normalement.")
			} else {
				log.Printf("Erreur de lecture du message, fermeture de la connexion : %v", err)
			}
			break
		}
	}
}

// WebSocketHandler gère les connexions WebSocket pour une conversation spécifique.
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	userID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	// ✅ MODIFIÉ: Récupérer ad_id et le nom du sender (expéditeur) au début
	var sellerID, buyerID, adID int
	var senderName string

	// Récupérer les IDs de la conversation ET l'ID de l'annonce
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT seller_id, buyer_id, ad_id FROM conversations WHERE id = $1`,
		conversationID).Scan(&sellerID, &buyerID, &adID)
	if err != nil {
		http.Error(w, "Conversation non trouvée", http.StatusNotFound)
		return
	}

	if userID != sellerID && userID != buyerID {
		http.Error(w, "Accès à la conversation non autorisé", http.StatusForbidden)
		return
	}

	// Récupérer le nom du sender (pour les notifications)
	// Utiliser COALESCE pour gérer le shop_name des pros, sinon le prénom
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT 
			CASE
				WHEN account_type = 'Professionnel' AND shop_name IS NOT NULL AND shop_name != '' THEN shop_name
				ELSE first_name
			END 
		 FROM users WHERE id = $1`,
		userID).Scan(&senderName)

	if err != nil {
		log.Printf("Erreur récupération nom sender (ID: %d): %v", userID, err)
		http.Error(w, "Erreur récupération données utilisateur", http.StatusInternalServerError)
		return
	}

	// Définir l'ID du destinataire (otherUserID)
	var otherUserID int
	if userID == sellerID {
		otherUserID = buyerID
	} else {
		otherUserID = sellerID
	}

	// CORRECTION: Vérifier si l'utilisateur est bloqué AVANT d'établir la connexion WebSocket
	var blockCount int
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM user_blocks 
		 WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)`,
		userID, otherUserID).Scan(&blockCount)

	if err != nil {
		log.Printf("Erreur lors de la vérification du blocage: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	if blockCount > 0 {
		http.Error(w, "Connexion refusée : un blocage existe entre ces utilisateurs.", http.StatusForbidden)
		return
	}

	conn, err := wsManager.GetUpgrader().Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Échec de la mise à niveau de la connexion WebSocket: %v", err)
		return
	}
	client := &localwebsocket.Client{Conn: conn}

	wsManager.Register(conversationID, client)
	log.Printf("Connexion WebSocket établie pour la conversation %d.", conversationID)

	defer func() {
		conn.Close()
		wsManager.Unregister(conversationID, client)
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Le client a fermé la connexion normally.")
			} else {
				log.Printf("Erreur de lecture du message, fermeture de la connexion : %v", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			// CORRECTION: Vérifier à nouveau le blocage avant de traiter chaque message
			var currentBlockCount int
			err = config.DB.QueryRowContext(context.Background(),
				`SELECT COUNT(*) FROM user_blocks 
				 WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)`,
				userID, otherUserID).Scan(&currentBlockCount)

			if err != nil {
				log.Printf("Erreur lors de la vérification du blocage: %v", err)
				continue
			}

			if currentBlockCount > 0 {
				log.Printf("Message rejeté : utilisateur %d est bloqué", userID)
				continue
			}

			var incomingMessage models.IncomingMessage
			err = json.Unmarshal(message, &incomingMessage)
			if err != nil {
				log.Printf("Erreur de désérialisation du message: %v", err)
				continue
			}

			log.Printf("Message JSON reçu: %+v", incomingMessage)

			// Validation des messages
			if incomingMessage.Type == "text" && incomingMessage.Text == "" {
				log.Printf("Message de texte vide, ignoré.")
				continue
			}
			if incomingMessage.Type == "image" && len(incomingMessage.Images) == 0 {
				log.Printf("Message d'image sans images, ignoré.")
				continue
			}

			// Créer le message à sauvegarder
			msgToSave := models.Message{
				SenderID:       strconv.Itoa(userID),
				ConversationID: conversationID,
				Text:           incomingMessage.Text,
				OfferAmount:    incomingMessage.OfferAmount,
				Type:           incomingMessage.Type,
				CreatedAt:      time.Now(),
				IsRead:         false,
			}

			// Traitement spécifique pour les images
			if incomingMessage.Type == "image" && awsService != nil {
				log.Printf("Traitement de %d images", len(incomingMessage.Images))

				imageURLs, err := awsService.UploadBase64Images(incomingMessage.Images)
				if err != nil {
					log.Printf("Erreur lors de l'upload des images: %v", err)
					continue
				}

				if len(imageURLs) == 0 {
					log.Printf("Aucune image uploadée avec succès")
					continue
				}

				msgToSave.ImageURLs = models.StringArray(imageURLs)
				log.Printf("Images uploadées: %v", imageURLs)
			}

			// Sauvegarder le message dans la base de données
			log.Println("Début de l'insertion du message dans la base de données.")
			var lastInsertID int
			err = config.DB.QueryRowContext(context.Background(),
				`INSERT INTO messages (conversation_id, sender_id, text, offer_amount, type, created_at, is_read, image_urls)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
				msgToSave.ConversationID, msgToSave.SenderID, msgToSave.Text,
				msgToSave.OfferAmount, msgToSave.Type, msgToSave.CreatedAt,
				msgToSave.IsRead, pq.Array(msgToSave.ImageURLs),
			).Scan(&lastInsertID)

			if err != nil {
				log.Printf("Erreur de base de données lors de l'insertion du message: %v", err)
				continue
			}
			msgToSave.ID = lastInsertID
			log.Printf("Message inséré avec succès. Nouvel ID: %d", msgToSave.ID)

			// ✅ ================================================================
			// ✅ DÉBUT DE L'AJOUT POUR LES NOTIFICATIONS PUSH
			// ✅ ================================================================
			// 👇 ASSUREZ-VOUS QUE "kivendi-backend/services" EST DANS VOS IMPORTS EN HAUT DU FICHIER

			if services.PushSvc != nil {
				// Exécuter l'envoi de la notification dans une goroutine
				// pour ne pas bloquer la boucle de chat.
				go func(msg models.Message) {
					log.Printf("[Push] Lancement de l'envoi de notif pour la conv %d à l'utilisateur %d", conversationID, otherUserID)

					// 👇 MODIFICATION ICI: Utilisation de services.PushSvc
					err := services.PushSvc.SendChatMessagePush(
						context.Background(), // Utiliser un nouveau contexte
						otherUserID,
						senderName,
						&msg,
						conversationID,
						adID,
					)
					if err != nil {
						// Loguer l'erreur, mais ne pas planter le serveur
						log.Printf("[Push] Erreur lors de l'envoi de la notif (conv %d): %v", conversationID, err)
					}
				}(msgToSave) // Passer une copie de msgToSave à la goroutine
			} else {
				// 👇 MODIFICATION ICI: Le log est plus précis
				// (Cette erreur ne devrait plus arriver si main.go est correct)
				log.Println("[Push] CRITIQUE: services.PushSvc (le service global) n'est pas initialisé.")
			}
			// ✅ FIN DE L'AJOUT
			// ✅ ================================================================

			// Envoyer notification à l'autre utilisateur
			notificationMessage := map[string]interface{}{
				"type":            "new_message_notification",
				"conversation_id": conversationID,
			}
			notificationBytes, _ := json.Marshal(notificationMessage)
			notificationManager.Notify(otherUserID, notificationBytes)
			log.Printf("Notification envoyée à l'autre utilisateur de la conversation %d.", conversationID)

			savedMessageBytes, _ := json.Marshal(msgToSave)
			log.Printf("Message re-sérialisé pour la diffusion: %s", string(savedMessageBytes))

			// Diffuser le message à tous les clients de la conversation
			wsManager.Broadcast(conversationID, savedMessageBytes)
		}
	}
}

// BlockUserHandler gère le blocage d'un utilisateur
func BlockUserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	blockerID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	// Vérifier que l'utilisateur fait partie de la conversation
	var sellerID, buyerID int
	err = config.DB.QueryRowContext(r.Context(),
		"SELECT seller_id, buyer_id FROM conversations WHERE id = $1",
		conversationID).Scan(&sellerID, &buyerID)
	if err != nil {
		http.Error(w, "Conversation non trouvée", http.StatusNotFound)
		return
	}

	if blockerID != sellerID && blockerID != buyerID {
		http.Error(w, "Accès à la conversation non autorisé", http.StatusForbidden)
		return
	}

	// Déterminer l'ID de l'utilisateur à bloquer
	var blockedID int
	if blockerID == sellerID {
		blockedID = buyerID
	} else {
		blockedID = sellerID
	}

	// 🆕 NOUVEAU: Vérifier si le blocage existe déjà de manière globale
	var existingID int
	err = config.DB.QueryRowContext(r.Context(),
		"SELECT blocker_id FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2",
		blockerID, blockedID).Scan(&existingID)

	if err == nil {
		json.NewEncoder(w).Encode(map[string]string{"message": "Utilisateur déjà bloqué"})
		return
	}

	if err != sql.ErrNoRows {
		log.Printf("Erreur lors de la vérification du blocage existant: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	// Créer le blocage
	_, err = config.DB.ExecContext(r.Context(),
		`INSERT INTO user_blocks (blocker_id, blocked_id, created_at) 
         VALUES ($1, $2, $3)`,
		blockerID, blockedID, time.Now())

	if err != nil {
		log.Printf("Erreur lors du blocage de l'utilisateur: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Utilisateur %d a bloqué l'utilisateur %d de manière permanente", blockerID, blockedID)
	json.NewEncoder(w).Encode(map[string]string{"message": "Utilisateur bloqué avec succès"})
}

// UnblockUserHandler gère le déblocage d'un utilisateur
func UnblockUserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	unblockerID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	// 🆕 NOUVEAU: Trouver l'ID de l'utilisateur qui a été bloqué
	var sellerID, buyerID int
	err = config.DB.QueryRowContext(r.Context(),
		"SELECT seller_id, buyer_id FROM conversations WHERE id = $1",
		conversationID).Scan(&sellerID, &buyerID)
	if err != nil {
		http.Error(w, "Conversation non trouvée", http.StatusNotFound)
		return
	}

	var blockedID int
	if unblockerID == sellerID {
		blockedID = buyerID
	} else {
		blockedID = sellerID
	}

	// Supprimer le blocage
	result, err := config.DB.ExecContext(r.Context(),
		`DELETE FROM user_blocks 
         WHERE blocker_id = $1 AND blocked_id = $2`,
		unblockerID, blockedID)

	if err != nil {
		log.Printf("Erreur lors du déblocage de l'utilisateur: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Aucun blocage trouvé", http.StatusNotFound)
		return
	}

	log.Printf("Utilisateur %d a débloqué l'utilisateur %d.", unblockerID, blockedID)
	json.NewEncoder(w).Encode(map[string]string{"message": "Utilisateur débloqué avec succès"})
}

// ReportUserHandler gère le signalement d'un utilisateur
func ReportUserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	reporterID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	// Parse le corps de la requête pour obtenir la raison
	var requestBody struct {
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// Vérifier que l'utilisateur fait partie de la conversation
	var sellerID, buyerID int
	err = config.DB.QueryRowContext(r.Context(),
		"SELECT seller_id, buyer_id FROM conversations WHERE id = $1",
		conversationID).Scan(&sellerID, &buyerID)
	if err != nil {
		http.Error(w, "Conversation non trouvée", http.StatusNotFound)
		return
	}

	if reporterID != sellerID && reporterID != buyerID {
		http.Error(w, "Accès à la conversation non autorisé", http.StatusForbidden)
		return
	}

	// Déterminer l'ID de l'utilisateur signalé
	var reportedID int
	if reporterID == sellerID {
		reportedID = buyerID
	} else {
		reportedID = sellerID
	}

	// Créer le signalement
	_, err = config.DB.ExecContext(r.Context(),
		`INSERT INTO user_reports (reporter_id, reported_id, conversation_id, reason, created_at) 
		 VALUES ($1, $2, $3, $4, $5)`,
		reporterID, reportedID, conversationID, requestBody.Reason, time.Now())

	if err != nil {
		log.Printf("Erreur lors du signalement de l'utilisateur: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Utilisateur %d a signalé l'utilisateur %d dans la conversation %d pour: %s",
		reporterID, reportedID, conversationID, requestBody.Reason)
	json.NewEncoder(w).Encode(map[string]string{"message": "Utilisateur signalé avec succès"})
}

// CheckBlockStatusHandler vérifie si un utilisateur est bloqué dans une conversation
func CheckBlockStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	conversationID, err := strconv.Atoi(vars["conversationID"])
	if err != nil {
		http.Error(w, "ID de conversation invalide", http.StatusBadRequest)
		return
	}

	userID, exists := GetUserIDFromContext(r)
	if !exists {
		http.Error(w, "Utilisateur non authentifié", http.StatusUnauthorized)
		return
	}

	// 🆕 NOUVEAU: Trouver l'ID de l'autre utilisateur dans la conversation
	var otherUserID int
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT CASE WHEN seller_id = $1 THEN buyer_id ELSE seller_id END
         FROM conversations WHERE id = $2`,
		userID, conversationID).Scan(&otherUserID)

	if err != nil {
		log.Printf("Erreur lors de la récupération de l'autre utilisateur de la conversation: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	// 🆕 NOUVEAU: Vérifier le statut de blocage entre les deux utilisateurs, sans l'ID de la conversation
	var count int
	err = config.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM user_blocks 
		 WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)`,
		userID, otherUserID).Scan(&count)

	if err != nil {
		log.Printf("Erreur lors de la vérification du statut de blocage: %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"is_blocked":      count > 0,
		"conversation_id": conversationID,
	}

	json.NewEncoder(w).Encode(response)
}
