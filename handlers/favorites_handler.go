package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"kivendi-backend/config"
	"kivendi-backend/models"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

// AddFavoriteHandler gère l'ajout d'une annonce aux favoris de l'utilisateur.
func AddFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	var payload struct {
		AdID int `json:"ad_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Corps de requête invalide", http.StatusBadRequest)
		return
	}

	// Vérifier si l'annonce existe avant de l'ajouter
	var adExists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM ads WHERE id = $1)", payload.AdID).Scan(&adExists)
	if err != nil {
		log.Printf("Erreur lors de la vérification de l'annonce : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if !adExists {
		http.Error(w, "L'annonce spécifiée n'existe pas", http.StatusNotFound)
		return
	}

	_, err = config.DB.Exec("INSERT INTO favorites (user_id, ad_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, payload.AdID)
	if err != nil {
		log.Printf("Erreur lors de l'ajout aux favoris : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Annonce ajoutée aux favoris"})
}

// RemoveFavoriteHandler gère la suppression d'une annonce des favoris.
func RemoveFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Extraire l'ID de l'annonce depuis le chemin de l'URL
	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// 2. Récupérer l'ID de l'utilisateur depuis le contexte (mis en place par le middleware)
	userID := r.Context().Value(userIDContextKey).(int)

	// 3. Effectuer la suppression dans la base de données
	result, err := config.DB.Exec("DELETE FROM favorites WHERE user_id = $1 AND ad_id = $2", userID, adID)
	if err != nil {
		log.Printf("Erreur lors du retrait des favoris : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "L'annonce n'était pas dans les favoris", http.StatusNotFound)
		return
	}

	// 4. Si tout est bon, répondre avec succès
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Favori supprimé avec succès"))
}

// GetFavoritesHandler récupère toutes les annonces favorites de l'utilisateur actuel.
func GetFavoritesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	rows, err := config.DB.Query(`
        SELECT 
            a.id, a.title, a.description, a.price, a.images, a.form_data, 
            a.city, a.phone_number, a.is_phone_visible, a.latitude, a.longitude, a.created_at,
            u.first_name, u.last_name, u.shop_name, u.account_type
        FROM favorites f
        JOIN ads a ON f.ad_id = a.id
        JOIN users u ON a.user_id = u.id
        WHERE f.user_id = $1
        ORDER BY f.created_at DESC
    `, userID)
	if err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []models.Ad

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var formDataStr sql.NullString
		var firstName, lastName, accountType string
		var shopName sql.NullString

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
			&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &ad.Latitude, &ad.Longitude, &ad.CreatedAt,
			&firstName, &lastName, &shopName, &accountType,
		)
		if err != nil {
			log.Printf("Erreur lors de la lecture des lignes des favoris: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}

		ad.Images = []string(images)

		if formDataStr.Valid {
			err = json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
			if err != nil {
				ad.FormData = nil
			}
		}

		if accountType == "Professionnel" {
			ad.User.IsProAccount = true
			ad.User.ShopName = shopName
			if shopName.Valid {
				ad.User.DisplayName = shopName.String
			}
		} else {
			ad.User.IsProAccount = false
			ad.User.FirstName = firstName
			ad.User.LastName = lastName
			ad.User.DisplayName = firstName + " " + lastName
		}

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// ⭐ AJOUT DE CETTE CONDITION CLÉ ⭐
	// S'il n'y a pas de favoris, renvoyer un tableau JSON vide.
	if len(ads) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ads); err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
}
