// handlers/kkiapay_webhook_handler.go
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"kivendi-backend/config"
)

// KKiaPayWebhookPayload représente les données reçues du webhook KKiaPay
type KKiaPayWebhookPayload struct {
	Type          string  `json:"type"`
	TransactionID string  `json:"transactionId"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	State         string  `json:"state"`
	Reason        string  `json:"reason"`
	CreatedAt     string  `json:"createdAt"`
	PerformedAt   string  `json:"performedAt"`
	Country       string  `json:"country"`
	Currency      string  `json:"currency"`
}

// KKiaPayWebhookHandler gère les notifications webhook de KKiaPay
func KKiaPayWebhookHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Webhook KKiaPay reçu")

	// Lire le corps de la requête
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Erreur lecture body webhook: %v", err)
		http.Error(w, "Erreur de lecture", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Vérifier la signature HMAC
	signature := r.Header.Get("X-KKiaPay-Signature")
	if !verifyWebhookSignature(body, signature) {
		log.Println("Signature webhook invalide")
		http.Error(w, "Signature invalide", http.StatusUnauthorized)
		return
	}

	// Parser le payload
	var payload KKiaPayWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Erreur parsing webhook payload: %v", err)
		http.Error(w, "Payload invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Webhook Type: %s, TransactionID: %s, Status: %s, State: %s",
		payload.Type, payload.TransactionID, payload.Status, payload.State)

	// Traiter le webhook selon le type
	switch payload.Type {
	case "PAYMENT_SUCCEEDED":
		handlePaymentSucceeded(payload)
	case "PAYMENT_FAILED":
		handlePaymentFailed(payload)
	case "REFUND":
		handleRefund(payload)
	default:
		log.Printf("Type de webhook non géré: %s", payload.Type)
	}

	// Enregistrer le webhook dans la base de données pour audit
	logWebhookToDB(payload, body)

	// Répondre au webhook (important pour KKiaPay)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "received",
	})
}

// verifyWebhookSignature vérifie l'authenticité du webhook
func verifyWebhookSignature(body []byte, signature string) bool {
	// Créer un HMAC avec le secret KKiaPay
	h := hmac.New(sha256.New, []byte(config.KKiaPay.Secret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Comparer les signatures
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// handlePaymentSucceeded traite un paiement réussi
func handlePaymentSucceeded(payload KKiaPayWebhookPayload) {
	log.Printf("Paiement réussi: %s", payload.TransactionID)

	// Mettre à jour le statut dans la base de données si nécessaire
	query := `
		UPDATE ad_boosts 
		SET payment_status = 'completed', 
		    updated_at = NOW()
		WHERE transaction_id = $1 
		AND payment_status != 'completed'
	`

	result, err := config.DB.Exec(query, payload.TransactionID)
	if err != nil {
		log.Printf("Erreur mise à jour boost après webhook: %v", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Boost mis à jour suite au webhook pour transaction %s", payload.TransactionID)

		// Optionnel : Envoyer une notification à l'utilisateur
		// sendBoostActivationNotification(payload.TransactionID)
	}
}

// handlePaymentFailed traite un paiement échoué
func handlePaymentFailed(payload KKiaPayWebhookPayload) {
	log.Printf("Paiement échoué: %s - Raison: %s", payload.TransactionID, payload.Reason)

	// Marquer la transaction comme échouée
	query := `
		UPDATE ad_boosts 
		SET payment_status = 'failed',
		    is_active = FALSE,
		    updated_at = NOW()
		WHERE transaction_id = $1
	`

	_, err := config.DB.Exec(query, payload.TransactionID)
	if err != nil {
		log.Printf("Erreur mise à jour boost après échec: %v", err)
		return
	}

	// Optionnel : Notifier l'utilisateur de l'échec
	// sendPaymentFailedNotification(payload.TransactionID, payload.Reason)
}

// handleRefund traite un remboursement
func handleRefund(payload KKiaPayWebhookPayload) {
	log.Printf("Remboursement: %s", payload.TransactionID)

	// Désactiver le boost et marquer comme remboursé
	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur début transaction pour remboursement: %v", err)
		return
	}
	defer tx.Rollback()

	// Mettre à jour le boost
	_, err = tx.Exec(`
		UPDATE ad_boosts 
		SET payment_status = 'refunded',
		    is_active = FALSE,
		    updated_at = NOW()
		WHERE transaction_id = $1
	`, payload.TransactionID)
	if err != nil {
		log.Printf("Erreur mise à jour boost pour remboursement: %v", err)
		return
	}

	// Mettre à jour l'annonce
	_, err = tx.Exec(`
		UPDATE ads 
		SET is_boosted = FALSE,
		    updated_at = NOW()
		WHERE id IN (
			SELECT ad_id FROM ad_boosts 
			WHERE transaction_id = $1
		)
	`, payload.TransactionID)
	if err != nil {
		log.Printf("Erreur mise à jour annonce pour remboursement: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Erreur commit transaction remboursement: %v", err)
		return
	}

	log.Printf("Remboursement traité avec succès pour %s", payload.TransactionID)

	// Optionnel : Notifier l'utilisateur
	// sendRefundNotification(payload.TransactionID)
}

// logWebhookToDB enregistre le webhook dans la base pour audit
func logWebhookToDB(payload KKiaPayWebhookPayload, rawBody []byte) {
	query := `
		INSERT INTO kkiapay_transactions 
		(transaction_id, amount, status, state, raw_response, verified_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (transaction_id) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			state = EXCLUDED.state,
			raw_response = EXCLUDED.raw_response,
			updated_at = NOW()
	`

	var rawResponse map[string]interface{}
	json.Unmarshal(rawBody, &rawResponse)
	rawJSON, _ := json.Marshal(rawResponse)

	_, err := config.DB.Exec(query,
		payload.TransactionID,
		payload.Amount,
		payload.Status,
		payload.State,
		rawJSON,
	)

	if err != nil {
		log.Printf("Erreur sauvegarde webhook dans DB: %v", err)
	}
}

// GetWebhookLogsHandler récupère les logs des webhooks (pour le debug)
func GetWebhookLogsHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que l'utilisateur est admin (à implémenter selon votre logique)
	// userID := r.Context().Value(userIDContextKey).(int)
	// if !isAdmin(userID) { ... }

	query := `
		SELECT transaction_id, amount, status, state, verified_at, created_at
		FROM kkiapay_transactions
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("Erreur récupération logs webhook: %v", err)
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var log map[string]interface{}
		// Scanner les résultats
		// ... (à implémenter selon vos besoins)
		logs = append(logs, log)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
