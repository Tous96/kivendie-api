package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"kivendi-backend/config"
)

// DashboardStats définit la structure pour les statistiques du tableau de bord.
type DashboardStats struct {
	TotalUsers           int64             `json:"totalUsers"`
	NewUsersThisWeek     int64             `json:"newUsersThisWeek"`
	PendingAds           int64             `json:"pendingAds"`
	BoostedAds           int64             `json:"boostedAds"`
	PendingMessages      int64             `json:"pendingMessages"`
	TotalAds             int64             `json:"totalAds"`       // NOUVEAU
	RejectedAds          int64             `json:"rejectedAds"`    // NOUVEAU
	PendingReports       int64             `json:"pendingReports"` // NOUVEAU
	TransactionsPerMonth []TransactionData `json:"transactionsPerMonth"`
}

// TransactionData représente les données de transaction pour un mois donné.
type TransactionData struct {
	Month string  `json:"month"`
	Total float64 `json:"total"`
}

// GetDashboardStatsHandler récupère toutes les statistiques nécessaires pour le tableau de bord admin.
func GetDashboardStatsHandler(w http.ResponseWriter, r *http.Request) {
	var stats DashboardStats
	var wg sync.WaitGroup
	var errChan = make(chan error, 9) // Augmentation de la taille du canal

	// ... (les 5 premières goroutines restent identiques) ...
	// 1. Total des utilisateurs
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
		if err != nil {
			log.Printf("Erreur lors de la récupération du nombre total d'utilisateurs: %v", err)
			errChan <- err
		}
	}()

	// 2. Nouveaux utilisateurs cette semaine
	wg.Add(1)
	go func() {
		defer wg.Done()
		oneWeekAgo := time.Now().AddDate(0, 0, -7)
		err := config.DB.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= $1", oneWeekAgo).Scan(&stats.NewUsersThisWeek)
		if err != nil {
			log.Printf("Erreur lors de la récupération des nouveaux utilisateurs: %v", err)
			errChan <- err
		}
	}()

	// 3. Annonces en attente de validation
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE is_validated = FALSE AND is_rejected = FALSE AND is_deactivated = FALSE").Scan(&stats.PendingAds)
		if err != nil {
			log.Printf("Erreur lors de la récupération des annonces en attente: %v", err)
			errChan <- err
		}
	}()

	// 4. Annonces actuellement boostées
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM ad_boosts WHERE is_active = TRUE AND end_date > NOW()").Scan(&stats.BoostedAds)
		if err != nil {
			log.Printf("Erreur lors de la récupération des annonces boostées: %v", err)
			errChan <- err
		}
	}()

	// 5. Messages en attente (non lus)
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE is_read = FALSE").Scan(&stats.PendingMessages)
		if err != nil {
			log.Printf("Erreur lors de la récupération des messages en attente: %v", err)
			errChan <- err
		}
	}()

	// 6. Total des transactions par mois
	wg.Add(1)
	go func() {
		defer wg.Done()
		query := `
            SELECT
                TO_CHAR(DATE_TRUNC('month', created_at), 'YYYY-MM') AS month,
                SUM(amount) AS total
            FROM kkiapay_transactions
            WHERE status = 'SUCCESS' AND created_at >= DATE_TRUNC('month', NOW() - INTERVAL '11 months')
            GROUP BY month
            ORDER BY month;
        `
		rows, err := config.DB.Query(query)
		if err != nil {
			log.Printf("Erreur lors de la récupération des transactions par mois: %v", err)
			errChan <- err
			return
		}
		defer rows.Close()

		var transactions []TransactionData
		for rows.Next() {
			var data TransactionData
			if err := rows.Scan(&data.Month, &data.Total); err != nil {
				log.Printf("Erreur lors du scan des données de transaction: %v", err)
				errChan <- err
				return
			}
			transactions = append(transactions, data)
		}
		stats.TransactionsPerMonth = transactions
	}()

	// 7. NOUVEAU: Total des annonces
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM ads").Scan(&stats.TotalAds)
		if err != nil {
			log.Printf("Erreur lors de la récupération du nombre total d'annonces: %v", err)
			errChan <- err
		}
	}()

	// 8. NOUVEAU: Annonces rejetées
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE is_rejected = TRUE").Scan(&stats.RejectedAds)
		if err != nil {
			log.Printf("Erreur lors de la récupération des annonces rejetées: %v", err)
			errChan <- err
		}
	}()

	// 9. NOUVEAU: Signalements en attente
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := config.DB.QueryRow("SELECT COUNT(*) FROM user_reports WHERE status = 'pending'").Scan(&stats.PendingReports)
		if err != nil {
			log.Printf("Erreur lors de la récupération des signalements en attente: %v", err)
			errChan <- err
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			http.Error(w, "Erreur lors de la récupération des statistiques.", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Erreur lors de l'encodage de la réponse.", http.StatusInternalServerError)
	}
}
