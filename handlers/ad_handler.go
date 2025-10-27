package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq" // Importation nécessaire pour utiliser pq.Array

	"kivendi-backend/config"
	"kivendi-backend/models"
	"kivendi-backend/services"
)

// Déclaration de la clé de contexte pour l'ID de l'utilisateur.

// GetCategoriesWithSubCategories renvoie les catégories et leurs sous-catégories.
func GetCategoriesWithSubCategories(w http.ResponseWriter, r *http.Request) {
	// Création des tranches et cartes pour garantir qu'elles ne sont jamais nil
	categories := []models.Category{}
	subCategories := make(map[int][]models.SubCategory)

	rows, err := config.DB.Query(`
		SELECT id, name, icon FROM categories ORDER BY name
	`)
	if err != nil {
		log.Printf("Erreur lors de la récupération des catégories: %v", err)
		http.Error(w, "Erreur lors de la récupération des catégories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var cat models.Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Icon); err != nil {
			log.Printf("Erreur lors de la lecture des catégories: %v", err)
			http.Error(w, "Erreur lors de la lecture des catégories", http.StatusInternalServerError)
			return
		}
		categories = append(categories, cat)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Erreur lors de l'itération des catégories: %v", err)
		http.Error(w, "Erreur lors de l'itération des catégories", http.StatusInternalServerError)
		return
	}

	subCatRows, err := config.DB.Query(`
		SELECT id, name, icon, category_id FROM sub_categories ORDER BY name
	`)
	if err != nil {
		log.Printf("Erreur lors de la récupération des sous-catégories: %v", err)
		http.Error(w, "Erreur lors de la récupération des sous-catégories", http.StatusInternalServerError)
		return
	}
	defer subCatRows.Close()

	for subCatRows.Next() {
		var sub models.SubCategory
		if err := subCatRows.Scan(&sub.ID, &sub.Name, &sub.Icon, &sub.CategoryID); err != nil {
			log.Printf("Erreur lors de la lecture des sous-catégories: %v", err)
			http.Error(w, "Erreur lors de la lecture des sous-catégories", http.StatusInternalServerError)
			return
		}
		subCategories[sub.CategoryID] = append(subCategories[sub.CategoryID], sub)
	}
	if err = subCatRows.Err(); err != nil {
		log.Printf("Erreur lors de l'itération des sous-catégories: %v", err)
		http.Error(w, "Erreur lors de l'itération des sous-catégories", http.StatusInternalServerError)
		return
	}

	// Préparer la réponse
	response := struct {
		Categories    []models.Category            `json:"categories"`
		SubCategories map[int][]models.SubCategory `json:"subCategories"`
	}{
		Categories:    categories,
		SubCategories: subCategories,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
		return
	}
}

// CreateAdHandler gère la création d'une nouvelle annonce, y compris l'upload des images sur AWS S3.
func CreateAdHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de création d'annonce.")

	// Récupérer l'ID utilisateur du contexte de la requête
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte de la requête.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}
	log.Printf("ID utilisateur trouvé dans le contexte: %d", userID)

	// Limiter la taille de la requête pour éviter les attaques DoS
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB

	// Analyser la requête multipart pour les images et les champs de formulaire
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
		log.Printf("Erreur lors de l'analyse du formulaire multipart : %v", err)
		http.Error(w, "La requête est trop grande", http.StatusRequestEntityTooLarge)
		return
	}
	log.Println("Formulaire multipart analysé avec succès.")

	// Récupération des champs de formulaire
	title := r.FormValue("title")
	description := r.FormValue("description")
	priceStr := r.FormValue("price")
	subCategoryIDStr := r.FormValue("sub_category_id")
	formDataStr := r.Form.Get("form_data")
	city := r.FormValue("city")
	phoneNumber := r.FormValue("phone_number")
	isPhoneVisibleStr := r.FormValue("is_phone_visible")
	latitudeStr := r.FormValue("latitude")
	longitudeStr := r.FormValue("longitude")
	isDeliveryAvailableStr := r.FormValue("is_delivery_available")

	log.Printf("Valeurs reçues: title='%s', description='%s', price='%s', sub_category_id='%s', latitude='%s', longitude='%s', city='%s', phone_number='%s', is_phone_visible='%s', is_delivery_available='%s'",
		title, description, priceStr, subCategoryIDStr, latitudeStr, longitudeStr, city, phoneNumber, isPhoneVisibleStr, isDeliveryAvailableStr)

	// Vérification des champs obligatoires
	if title == "" || description == "" || priceStr == "" || subCategoryIDStr == "" || formDataStr == "" {
		log.Println("Erreur: Les champs obligatoires sont manquants.")
		http.Error(w, "Les champs obligatoires sont manquants.", http.StatusBadRequest)
		return
	}
	log.Println("Validation des champs de texte réussie.")

	// Convertir les chaînes en types appropriés
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		log.Printf("Erreur: Le prix '%s' n'est pas un nombre valide. Détails: %v", priceStr, err)
		http.Error(w, "Le prix n'est pas un nombre valide", http.StatusBadRequest)
		return
	}

	subCategoryID, err := strconv.Atoi(subCategoryIDStr)
	if err != nil {
		log.Printf("Erreur: L'ID de sous-catégorie '%s' n'est pas un nombre valide. Détails: %v", subCategoryIDStr, err)
		http.Error(w, "L'ID de sous-catégorie n'est pas un nombre valide", http.StatusBadRequest)
		return
	}

	isPhoneVisible, err := strconv.ParseBool(isPhoneVisibleStr)
	if err != nil {
		log.Printf("Erreur: La visibilité du téléphone '%s' n'est pas une valeur booléenne valide. Détails: %v", isPhoneVisibleStr, err)
		http.Error(w, "La visibilité du téléphone n'est pas une valeur booléenne valide", http.StatusBadRequest)
		return
	}

	isDeliveryAvailable, err := strconv.ParseBool(isDeliveryAvailableStr)
	if err != nil {
		log.Printf("Erreur: La disponibilité de la livraison '%s' n'est pas une valeur booléenne valide. Détails: %v", isDeliveryAvailableStr, err)
		http.Error(w, "La disponibilité de la livraison n'est pas une valeur booléenne valide", http.StatusBadRequest)
		return
	}

	log.Println("Conversion des types réussie pour le prix, l'ID de sous-catégorie et la visibilité du téléphone.")

	// Conversion des chaînes de latitude et longitude en float64
	var latitude, longitude *float64
	if latitudeStr != "" {
		lat, err := strconv.ParseFloat(latitudeStr, 64)
		if err != nil {
			log.Printf("Erreur: La latitude '%s' n'est pas un nombre valide. Détails: %v", latitudeStr, err)
			http.Error(w, "La latitude n'est pas un nombre valide", http.StatusBadRequest)
			return
		}
		latitude = &lat
	}
	if longitudeStr != "" {
		lon, err := strconv.ParseFloat(longitudeStr, 64)
		if err != nil {
			log.Printf("Erreur: La longitude '%s' n'est pas un nombre valide. Détails: %v", longitudeStr, err)
			http.Error(w, "La longitude n'est pas un nombre valide", http.StatusBadRequest)
			return
		}
		longitude = &lon
	}

	// Valider les données du formulaire JSON
	var formData map[string]interface{}
	if err := json.Unmarshal([]byte(formDataStr), &formData); err != nil {
		log.Printf("Erreur: Les données du formulaire JSON sont invalides. Détails: %v", err)
		http.Error(w, "Les données du formulaire sont invalides", http.StatusBadRequest)
		return
	}
	log.Println("Analyse du JSON du formulaire réussie.")

	// Récupérer les fichiers d'images
	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		log.Println("Erreur: Aucune image n'a été fournie.")
		http.Error(w, "Au moins une image est requise", http.StatusBadRequest)
		return
	}
	log.Printf("Début de l'upload de %d images vers S3 via le service AWS.", len(files))

	// Initialiser le service AWS
	awsService, err := services.NewAWSService()
	if err != nil {
		log.Printf("Erreur lors de l'initialisation du service AWS: %v", err)
		http.Error(w, "Erreur de configuration du service de stockage", http.StatusInternalServerError)
		return
	}

	// Uploader les images via le service AWS
	uploadedImageURLs, err := awsService.UploadAdMultipartImages(files)
	if err != nil {
		log.Printf("Erreur lors de l'upload des images: %v", err)
		http.Error(w, "Erreur lors de l'upload des images", http.StatusInternalServerError)
		return
	}

	log.Printf("Toutes les images ont été uploadées avec succès sur S3. Total: %d images", len(uploadedImageURLs))

	// Insérer l'annonce dans la base de données
	log.Println("Préparation de la requête SQL pour insérer l'annonce dans la base de données.")
	stmt, err := config.DB.PrepareContext(context.Background(), `
        INSERT INTO ads (title, description, price, sub_category_id, images, form_data, is_validated, is_deactivated, is_rejected, latitude, longitude, city, phone_number, is_phone_visible, is_delivery_available, user_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
        RETURNING id
    `)
	if err != nil {
		log.Printf("Erreur de préparation de la requête SQL : %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()
	log.Println("Requête SQL préparée.")

	// Enregistrer l'annonce dans la base de données
	var newAdID int
	log.Println("Exécution de la requête SQL pour insérer la nouvelle annonce.")
	err = stmt.QueryRow(
		title,
		description,
		price,
		subCategoryID,
		pq.Array(uploadedImageURLs),
		formDataStr,
		false, // is_validated
		false, // is_deactivated
		false, // is_rejected
		latitude,
		longitude,
		city,
		phoneNumber,
		isPhoneVisible,
		isDeliveryAvailable,
		userID,
	).Scan(&newAdID)
	if err != nil {
		log.Printf("Erreur lors de l'insertion de l'annonce dans la base de données : %v", err)

		// En cas d'erreur, essayer de supprimer les images uploadées
		log.Println("Tentative de suppression des images uploadées suite à l'échec de l'insertion...")
		if deleteErr := awsService.DeleteImages(uploadedImageURLs); deleteErr != nil {
			log.Printf("Erreur lors de la suppression des images: %v", deleteErr)
		}

		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Annonce créée avec succès. ID: %d", newAdID)

	// Réponse de succès
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      newAdID,
		"message": "Annonce créée avec succès",
		"images":  uploadedImageURLs,
	})
}

