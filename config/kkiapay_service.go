// config/kkiapay_service.go
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type KKiaPayConfig struct {
	PublicKey  string
	PrivateKey string
	Secret     string
	IsSandbox  bool
}

type KKiaPayTransactionStatus struct {
	TransactionID string  `json:"transactionId"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	State         string  `json:"state"`
	CreatedAt     string  `json:"createdAt"`
	PerformedAt   string  `json:"performedAt,omitempty"`
}

var KKiaPay *KKiaPayConfig

// InitKKiaPay initialise la configuration KKiaPay
func InitKKiaPay() {
	KKiaPay = &KKiaPayConfig{
		PublicKey:  os.Getenv("KKIAPAY_PUBLIC_KEY"),
		PrivateKey: os.Getenv("KKIAPAY_PRIVATE_KEY"),
		Secret:     os.Getenv("KKIAPAY_SECRET"),
		IsSandbox:  os.Getenv("KKIAPAY_SANDBOX") == "true",
	}

	log.Println("=== CONFIGURATION KKIAPAY ===")
	log.Printf("Public Key: %s", maskKey(KKiaPay.PublicKey))
	log.Printf("Private Key: %s", maskKey(KKiaPay.PrivateKey))
	log.Printf("Secret: %s", maskKey(KKiaPay.Secret))
	log.Printf("Sandbox: %v", KKiaPay.IsSandbox)
	log.Printf("Base URL: %s", KKiaPay.GetBaseURL())

	if KKiaPay.IsSandbox {
		log.Println("⚠️  MODE SANDBOX: Vérification simplifiée des transactions")
	}

	if KKiaPay.PrivateKey == "" {
		log.Println("⚠️  WARNING: KKIAPAY_PRIVATE_KEY is not set!")
	}
}

func maskKey(key string) string {
	if key == "" {
		return "[NOT SET]"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// GetBaseURL retourne l'URL de base selon l'environnement
func (k *KKiaPayConfig) GetBaseURL() string {
	if k.IsSandbox {
		return "https://api-sandbox.kkiapay.me/api/v1"
	}
	return "https://api.kkiapay.me/api/v1"
}

// VerifyTransaction vérifie le statut d'une transaction KKiaPay
func (k *KKiaPayConfig) VerifyTransaction(transactionID string) (*KKiaPayTransactionStatus, error) {
	log.Printf("=== VÉRIFICATION TRANSACTION KKIAPAY ===")
	log.Printf("Transaction ID: %s", transactionID)
	log.Printf("Mode: %s", map[bool]string{true: "SANDBOX", false: "PRODUCTION"}[k.IsSandbox])

	// En mode Sandbox, si la vérification API échoue, on accepte la transaction
	// car le widget sandbox peut ne pas persister les transactions dans l'API
	if k.IsSandbox {
		log.Println("🧪 MODE SANDBOX: Tentative de vérification avec fallback...")

		transaction, err := k.verifyTransactionWithRetry(transactionID)

		if err != nil {
			log.Printf("⚠️  Vérification API échouée en sandbox: %v", err)
			log.Println("✅ Acceptation de la transaction en mode sandbox (pour tests)")

			// Créer une transaction simulée pour le sandbox
			return &KKiaPayTransactionStatus{
				TransactionID: transactionID,
				Amount:        0, // Le montant sera vérifié côté application
				Status:        "SUCCESS",
				State:         "RECEIVED",
				CreatedAt:     time.Now().Format(time.RFC3339),
			}, nil
		}

		log.Println("✅ Transaction vérifiée via l'API")
		return transaction, nil
	}

	// En mode Production, la vérification est obligatoire
	log.Println("🔴 MODE PRODUCTION: Vérification stricte requise")
	return k.verifyTransactionWithRetry(transactionID)
}

// verifyTransactionWithRetry tente de vérifier une transaction avec plusieurs essais
func (k *KKiaPayConfig) verifyTransactionWithRetry(transactionID string) (*KKiaPayTransactionStatus, error) {
	var lastErr error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Tentative %d/%d...", attempt, maxRetries)

		transaction, err := k.verifyTransactionAttempt(transactionID)
		if err == nil {
			log.Printf("✅ Transaction vérifiée: Status=%s, State=%s",
				transaction.Status, transaction.State)
			return transaction, nil
		}

		lastErr = err
		log.Printf("⚠️  Tentative %d échouée: %v", attempt, err)

		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * time.Second
			log.Printf("Attente de %v avant le prochain essai...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return nil, fmt.Errorf("échec vérification après %d tentatives: %v", maxRetries, lastErr)
}

// verifyTransactionAttempt effectue une seule tentative de vérification
func (k *KKiaPayConfig) verifyTransactionAttempt(transactionID string) (*KKiaPayTransactionStatus, error) {
	url := fmt.Sprintf("%s/transactions/%s", k.GetBaseURL(), transactionID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("erreur création requête: %v", err)
	}

	req.Header.Set("x-api-key", k.PrivateKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur requête: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lecture réponse: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("statut HTTP %d: %s", resp.StatusCode, string(body))
	}

	var transaction KKiaPayTransactionStatus
	if err := json.Unmarshal(body, &transaction); err != nil {
		return nil, fmt.Errorf("erreur parsing JSON: %v", err)
	}

	return &transaction, nil
}

// RefundTransaction effectue un remboursement
func (k *KKiaPayConfig) RefundTransaction(transactionID string) error {
	log.Printf("=== DEMANDE DE REMBOURSEMENT ===")
	log.Printf("Transaction ID: %s", transactionID)

	if k.IsSandbox {
		log.Println("🧪 MODE SANDBOX: Simulation du remboursement")
		return nil
	}

	url := fmt.Sprintf("%s/transactions/%s/refund", k.GetBaseURL(), transactionID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("erreur création requête: %v", err)
	}

	req.Header.Set("x-api-key", k.PrivateKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erreur requête: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("échec remboursement: %s", string(body))
	}

	log.Printf("✅ Remboursement effectué avec succès")
	return nil
}
