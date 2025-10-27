package jobs

import (
	"log"
	"time"

	"kivendi-backend/config"
)

// DeactivateExpiredBoosts désactive les boosts expirés et met à jour le statut des annonces
func DeactivateExpiredBoosts() {
	log.Println("Début de la désactivation des boosts expirés...")

	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur lors du début de la transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// 1. Mettre à jour les boosts expirés (is_active = false)
	updateBoostsQuery := `
		UPDATE ad_boosts 
		SET is_active = FALSE, updated_at = NOW()
		WHERE is_active = TRUE AND end_date <= NOW()
	`
	result, err := tx.Exec(updateBoostsQuery)
	if err != nil {
		log.Printf("Erreur lors de la désactivation des boosts expirés: %v", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("%d boosts expirés désactivés", rowsAffected)

	// 2. Mettre à jour les annonces qui n'ont plus de boost actif
	updateAdsQuery := `
		UPDATE ads 
		SET is_boosted = FALSE, updated_at = NOW()
		WHERE is_boosted = TRUE 
		AND id NOT IN (
			SELECT ad_id 
			FROM ad_boosts 
			WHERE is_active = TRUE AND end_date > NOW()
		)
	`
	result, err = tx.Exec(updateAdsQuery)
	if err != nil {
		log.Printf("Erreur lors de la mise à jour des annonces: %v", err)
		return
	}

	adsUpdated, _ := result.RowsAffected()
	log.Printf("%d annonces mises à jour (is_boosted = false)", adsUpdated)

	// Valider la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur lors de la validation de la transaction: %v", err)
		return
	}

	log.Println("Désactivation des boosts expirés terminée avec succès")
}

// StartBoostCleanupJob démarre un job périodique pour nettoyer les boosts expirés
func StartBoostCleanupJob() {
	// Exécuter immédiatement au démarrage
	DeactivateExpiredBoosts()

	// Puis exécuter toutes les heures
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			DeactivateExpiredBoosts()
		}
	}()

	log.Println("Job de nettoyage des boosts démarré (exécution toutes les heures)")
}