// GetValidatedAdsHandler récupère toutes les annonces validées.
func GetValidatedAdsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Analyser les paramètres de requête pour la pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Valeurs par défaut
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Limite par défaut de 10 annonces par page
	}

	// 2. Calculer le décalage (offset)
	offset := (page - 1) * limit

	// 3. Modifier la requête SQL pour inclure LIMIT, OFFSET et a.is_delivery_available
	rows, err := config.DB.Query(`
        SELECT 
            a.id, a.title, a.description, a.price, a.images, a.form_data, 
            a.city, a.phone_number, a.is_phone_visible, a.latitude, a.longitude,
            u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
            a.is_delivery_available  -- NOUVEAU: Ajoutez cette colonne
        FROM ads a
        JOIN users u ON a.user_id = u.id
        WHERE a.is_validated = true
        ORDER BY a.created_at DESC
        LIMIT $1 OFFSET $2`,
		limit, offset)
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
		var shopName, avatarURL sql.NullString
		var isDeliveryAvailable bool // NOUVEAU: Déclarez cette variable

		err := rows.Scan(
			&ad.ID,
			&ad.Title,
			&ad.Description,
			&ad.Price,
			&images,
			&formDataStr,
			&ad.City,
			&ad.PhoneNumber,
			&ad.IsPhoneVisible,
			&ad.Latitude,
			&ad.Longitude,
			&firstName,
			&lastName,
			&shopName,
			&accountType,
			&avatarURL,
			&isDeliveryAvailable, // NOUVEAU: Scannez la valeur
		)
		if err != nil {
			http.Error(w, "Erreur interne du serveur lors du scan", http.StatusInternalServerError)
			return
		}

		ad.Images = []string(images)

		if formDataStr.Valid {
			err = json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
			if err != nil {
				ad.FormData = nil
			}
		}

		ad.IsDeliveryAvailable = isDeliveryAvailable // NOUVEAU: Assignez la valeur

		// Logique pour déterminer le nom d'affichage
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
		ad.User.AvatarURL = avatarURL // Assurez-vous d'avoir ce champ dans votre modèle

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ads); err != nil {
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
}

// GetAdDetailsHandler gère la récupération des détails d'une annonce par son ID.
// GetAdDetailsHandler gère la récupération des détails d'une annonce par son ID.
func GetAdDetailsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de détails de l'annonce.")
	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	var ad models.Ad
	var images pq.StringArray
	var formDataStr, shopName, avatarURL sql.NullString
	var latitude, longitude sql.NullFloat64
	var firstName, lastName, accountType string
	var subCategoryName, categoryName string
	var isDeliveryAvailable bool
	var userID int // LIGNE MODIFIÉE: Ajout de la variable pour l'ID de l'utilisateur

	query := `
        SELECT
            a.id, a.title, a.description, a.price, a.images,
            a.form_data, a.city, a.phone_number, a.is_phone_visible, a.is_delivery_available,
            a.latitude, a.longitude, a.created_at,
            u.id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
            sc.name as sub_category_name, c.name as category_name
        FROM ads a
        JOIN users u ON a.user_id = u.id
        JOIN sub_categories sc ON a.sub_category_id = sc.id
        JOIN categories c ON sc.category_id = c.id
        WHERE a.id = $1
    `
	row := config.DB.QueryRow(query, adID)

	err = row.Scan(
		&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images,
		&formDataStr, &ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &isDeliveryAvailable,
		&latitude, &longitude, &ad.CreatedAt,
		&userID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
		&subCategoryName, &categoryName,
	)

	log.Printf("Détails de l'annonce récupérés pour l'ID: %d", adID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la lecture de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	ad.Images = []string(images)
	ad.IsDeliveryAvailable = isDeliveryAvailable
	if formDataStr.Valid {
		_ = json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
	}

	if latitude.Valid {
		ad.Latitude = latitude
	}
	if longitude.Valid {
		ad.Longitude = longitude
	}

	ad.SubCategoryName = subCategoryName
	ad.CategoryName = categoryName

	// LIGNE MODIFIÉE: Assignation de l'ID de l'utilisateur à la structure Ad.User
	ad.User.ID = userID
	ad.User.FirstName = firstName
	ad.User.LastName = lastName
	ad.User.ShopName = shopName
	ad.User.IsProAccount = accountType == "Professionnel"
	ad.User.AvatarURL = avatarURL

	if accountType == "Professionnel" && shopName.Valid {
		ad.User.DisplayName = shopName.String
	} else {
		ad.User.DisplayName = fmt.Sprintf("%s %s", firstName, lastName)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ad)
}

// GetSimilarAdsHandler gère la récupération des annonces similaires.
func GetSimilarAdsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête pour les annonces similaires.")

	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	var subCategoryID int
	// Étape 1 : Récupérer le sub_category_id de l'annonce d'origine
	err = config.DB.QueryRow("SELECT sub_category_id FROM ads WHERE id = $1 AND is_validated = TRUE", adID).Scan(&subCategoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			// L'annonce n'existe pas ou n'est pas validée
			http.Error(w, "Annonce non trouvée ou non validée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la récupération du sub_category_id: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	var ads []models.Ad
	// Étape 2 : Récupérer les annonces similaires validées
	query := `
		SELECT
			a.id, a.title, a.description, a.price, a.images, a.city, a.created_at,
			u.first_name, u.last_name, u.shop_name, u.account_type,
			sc.name AS sub_category_name, c.name AS category_name
		FROM ads a
		JOIN users u ON a.user_id = u.id
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.sub_category_id = $1 AND a.id != $2 AND a.is_validated = TRUE
		LIMIT 5
	`
	rows, err := config.DB.Query(query, subCategoryID, adID)
	if err != nil {
		log.Printf("Erreur lors de la récupération des annonces similaires: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var shopName sql.NullString
		var firstName, lastName, accountType string
		var subCategoryName, categoryName string

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &ad.City, &ad.CreatedAt,
			&firstName, &lastName, &shopName, &accountType,
			&subCategoryName, &categoryName,
		)
		if err != nil {
			log.Printf("Erreur lors de la lecture d'une ligne d'annonce similaire: %v", err)
			continue
		}

		// Remplissage de la structure de l'annonce
		ad.Images = []string(images)
		ad.User.FirstName = firstName
		ad.User.LastName = lastName
		ad.User.ShopName = shopName
		ad.User.IsProAccount = accountType == "Professionnel"
		if accountType == "Professionnel" && shopName.Valid {
			ad.User.DisplayName = shopName.String
		} else {
			ad.User.DisplayName = fmt.Sprintf("%s %s", firstName, lastName)
		}
		ad.SubCategoryName = subCategoryName
		ad.CategoryName = categoryName

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ads)
}

// GetSellerProfileHandler récupère les informations du vendeur et ses articles validés
func GetSellerProfileHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête pour le profil vendeur.")

	vars := mux.Vars(r)
	userIDStr := vars["userID"]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// 1. Récupérer les informations du vendeur
	var seller models.User
	sellerQuery := `
		SELECT id, first_name, last_name, shop_name, avatar_url, account_type, created_at
		FROM users 
		WHERE id = $1
	`

	var createdAt time.Time
	err = config.DB.QueryRow(sellerQuery, userID).Scan(
		&seller.ID,
		&seller.FirstName,
		&seller.LastName,
		&seller.ShopName,
		&seller.AvatarURL,
		&seller.AccountType,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la récupération des informations du vendeur: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// 2. Analyser les paramètres de requête pour la pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Valeurs par défaut
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Limite par défaut de 10 annonces par page
	}

	// 3. Calculer le décalage (offset)
	offset := (page - 1) * limit

	// 4. Récupérer les articles validés du vendeur
	adsQuery := `
		SELECT 
			a.id, a.title, a.description, a.price, a.images, a.form_data, 
			a.city, a.phone_number, a.is_phone_visible, a.latitude, a.longitude,
			a.is_delivery_available, a.created_at,
			sc.name as sub_category_name, c.name as category_name
		FROM ads a
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.user_id = $1 AND a.is_validated = true
		ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := config.DB.Query(adsQuery, userID, limit, offset)
	if err != nil {
		log.Printf("Erreur lors de la récupération des annonces du vendeur: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []models.Ad

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var formDataStr sql.NullString
		var isDeliveryAvailable bool

		err := rows.Scan(
			&ad.ID,
			&ad.Title,
			&ad.Description,
			&ad.Price,
			&images,
			&formDataStr,
			&ad.City,
			&ad.PhoneNumber,
			&ad.IsPhoneVisible,
			&ad.Latitude,
			&ad.Longitude,
			&isDeliveryAvailable,
			&ad.CreatedAt,
			&ad.SubCategoryName,
			&ad.CategoryName,
		)
		if err != nil {
			log.Printf("Erreur lors du scan des annonces: %v", err)
			continue
		}

		ad.Images = []string(images)
		ad.IsDeliveryAvailable = isDeliveryAvailable

		if formDataStr.Valid {
			err = json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
			if err != nil {
				ad.FormData = nil
			}
		}

		// Remplir les informations du vendeur pour chaque annonce
		ad.User.FirstName = seller.FirstName
		ad.User.LastName = seller.LastName
		ad.User.ShopName = seller.ShopName
		ad.User.AvatarURL = seller.AvatarURL
		ad.User.IsProAccount = seller.AccountType == "Professionnel"

		if seller.AccountType == "Professionnel" && seller.ShopName.Valid {
			ad.User.DisplayName = seller.ShopName.String
		} else {
			ad.User.DisplayName = fmt.Sprintf("%s %s", seller.FirstName, seller.LastName)
		}

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 5. Compter le nombre total d'annonces pour la pagination
	var totalAds int
	countQuery := `SELECT COUNT(*) FROM ads WHERE user_id = $1 AND is_validated = true`
	err = config.DB.QueryRow(countQuery, userID).Scan(&totalAds)
	if err != nil {
		log.Printf("Erreur lors du comptage des annonces: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 6. Préparer la réponse
	response := struct {
		Seller struct {
			ID          int            `json:"id"`
			FirstName   string         `json:"first_name"`
			LastName    string         `json:"last_name"`
			ShopName    sql.NullString `json:"shop_name"`
			AvatarURL   sql.NullString `json:"avatar_url"`
			AccountType string         `json:"account_type"`
			DisplayName string         `json:"display_name"`
			CreatedAt   time.Time      `json:"created_at"`
		} `json:"seller"`
		Ads        []models.Ad `json:"ads"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
	}{
		Ads: ads,
	}

	// Remplir les informations du vendeur
	response.Seller.ID = seller.ID
	response.Seller.FirstName = seller.FirstName
	response.Seller.LastName = seller.LastName
	response.Seller.ShopName = seller.ShopName
	response.Seller.AvatarURL = seller.AvatarURL
	response.Seller.AccountType = seller.AccountType
	response.Seller.CreatedAt = createdAt

	if seller.AccountType == "Professionnel" && seller.ShopName.Valid {
		response.Seller.DisplayName = seller.ShopName.String
	} else {
		response.Seller.DisplayName = fmt.Sprintf("%s %s", seller.FirstName, seller.LastName)
	}

	// Calculer les informations de pagination
	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit // Calcul du nombre de pages

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	log.Printf("Profil vendeur récupéré avec succès pour l'utilisateur ID: %d", userID)
}

// GetAllAdsHandler retrieves a paginated list of all ads with filtering options
func GetAllAdsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting the GetAllAdsHandler request processing.")

	// 1. Parse query parameters for pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Parse filter parameters
	city := r.URL.Query().Get("city")
	minPriceStr := r.URL.Query().Get("min_price")
	maxPriceStr := r.URL.Query().Get("max_price")
	sortBy := r.URL.Query().Get("sort_by") // "newest", "oldest", "price_asc", "price_desc"

	// Set default values
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Default limit of 10 ads per page
	}

	// Parse price filters
	var minPrice, maxPrice *float64
	if minPriceStr != "" {
		if mp, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
			minPrice = &mp
		}
	}
	if maxPriceStr != "" {
		if mp, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
			maxPrice = &mp
		}
	}

	// Set default sort
	if sortBy == "" {
		sortBy = "newest"
	}

	// 2. Calculate the offset
	offset := (page - 1) * limit

	// 3. Build dynamic SQL query with filters
	baseQuery := `
        SELECT
            a.id, a.title, a.description, a.price, a.images, a.form_data,
            a.city, a.phone_number, a.is_phone_visible, a.is_delivery_available,
            a.latitude, a.longitude, a.created_at, a.is_validated, a.is_deactivated, a.is_rejected,
            u.id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
            sc.name AS sub_category_name, c.name AS category_name
        FROM ads a
        JOIN users u ON a.user_id = u.id
        JOIN sub_categories sc ON a.sub_category_id = sc.id
        JOIN categories c ON sc.category_id = c.id
        WHERE a.is_validated = TRUE AND a.is_deactivated = FALSE AND a.is_rejected = FALSE`

	baseCountQuery := `SELECT COUNT(*) FROM ads a WHERE a.is_validated = TRUE AND a.is_deactivated = FALSE AND a.is_rejected = FALSE`

	var whereClauses []string
	var args []interface{}
	var countArgs []interface{}
	argIndex := 1

	// Add city filter
	if city != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(`REPLACE(REPLACE(LOWER(a.city), '-', ''), ' ', '') = REPLACE(REPLACE(LOWER($%d), '-', ''), ' ', '')`, argIndex))
		args = append(args, city)
		countArgs = append(countArgs, city)
		argIndex++
	}

	// Add price filters
	if minPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.price >= $%d", argIndex))
		args = append(args, *minPrice)
		countArgs = append(countArgs, *minPrice)
		argIndex++
	}
	if maxPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.price <= $%d", argIndex))
		args = append(args, *maxPrice)
		countArgs = append(countArgs, *maxPrice)
		argIndex++
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " AND " + strings.Join(whereClauses, " AND ")
	}

	// Add ORDER BY clause
	var orderBy string
	switch sortBy {
	case "oldest":
		orderBy = "ORDER BY a.created_at ASC"
	case "price_asc":
		orderBy = "ORDER BY a.price ASC, a.created_at DESC"
	case "price_desc":
		orderBy = "ORDER BY a.price DESC, a.created_at DESC"
	case "newest":
		fallthrough
	default:
		orderBy = "ORDER BY a.created_at DESC"
	}

	// Complete queries
	query := baseQuery + whereClause + " " + orderBy + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	countQuery := baseCountQuery + whereClause

	// Add LIMIT and OFFSET to args
	args = append(args, limit, offset)

	log.Printf("Executing query: %s", query)
	log.Printf("With args: %v", args)

	// 4. Execute the main query
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		log.Printf("Error retrieving all ads: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []models.Ad

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var formDataStr, shopName, avatarURL sql.NullString
		var latitude, longitude sql.NullFloat64
		var firstName, lastName, accountType string
		var subCategoryName, categoryName string

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
			&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &ad.IsDeliveryAvailable,
			&latitude, &longitude, &ad.CreatedAt, &ad.IsValidated, &ad.IsDeactivated, &ad.IsRejected,
			&ad.User.ID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
			&subCategoryName, &categoryName,
		)
		if err != nil {
			log.Printf("Error scanning ad row: %v", err)
			continue
		}

		// Assign scanned values to the Ad and User structs
		ad.Images = []string(images)
		if formDataStr.Valid {
			json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
		}
		if latitude.Valid {
			ad.Latitude = latitude
		}
		if longitude.Valid {
			ad.Longitude = longitude
		}

		ad.SubCategoryName = subCategoryName
		ad.CategoryName = categoryName
		ad.User.FirstName = firstName
		ad.User.LastName = lastName
		ad.User.ShopName = shopName
		ad.User.AvatarURL = avatarURL
		ad.User.IsProAccount = accountType == "Professionnel"

		// Set the display name
		if ad.User.IsProAccount && shopName.Valid {
			ad.User.DisplayName = shopName.String
		} else {
			ad.User.DisplayName = fmt.Sprintf("%s %s", firstName, lastName)
		}

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error after iterating through rows: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 5. Count the total number of ads for pagination metadata
	var totalAds int
	err = config.DB.QueryRow(countQuery, countArgs...).Scan(&totalAds)
	if err != nil {
		log.Printf("Error counting total ads: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 6. Prepare the final response with ads and pagination info
	response := struct {
		Ads        []models.Ad `json:"ads"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
		Filters struct {
			City     string   `json:"city,omitempty"`
			MinPrice *float64 `json:"min_price,omitempty"`
			MaxPrice *float64 `json:"max_price,omitempty"`
			SortBy   string   `json:"sort_by"`
		} `json:"filters"`
	}{
		Ads: ads,
	}

	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit // Calculate total pages

	// Add filter info to response
	response.Filters.City = city
	response.Filters.MinPrice = minPrice
	response.Filters.MaxPrice = maxPrice
	response.Filters.SortBy = sortBy

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	log.Println("GetAllAdsHandler request successfully processed.")
}

// GetAvailableCitiesHandler retrieves the list of cities that have ads
func GetAvailableCitiesHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting the GetAvailableCitiesHandler request processing.")

	// SQL query to get distinct, normalized cities from ads table
	query := `
        SELECT DISTINCT REPLACE(REPLACE(city, '-', ''), ' ', '')
        FROM ads 
        WHERE city IS NOT NULL 
        AND TRIM(city) != '' 
        ORDER BY REPLACE(REPLACE(city, '-', ''), ' ', '') ASC
    `

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("Error retrieving available cities: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var cities []string

	for rows.Next() {
		var city string
		err := rows.Scan(&city)
		if err != nil {
			log.Printf("Error scanning city row: %v", err)
			continue
		}
		cities = append(cities, city)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error after iterating through city rows: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If no cities found, return empty array
	if cities == nil {
		cities = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cities); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	log.Printf("GetAvailableCitiesHandler request successfully processed. Found %d cities.", len(cities))
}

// GetAdsByCategoryHandler récupère les annonces filtrées par catégorie ou sous-catégorie
func GetAdsByCategoryHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de GetAdsByCategoryHandler.")

	// 1. Récupérer les paramètres de l'URL
	vars := mux.Vars(r)
	categoryIDStr := vars["categoryID"]

	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		log.Printf("ID de catégorie invalide: %s", categoryIDStr)
		http.Error(w, "ID de catégorie invalide", http.StatusBadRequest)
		return
	}

	// 2. Analyser les paramètres de requête pour la pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	subCategoryIDStr := r.URL.Query().Get("sub_category_id")

	// Parse des filtres supplémentaires (comme dans GetAllAdsHandler)
	city := r.URL.Query().Get("city")
	minPriceStr := r.URL.Query().Get("min_price")
	maxPriceStr := r.URL.Query().Get("max_price")
	sortBy := r.URL.Query().Get("sort_by")

	// Valeurs par défaut
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}

	// Parse sub_category_id si fourni
	var subCategoryID *int
	if subCategoryIDStr != "" {
		if subID, err := strconv.Atoi(subCategoryIDStr); err == nil {
			subCategoryID = &subID
		}
	}

	// Parse des filtres de prix
	var minPrice, maxPrice *float64
	if minPriceStr != "" {
		if mp, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
			minPrice = &mp
		}
	}
	if maxPriceStr != "" {
		if mp, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
			maxPrice = &mp
		}
	}

	// Sort par défaut
	if sortBy == "" {
		sortBy = "newest"
	}

	// 3. Calculer le décalage (offset)
	offset := (page - 1) * limit

	// 4. Construire la requête SQL dynamique
	baseQuery := `
        SELECT
            a.id, a.title, a.description, a.price, a.images, a.form_data,
            a.city, a.phone_number, a.is_phone_visible, a.is_delivery_available,
            a.latitude, a.longitude, a.created_at,
            u.id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
            sc.name AS sub_category_name, c.name AS category_name
        FROM ads a
        JOIN users u ON a.user_id = u.id
        JOIN sub_categories sc ON a.sub_category_id = sc.id
        JOIN categories c ON sc.category_id = c.id
        WHERE a.is_validated = TRUE AND a.is_deactivated = FALSE AND a.is_rejected = FALSE`

	baseCountQuery := `
        SELECT COUNT(*) 
        FROM ads a 
        JOIN sub_categories sc ON a.sub_category_id = sc.id
        WHERE a.is_validated = TRUE AND a.is_deactivated = FALSE AND a.is_rejected = FALSE`

	var whereClauses []string
	var args []interface{}
	var countArgs []interface{}
	argIndex := 1

	// Ajouter le filtre de catégorie obligatoire
	if subCategoryID != nil {
		// Filtrer par sous-catégorie spécifique
		whereClauses = append(whereClauses, fmt.Sprintf("a.sub_category_id = $%d", argIndex))
		args = append(args, *subCategoryID)
		countArgs = append(countArgs, *subCategoryID)
	} else {
		// Filtrer par catégorie (toutes les sous-catégories de cette catégorie)
		whereClauses = append(whereClauses, fmt.Sprintf("sc.category_id = $%d", argIndex))
		args = append(args, categoryID)
		countArgs = append(countArgs, categoryID)
	}
	argIndex++

	// Ajouter les filtres optionnels (ville, prix)
	if city != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(`REPLACE(REPLACE(LOWER(a.city), '-', ''), ' ', '') = REPLACE(REPLACE(LOWER($%d), '-', ''), ' ', '')`, argIndex))
		args = append(args, city)
		countArgs = append(countArgs, city)
		argIndex++
	}

	if minPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.price >= $%d", argIndex))
		args = append(args, *minPrice)
		countArgs = append(countArgs, *minPrice)
		argIndex++
	}

	if maxPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.price <= $%d", argIndex))
		args = append(args, *maxPrice)
		countArgs = append(countArgs, *maxPrice)
		argIndex++
	}

	// Construire la clause WHERE
	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " AND " + strings.Join(whereClauses, " AND ")
	}

	// Ajouter la clause ORDER BY
	var orderBy string
	switch sortBy {
	case "oldest":
		orderBy = "ORDER BY a.created_at ASC"
	case "price_asc":
		orderBy = "ORDER BY a.price ASC, a.created_at DESC"
	case "price_desc":
		orderBy = "ORDER BY a.price DESC, a.created_at DESC"
	case "newest":
		fallthrough
	default:
		orderBy = "ORDER BY a.created_at DESC"
	}

	// Requêtes complètes
	query := baseQuery + whereClause + " " + orderBy + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	countQuery := baseCountQuery + whereClause

	// Ajouter LIMIT et OFFSET aux arguments
	args = append(args, limit, offset)

	log.Printf("Exécution de la requête: %s", query)
	log.Printf("Avec les arguments: %v", args)

	// 5. Exécuter la requête principale
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		log.Printf("Erreur lors de la récupération des annonces par catégorie: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []models.Ad

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var formDataStr, shopName, avatarURL sql.NullString
		var latitude, longitude sql.NullFloat64
		var firstName, lastName, accountType string
		var subCategoryName, categoryName string

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
			&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &ad.IsDeliveryAvailable,
			&latitude, &longitude, &ad.CreatedAt,
			&ad.User.ID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
			&subCategoryName, &categoryName,
		)
		if err != nil {
			log.Printf("Erreur lors du scan de l'annonce: %v", err)
			continue
		}

		// Assigner les valeurs scannées
		ad.Images = []string(images)
		if formDataStr.Valid {
			json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
		}
		if latitude.Valid {
			ad.Latitude = latitude
		}
		if longitude.Valid {
			ad.Longitude = longitude
		}

		ad.SubCategoryName = subCategoryName
		ad.CategoryName = categoryName
		ad.User.FirstName = firstName
		ad.User.LastName = lastName
		ad.User.ShopName = shopName
		ad.User.AvatarURL = avatarURL
		ad.User.IsProAccount = accountType == "Professionnel"

		// Définir le nom d'affichage
		if ad.User.IsProAccount && shopName.Valid {
			ad.User.DisplayName = shopName.String
		} else {
			ad.User.DisplayName = fmt.Sprintf("%s %s", firstName, lastName)
		}

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 6. Compter le nombre total d'annonces pour la pagination
	var totalAds int
	err = config.DB.QueryRow(countQuery, countArgs...).Scan(&totalAds)
	if err != nil {
		log.Printf("Erreur lors du comptage des annonces: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 7. Récupérer les informations de la catégorie pour la réponse
	var categoryInfo struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Icon string `json:"icon"`
	}

	categoryQuery := `SELECT id, name, icon FROM categories WHERE id = $1`
	err = config.DB.QueryRow(categoryQuery, categoryID).Scan(&categoryInfo.ID, &categoryInfo.Name, &categoryInfo.Icon)
	if err != nil {
		log.Printf("Erreur lors de la récupération des informations de catégorie: %v", err)
		// Ne pas faire échouer la requête pour cette erreur
		categoryInfo = struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Icon string `json:"icon"`
		}{ID: categoryID, Name: "Catégorie inconnue", Icon: ""}
	}

	// 8. Préparer la réponse finale
	response := struct {
		Ads        []models.Ad `json:"ads"`
		Category   interface{} `json:"category"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
		Filters struct {
			CategoryID    int      `json:"category_id"`
			SubCategoryID *int     `json:"sub_category_id,omitempty"`
			City          string   `json:"city,omitempty"`
			MinPrice      *float64 `json:"min_price,omitempty"`
			MaxPrice      *float64 `json:"max_price,omitempty"`
			SortBy        string   `json:"sort_by"`
		} `json:"filters"`
	}{
		Ads:      ads,
		Category: categoryInfo,
	}

	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit

	// Ajouter les informations de filtre à la réponse
	response.Filters.CategoryID = categoryID
	response.Filters.SubCategoryID = subCategoryID
	response.Filters.City = city
	response.Filters.MinPrice = minPrice
	response.Filters.MaxPrice = maxPrice
	response.Filters.SortBy = sortBy

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
	}

	log.Printf("GetAdsByCategoryHandler traité avec succès. Trouvé %d annonces pour la catégorie %d.", len(ads), categoryID)
}

// GetUserAdsHandler récupère toutes les annonces de l'utilisateur connecté
func GetUserAdsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des annonces pour l'utilisateur connecté.")

	// Récupérer l'ID utilisateur du contexte de la requête
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte. Assurez-vous que le middleware JWT est en place.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}
	log.Printf("ID utilisateur trouvé: %d", userID)

	// Analyser les paramètres de requête pour la pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // Limite par défaut
	}

	offset := (page - 1) * limit

	var ads []models.Ad

	// ✅ Requête SQL CORRIGÉE avec DISTINCT ON pour obtenir le boost le plus récent PAR annonce
	query := `
		SELECT 
			a.id, a.title, a.description, a.price, a.images, a.form_data, 
			a.city, a.phone_number, a.is_phone_visible, a.latitude, a.longitude,
			a.is_validated, a.is_deactivated, a.is_rejected, a.is_delivery_available, 
			a.is_sold, a.created_at, a.views_count,
			sc.name as sub_category_name, c.name as category_name,
			COALESCE(f.favorites_count, 0) as favorites_count,
			a.is_boosted,
			ab.end_date as boost_expires_at
		FROM ads a
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		LEFT JOIN (
			SELECT ad_id, COUNT(*) as favorites_count 
			FROM favorites 
			GROUP BY ad_id
		) f ON a.id = f.ad_id
		LEFT JOIN LATERAL (
			SELECT end_date
			FROM ad_boosts
			WHERE ad_id = a.id 
			AND is_active = true 
			AND payment_status = 'completed'
			AND end_date > CURRENT_TIMESTAMP
			ORDER BY end_date DESC
			LIMIT 1
		) ab ON true
		WHERE a.user_id = $1
		ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := config.DB.Query(query, userID, limit, offset)
	if err != nil {
		log.Printf("Erreur lors de la récupération des annonces de l'utilisateur: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var formDataStr sql.NullString
		var latitude, longitude sql.NullFloat64
		var viewsCount int
		var favoritesCount int
		var isBoosted bool
		var boostExpiresAt sql.NullTime

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
			&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &latitude, &longitude,
			&ad.IsValidated, &ad.IsDeactivated, &ad.IsRejected, &ad.IsDeliveryAvailable,
			&ad.IsSold, &ad.CreatedAt,
			&viewsCount,
			&ad.SubCategoryName, &ad.CategoryName,
			&favoritesCount,
			&isBoosted,
			&boostExpiresAt,
		)

		if err != nil {
			log.Printf("Erreur lors du scan d'une annonce utilisateur: %v", err)
			continue
		}

		ad.Images = []string(images)
		if formDataStr.Valid {
			_ = json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
		}
		if latitude.Valid {
			ad.Latitude = latitude
		}
		if longitude.Valid {
			ad.Longitude = longitude
		}

		ad.ViewsCount = viewsCount
		ad.FavoritesCount = favoritesCount
		ad.IsBoosted = isBoosted

		if boostExpiresAt.Valid {
			ad.BoostExpiresAt = &boostExpiresAt.Time
		}

		// ✅ Log pour vérifier les données de boost par annonce
		log.Printf("📦 Annonce ID=%d, IsBoosted=%v, BoostExpiresAt=%v", ad.ID, ad.IsBoosted, ad.BoostExpiresAt)

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des annonces de l'utilisateur: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Compter le nombre total d'annonces de l'utilisateur pour la pagination
	var totalAds int
	countQuery := `SELECT COUNT(*) FROM ads WHERE user_id = $1`
	err = config.DB.QueryRow(countQuery, userID).Scan(&totalAds)
	if err != nil {
		log.Printf("Erreur lors du comptage des annonces de l'utilisateur: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	response := struct {
		Ads        []models.Ad `json:"ads"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
	}{
		Ads: ads,
	}

	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
	}

	log.Printf("✅ Récupération réussie de %d annonces pour l'utilisateur %d", len(ads), userID)
}

// MarkAdAsSoldHandler gère le marquage d'une annonce comme vendue
func MarkAdAsSoldHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête pour marquer une annonce comme vendue.")

	// Récupérer l'ID utilisateur du contexte de la requête
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'annonce depuis l'URL
	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// Structure pour recevoir les données de vente (optionnelles)
	var saleData struct {
		SalePrice    *float64 `json:"sale_price"`
		BuyerContact *string  `json:"buyer_contact"`
		Notes        *string  `json:"notes"`
	}

	// Décoder le body JSON (optionnel)
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&saleData); err != nil {
			log.Printf("Erreur lors du décodage des données de vente: %v", err)
			// Continue sans les données optionnelles
		}
	}

	// Vérifier que l'utilisateur est propriétaire de l'annonce
	var adOwnerID int
	var isAlreadySold bool
	checkQuery := `SELECT user_id, is_sold FROM ads WHERE id = $1`
	err = config.DB.QueryRow(checkQuery, adID).Scan(&adOwnerID, &isAlreadySold)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la vérification de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// Vérifier que l'utilisateur est le propriétaire de l'annonce
	if adOwnerID != userID {
		http.Error(w, "Vous n'êtes pas autorisé à modifier cette annonce", http.StatusForbidden)
		return
	}

	// Vérifier que l'annonce n'est pas déjà vendue
	if isAlreadySold {
		http.Error(w, "Cette annonce est déjà marquée comme vendue", http.StatusConflict)
		return
	}

	// Commencer une transaction
	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur lors du début de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Marquer l'annonce comme vendue dans la table ads
	updateQuery := `UPDATE ads SET is_sold = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err = tx.Exec(updateQuery, adID)
	if err != nil {
		log.Printf("Erreur lors de la mise à jour de l'annonce: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Insérer les détails de la vente dans la table sold_ads
	insertQuery := `
		INSERT INTO sold_ads (ad_id, user_id, sale_price, buyer_contact, notes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, sold_at
	`
	var soldAdID int
	var soldAt time.Time
	err = tx.QueryRow(insertQuery, adID, userID, saleData.SalePrice, saleData.BuyerContact, saleData.Notes).Scan(&soldAdID, &soldAt)
	if err != nil {
		log.Printf("Erreur lors de l'insertion des détails de vente: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Valider la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur lors de la validation de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Préparer la réponse
	response := struct {
		Message string    `json:"message"`
		AdID    int       `json:"ad_id"`
		SoldAt  time.Time `json:"sold_at"`
	}{
		Message: "Annonce marquée comme vendue avec succès",
		AdID:    adID,
		SoldAt:  soldAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
	}

	log.Printf("Annonce %d marquée comme vendue avec succès par l'utilisateur %d", adID, userID)
}

// UnmarkAdAsSoldHandler gère le démarquage d'une annonce comme vendue (pour réactiver)
func UnmarkAdAsSoldHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête pour démarquer une annonce comme vendue.")

	// Récupérer l'ID utilisateur du contexte de la requête
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'annonce depuis l'URL
	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// Vérifier que l'utilisateur est propriétaire de l'annonce
	var adOwnerID int
	var isSold bool
	checkQuery := `SELECT user_id, is_sold FROM ads WHERE id = $1`
	err = config.DB.QueryRow(checkQuery, adID).Scan(&adOwnerID, &isSold)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la vérification de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// Vérifier que l'utilisateur est le propriétaire de l'annonce
	if adOwnerID != userID {
		http.Error(w, "Vous n'êtes pas autorisé à modifier cette annonce", http.StatusForbidden)
		return
	}

	// Vérifier que l'annonce est bien marquée comme vendue
	if !isSold {
		http.Error(w, "Cette annonce n'est pas marquée comme vendue", http.StatusConflict)
		return
	}

	// Commencer une transaction
	tx, err := config.DB.Begin()
	if err != nil {
		log.Printf("Erreur lors du début de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Supprimer l'entrée de sold_ads
	deleteQuery := `DELETE FROM sold_ads WHERE ad_id = $1`
	_, err = tx.Exec(deleteQuery, adID)
	if err != nil {
		log.Printf("Erreur lors de la suppression des détails de vente: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Remettre l'annonce comme non vendue
	updateQuery := `UPDATE ads SET is_sold = FALSE, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err = tx.Exec(updateQuery, adID)
	if err != nil {
		log.Printf("Erreur lors de la mise à jour de l'annonce: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Valider la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur lors de la validation de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Réponse de succès
	response := struct {
		Message string `json:"message"`
		AdID    int    `json:"ad_id"`
	}{
		Message: "Annonce remise en vente avec succès",
		AdID:    adID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
	}

	log.Printf("Annonce %d remise en vente avec succès par l'utilisateur %d", adID, userID)
}

// GetSoldAdsHandler récupère toutes les annonces vendues de l'utilisateur
func GetSoldAdsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des annonces vendues pour l'utilisateur connecté.")

	// Récupérer l'ID utilisateur du contexte de la requête
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur non trouvé dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Analyser les paramètres de requête pour la pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Requête pour récupérer les annonces vendues
	query := `
		SELECT 
			a.id, a.title, a.description, a.price, a.images, a.city, a.created_at,
			sa.sold_at, sa.sale_price, sa.buyer_contact, sa.notes,
			sc.name as sub_category_name, c.name as category_name
		FROM ads a
		JOIN sold_ads sa ON a.id = sa.ad_id
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.user_id = $1 AND a.is_sold = TRUE
		ORDER BY sa.sold_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := config.DB.Query(query, userID, limit, offset)
	if err != nil {
		log.Printf("Erreur lors de la récupération des annonces vendues: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var soldAds []struct {
		models.Ad
		SoldAt       time.Time `json:"sold_at"`
		SalePrice    *float64  `json:"sale_price"`
		BuyerContact *string   `json:"buyer_contact"`
		Notes        *string   `json:"notes"`
	}

	for rows.Next() {
		var soldAd struct {
			models.Ad
			SoldAt       time.Time `json:"sold_at"`
			SalePrice    *float64  `json:"sale_price"`
			BuyerContact *string   `json:"buyer_contact"`
			Notes        *string   `json:"notes"`
		}
		var images pq.StringArray

		err := rows.Scan(
			&soldAd.ID, &soldAd.Title, &soldAd.Description, &soldAd.Price, &images, &soldAd.City, &soldAd.CreatedAt,
			&soldAd.SoldAt, &soldAd.SalePrice, &soldAd.BuyerContact, &soldAd.Notes,
			&soldAd.SubCategoryName, &soldAd.CategoryName,
		)
		if err != nil {
			log.Printf("Erreur lors du scan d'une annonce vendue: %v", err)
			continue
		}

		soldAd.Images = []string(images)
		soldAds = append(soldAds, soldAd)
	}

	// Compter le nombre total d'annonces vendues
	var totalSoldAds int
	countQuery := `SELECT COUNT(*) FROM sold_ads WHERE user_id = $1`
	err = config.DB.QueryRow(countQuery, userID).Scan(&totalSoldAds)
	if err != nil {
		log.Printf("Erreur lors du comptage des annonces vendues: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	response := struct {
		SoldAds    interface{} `json:"sold_ads"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
	}{
		SoldAds: soldAds,
	}

	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalSoldAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalSoldAds + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
	}

	log.Printf("Récupération des annonces vendues réussie pour l'utilisateur %d", userID)
}

// DeleteAdHandler gère la suppression d'une annonce par son ID.
func DeleteAdHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer l'ID de l'utilisateur depuis le contexte
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'annonce depuis les variables de la requête
	vars := mux.Vars(r)
	adIDStr := vars["adID"]
	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// Vérifier si l'utilisateur est bien le propriétaire de l'annonce
	var ownerID int
	err = config.DB.QueryRow("SELECT user_id FROM ads WHERE id = $1", adID).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la vérification du propriétaire de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	if ownerID != userID {
		http.Error(w, "Vous n'êtes pas autorisé à supprimer cette annonce", http.StatusForbidden)
		return
	}

	// Supprimer l'annonce de la base de données
	_, err = config.DB.Exec("DELETE FROM ads WHERE id = $1", adID)
	if err != nil {
		log.Printf("Erreur lors de la suppression de l'annonce: %v", err)
		http.Error(w, "Échec de la suppression de l'annonce", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content pour une suppression réussie
}

// IncrementAdViewsHandler incrémente le nombre de vues pour une annonce spécifique.
func IncrementAdViewsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer les variables de l'URL
	vars := mux.Vars(r)
	adIDStr := vars["adID"]

	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// Préparer la requête SQL pour incrémenter le compteur de vues
	query := `
        UPDATE ads 
        SET views_count = views_count + 1 
        WHERE id = $1
    `
	_, err = config.DB.Exec(query, adID)
	if err != nil {
		log.Printf("Erreur lors de l'incrémentation des vues pour l'annonce %d: %v", adID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Vues incrémentées avec succès"})
}

// Structure pour la requête de mise à jour de l'annonce
type AdUpdateRequest struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	PhoneNumber    string   `json:"phoneNumber"`
	IsPhoneVisible bool     `json:"isPhoneVisible"`
	City           string   `json:"city"`
	Images         []string `json:"images"`    // URLs des images existantes
	NewImages      []string `json:"newImages"` // Images base64 à uploader
	Price          float64  `json:"price"`
	RemovedImages  []string `json:"removedImages"` // URLs des images à supprimer
}

// EditAdHandler gère la modification d'une annonce par son ID avec gestion complète des images.
func EditAdHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Récupérer l'ID de l'utilisateur depuis le contexte
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("Erreur: ID utilisateur manquant dans le contexte.")
		http.Error(w, "ID utilisateur manquant", http.StatusUnauthorized)
		return
	}

	log.Printf("User ID from context: %d", userID)

	// Récupérer l'ID de l'annonce depuis les variables de la requête
	vars := mux.Vars(r)
	adIDStr, exists := vars["adID"]
	if !exists {
		log.Println("Erreur: ID d'annonce manquant dans l'URL")
		http.Error(w, "ID d'annonce manquant", http.StatusBadRequest)
		return
	}

	log.Printf("Raw adID received from URL: %s", adIDStr)

	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		log.Printf("Erreur de conversion de l'ID d'annonce: %v", err)
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Parsed adID: %d", adID)

	// Décoder le corps de la requête
	var req AdUpdateRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Erreur de décodage JSON: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	log.Printf("Request data: %+v", req)

	// Récupérer les images actuelles de l'annonce et vérifier le propriétaire
	var ownerID int
	var currentImagesArray pq.StringArray
	err = config.DB.QueryRow("SELECT user_id, images FROM ads WHERE id = $1", adID).Scan(&ownerID, &currentImagesArray)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Annonce non trouvée: %d", adID)
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la vérification de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	currentImages := []string(currentImagesArray)
	log.Printf("Images actuelles: %v", currentImages)
	log.Printf("Owner ID: %d, User ID: %d", ownerID, userID)

	if ownerID != userID {
		log.Printf("Utilisateur %d non autorisé à modifier l'annonce %d (propriétaire: %d)", userID, adID, ownerID)
		http.Error(w, "Vous n'êtes pas autorisé à modifier cette annonce", http.StatusForbidden)
		return
	}

	// Initialiser le service AWS
	awsService, err := services.NewAWSService()
	if err != nil {
		log.Printf("Erreur lors de l'initialisation du service AWS: %v", err)
		http.Error(w, "Erreur de configuration du service de stockage", http.StatusInternalServerError)
		return
	}

	// Traitement des nouvelles images à uploader (base64)
	var newUploadedImages []string
	if len(req.NewImages) > 0 {
		log.Printf("Upload de %d nouvelles images (base64)", len(req.NewImages))
		newUploadedImages, err = awsService.UploadAdImages(req.NewImages)
		if err != nil {
			log.Printf("Erreur lors de l'upload des nouvelles images: %v", err)
			http.Error(w, "Erreur lors de l'upload des nouvelles images", http.StatusInternalServerError)
			return
		}
		log.Printf("Nouvelles images uploadées: %v", newUploadedImages)
	}

	// Identifier les images à supprimer réellement de S3
	var imagesToDeleteFromS3 []string
	for _, removedImg := range req.RemovedImages {
		// Vérifier que l'image fait partie des images actuelles
		isCurrentImage := false
		for _, currentImg := range currentImages {
			if currentImg == removedImg {
				isCurrentImage = true
				break
			}
		}
		if isCurrentImage {
			imagesToDeleteFromS3 = append(imagesToDeleteFromS3, removedImg)
		}
	}

	// Supprimer les images de S3
	if len(imagesToDeleteFromS3) > 0 {
		log.Printf("Suppression de %d images de S3", len(imagesToDeleteFromS3))
		err = awsService.DeleteImages(imagesToDeleteFromS3)
		if err != nil {
			log.Printf("Erreur lors de la suppression des images S3: %v", err)
			// Ne pas faire échouer la requête pour cette erreur, juste logger
		} else {
			log.Printf("Images supprimées avec succès de S3")
		}
	}

	// Construire la liste finale des images
	finalImages := make([]string, 0)

	// Ajouter les images existantes (sauf celles à supprimer)
	for _, img := range req.Images {
		// Vérifier que l'image n'est pas dans la liste des images à supprimer
		shouldKeep := true
		for _, removedImg := range req.RemovedImages {
			if img == removedImg {
				shouldKeep = false
				break
			}
		}
		if shouldKeep {
			finalImages = append(finalImages, img)
		}
	}

	// Ajouter les nouvelles images uploadées
	finalImages = append(finalImages, newUploadedImages...)

	log.Printf("Images finales: %v", finalImages)

	// Vérifier qu'il reste au moins une image
	if len(finalImages) == 0 {
		log.Println("Erreur: Aucune image restante après mise à jour")
		http.Error(w, "Au moins une image est requise", http.StatusBadRequest)
		return
	}

	// Mise à jour de l'annonce dans la base de données
	updateQuery := `
        UPDATE ads 
        SET title = $1, 
            description = $2, 
            images = $3, 
            phone_number = $4, 
            is_phone_visible = $5, 
            city = $6, 
            price = $7, 
            updated_at = NOW(),
            is_validated = FALSE, 
            is_deactivated = FALSE, 
            is_rejected = FALSE
        WHERE id = $8
    `

	_, err = config.DB.Exec(updateQuery,
		req.Title,
		req.Description,
		pq.Array(finalImages),
		req.PhoneNumber,
		req.IsPhoneVisible,
		req.City,
		req.Price,
		adID,
	)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour de l'annonce: %v", err)

		// En cas d'erreur, essayer de supprimer les nouvelles images uploadées
		if len(newUploadedImages) > 0 {
			log.Println("Tentative de suppression des nouvelles images suite à l'échec de la mise à jour...")
			if deleteErr := awsService.DeleteImages(newUploadedImages); deleteErr != nil {
				log.Printf("Erreur lors de la suppression des nouvelles images: %v", deleteErr)
			}
		}

		http.Error(w, "Échec de la mise à jour de l'annonce", http.StatusInternalServerError)
		return
	}

	log.Printf("Annonce %d mise à jour avec succès par l'utilisateur %d", adID, userID)

	// Renvoyer une réponse de succès avec les nouvelles images
	response := struct {
		Message       string   `json:"message"`
		Images        []string `json:"images"`
		ImagesCount   int      `json:"images_count"`
		AddedImages   int      `json:"added_images"`
		RemovedImages int      `json:"removed_images"`
	}{
		Message:       "Annonce mise à jour avec succès",
		Images:        finalImages,
		ImagesCount:   len(finalImages),
		AddedImages:   len(newUploadedImages),
		RemovedImages: len(imagesToDeleteFromS3),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// SearchAdsHandler gère la recherche d'annonces avec filtres et pagination
func SearchAdsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de recherche d'annonces.")

	// 1. Récupérer et parser les paramètres de recherche
	searchQuery := r.URL.Query().Get("q") // Terme de recherche
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Filtres optionnels
	city := r.URL.Query().Get("city")
	categoryIDStr := r.URL.Query().Get("category_id")
	subCategoryIDStr := r.URL.Query().Get("sub_category_id")
	minPriceStr := r.URL.Query().Get("min_price")
	maxPriceStr := r.URL.Query().Get("max_price")
	sortBy := r.URL.Query().Get("sort_by") // "newest", "oldest", "price_asc", "price_desc", "relevance"
	isDeliveryAvailableStr := r.URL.Query().Get("is_delivery_available")

	// Valeurs par défaut pour la pagination
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20 // Limite par défaut pour la recherche
	}

	// Parser les filtres optionnels
	var categoryID, subCategoryID *int
	if categoryIDStr != "" {
		if cid, err := strconv.Atoi(categoryIDStr); err == nil {
			categoryID = &cid
		}
	}
	if subCategoryIDStr != "" {
		if scid, err := strconv.Atoi(subCategoryIDStr); err == nil {
			subCategoryID = &scid
		}
	}

	var minPrice, maxPrice *float64
	if minPriceStr != "" {
		if mp, err := strconv.ParseFloat(minPriceStr, 64); err == nil {
			minPrice = &mp
		}
	}
	if maxPriceStr != "" {
		if mp, err := strconv.ParseFloat(maxPriceStr, 64); err == nil {
			maxPrice = &mp
		}
	}

	var isDeliveryAvailable *bool
	if isDeliveryAvailableStr != "" {
		if ida, err := strconv.ParseBool(isDeliveryAvailableStr); err == nil {
			isDeliveryAvailable = &ida
		}
	}

	// Sort par défaut
	if sortBy == "" {
		if searchQuery != "" {
			sortBy = "relevance" // Par pertinence si recherche textuelle
		} else {
			sortBy = "newest" // Par date sinon
		}
	}

	// 2. Calculer l'offset
	offset := (page - 1) * limit

	// 3. Construire la requête SQL dynamique
	baseQuery := `
		SELECT
			a.id, a.title, a.description, a.price, a.images, a.form_data,
			a.city, a.phone_number, a.is_phone_visible, a.is_delivery_available,
			a.latitude, a.longitude, a.created_at,
			u.id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
			sc.name AS sub_category_name, c.name AS category_name
		FROM ads a
		JOIN users u ON a.user_id = u.id
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.is_validated = TRUE 
		AND a.is_deactivated = FALSE 
		AND a.is_rejected = FALSE
		AND a.is_sold = FALSE`

	baseCountQuery := `
		SELECT COUNT(*) 
		FROM ads a 
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		WHERE a.is_validated = TRUE 
		AND a.is_deactivated = FALSE 
		AND a.is_rejected = FALSE
		AND a.is_sold = FALSE`

	var whereClauses []string
	var args []interface{}
	var countArgs []interface{}
	argIndex := 1

	// Filtre de recherche textuelle (recherche dans titre et description)
	if searchQuery != "" {
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		whereClauses = append(whereClauses,
			fmt.Sprintf("(LOWER(a.title) LIKE $%d OR LOWER(a.description) LIKE $%d)", argIndex, argIndex))
		args = append(args, searchPattern)
		countArgs = append(countArgs, searchPattern)
		argIndex++
	}

	// Filtre par catégorie
	if subCategoryID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.sub_category_id = $%d", argIndex))
		args = append(args, *subCategoryID)
		countArgs = append(countArgs, *subCategoryID)
		argIndex++
	} else if categoryID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("sc.category_id = $%d", argIndex))
		args = append(args, *categoryID)
		countArgs = append(countArgs, *categoryID)
		argIndex++
	}

	// Filtre par ville (normalisation pour comparaison)
	if city != "" {
		whereClauses = append(whereClauses,
			fmt.Sprintf(`REPLACE(REPLACE(LOWER(a.city), '-', ''), ' ', '') = REPLACE(REPLACE(LOWER($%d), '-', ''), ' ', '')`, argIndex))
		args = append(args, city)
		countArgs = append(countArgs, city)
		argIndex++
	}

	// Filtres de prix
	if minPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.price >= $%d", argIndex))
		args = append(args, *minPrice)
		countArgs = append(countArgs, *minPrice)
		argIndex++
	}
	if maxPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.price <= $%d", argIndex))
		args = append(args, *maxPrice)
		countArgs = append(countArgs, *maxPrice)
		argIndex++
	}

	// Filtre de disponibilité de livraison
	if isDeliveryAvailable != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.is_delivery_available = $%d", argIndex))
		args = append(args, *isDeliveryAvailable)
		countArgs = append(countArgs, *isDeliveryAvailable)
		argIndex++
	}

	// Construire la clause WHERE
	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " AND " + strings.Join(whereClauses, " AND ")
	}

	// Ajouter la clause ORDER BY
	var orderBy string
	switch sortBy {
	case "oldest":
		orderBy = "ORDER BY a.created_at ASC"
	case "price_asc":
		orderBy = "ORDER BY a.price ASC, a.created_at DESC"
	case "price_desc":
		orderBy = "ORDER BY a.price DESC, a.created_at DESC"
	case "relevance":
		// Pour la pertinence, on peut améliorer avec un score de recherche
		// Ici, simple ordre par pertinence basique (correspondance dans le titre prioritaire)
		if searchQuery != "" {
			orderBy = `ORDER BY 
				CASE 
					WHEN LOWER(a.title) LIKE $` + strconv.Itoa(argIndex) + ` THEN 1
					WHEN LOWER(a.description) LIKE $` + strconv.Itoa(argIndex) + ` THEN 2
					ELSE 3
				END,
				a.created_at DESC`
			args = append(args, "%"+strings.ToLower(searchQuery)+"%")
			argIndex++
		} else {
			orderBy = "ORDER BY a.created_at DESC"
		}
	case "newest":
		fallthrough
	default:
		orderBy = "ORDER BY a.created_at DESC"
	}

	// Requêtes complètes
	query := baseQuery + whereClause + " " + orderBy + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	countQuery := baseCountQuery + whereClause

	// Ajouter LIMIT et OFFSET aux arguments
	args = append(args, limit, offset)

	log.Printf("Exécution de la requête de recherche: %s", query)
	log.Printf("Avec les arguments: %v", args)

	// 4. Exécuter la requête principale
	rows, err := config.DB.Query(query, args...)
	if err != nil {
		log.Printf("Erreur lors de la recherche d'annonces: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []models.Ad

	for rows.Next() {
		var ad models.Ad
		var images pq.StringArray
		var formDataStr, shopName, avatarURL sql.NullString
		var latitude, longitude sql.NullFloat64
		var firstName, lastName, accountType string
		var subCategoryName, categoryName string

		err := rows.Scan(
			&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
			&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &ad.IsDeliveryAvailable,
			&latitude, &longitude, &ad.CreatedAt,
			&ad.User.ID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
			&subCategoryName, &categoryName,
		)
		if err != nil {
			log.Printf("Erreur lors du scan d'une annonce: %v", err)
			continue
		}

		// Assigner les valeurs scannées
		ad.Images = []string(images)
		if formDataStr.Valid {
			json.Unmarshal([]byte(formDataStr.String), &ad.FormData)
		}
		if latitude.Valid {
			ad.Latitude = latitude
		}
		if longitude.Valid {
			ad.Longitude = longitude
		}

		ad.SubCategoryName = subCategoryName
		ad.CategoryName = categoryName
		ad.User.FirstName = firstName
		ad.User.LastName = lastName
		ad.User.ShopName = shopName
		ad.User.AvatarURL = avatarURL
		ad.User.IsProAccount = accountType == "Professionnel"

		// Définir le nom d'affichage
		if ad.User.IsProAccount && shopName.Valid {
			ad.User.DisplayName = shopName.String
		} else {
			ad.User.DisplayName = fmt.Sprintf("%s %s", firstName, lastName)
		}

		ads = append(ads, ad)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Erreur après l'itération des lignes: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 5. Compter le nombre total d'annonces pour la pagination
	var totalAds int
	err = config.DB.QueryRow(countQuery, countArgs...).Scan(&totalAds)
	if err != nil {
		log.Printf("Erreur lors du comptage des annonces: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 6. Préparer la réponse finale
	response := struct {
		Ads        []models.Ad `json:"ads"`
		Pagination struct {
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			TotalAds    int `json:"total_ads"`
			Limit       int `json:"limit"`
		} `json:"pagination"`
		Search struct {
			Query               string   `json:"query,omitempty"`
			CategoryID          *int     `json:"category_id,omitempty"`
			SubCategoryID       *int     `json:"sub_category_id,omitempty"`
			City                string   `json:"city,omitempty"`
			MinPrice            *float64 `json:"min_price,omitempty"`
			MaxPrice            *float64 `json:"max_price,omitempty"`
			IsDeliveryAvailable *bool    `json:"is_delivery_available,omitempty"`
			SortBy              string   `json:"sort_by"`
		} `json:"search"`
	}{
		Ads: ads,
	}

	response.Pagination.CurrentPage = page
	response.Pagination.TotalAds = totalAds
	response.Pagination.Limit = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit

	// Ajouter les informations de recherche à la réponse
	response.Search.Query = searchQuery
	response.Search.CategoryID = categoryID
	response.Search.SubCategoryID = subCategoryID
	response.Search.City = city
	response.Search.MinPrice = minPrice
	response.Search.MaxPrice = maxPrice
	response.Search.IsDeliveryAvailable = isDeliveryAvailable
	response.Search.SortBy = sortBy

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
	}

	log.Printf("Recherche traitée avec succès. Trouvé %d annonces sur %d total.", len(ads), totalAds)
}
