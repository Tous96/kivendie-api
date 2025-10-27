package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"kivendi-backend/config"
	"kivendi-backend/models"
	"kivendi-backend/services"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

// GetAllAdsForAdminHandler récupère toutes les annonces pour le panel admin avec pagination et filtres.
// Les annonces en attente de validation sont affichées en premier.
func GetAllAdsForAdminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la récupération des annonces pour le panel admin.")

	// --- Pagination ---
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 10 {
		limit = 10 // Limite par défaut
	}
	offset := (page - 1) * limit

	// --- Récupération des données ---
	var ads []models.Ad
	var categories []models.Category
	var subCategories []models.SubCategory
	var totalAds int

	// Utilisation de goroutines pour paralléliser les appels à la base de données
	errChan := make(chan error, 4)
	defer close(errChan)

	// 1. Récupérer les annonces
	go func() {
		query := `
			SELECT 
				a.id, a.title, a.description, a.price, a.images, a.form_data, 
				a.city, a.phone_number, a.is_phone_visible, a.latitude, a.longitude,
				a.is_validated, a.is_deactivated, a.is_rejected, a.is_delivery_available, 
				a.is_sold, a.created_at, a.views_count,
				u.id as user_id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
				sc.name as sub_category_name, c.name as category_name
			FROM ads a
			JOIN users u ON a.user_id = u.id
			JOIN sub_categories sc ON a.sub_category_id = sc.id
			JOIN categories c ON sc.category_id = c.id
			ORDER BY 
				CASE 
					WHEN a.is_validated = false AND a.is_rejected = false AND a.is_deactivated = false THEN 1 
					ELSE 2 
				END, 
				a.created_at DESC
			LIMIT $1 OFFSET $2
		`
		rows, err := config.DB.Query(query, limit, offset)
		if err != nil {
			errChan <- err
			return
		}
		defer rows.Close()

		for rows.Next() {
			var ad models.Ad
			var images pq.StringArray
			var formDataStr, shopName, avatarURL sql.NullString
			var latitude, longitude sql.NullFloat64
			var firstName, lastName, accountType string

			if err := rows.Scan(
				&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
				&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &latitude, &longitude,
				&ad.IsValidated, &ad.IsDeactivated, &ad.IsRejected, &ad.IsDeliveryAvailable,
				&ad.IsSold, &ad.CreatedAt, &ad.ViewsCount,
				&ad.User.ID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
				&ad.SubCategoryName, &ad.CategoryName,
			); err != nil {
				errChan <- err
				return
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
			ad.User.FirstName = firstName
			ad.User.LastName = lastName
			ad.User.ShopName = shopName
			ad.User.AvatarURL = avatarURL
			ad.User.IsProAccount = accountType == "Professionnel"
			if ad.User.IsProAccount && shopName.Valid {
				ad.User.DisplayName = shopName.String
			} else {
				ad.User.DisplayName = firstName + " " + lastName
			}

			ads = append(ads, ad)
		}
		errChan <- rows.Err()
	}()

	// 2. Compter le total des annonces
	go func() {
		err := config.DB.QueryRow("SELECT COUNT(*) FROM ads").Scan(&totalAds)
		errChan <- err
	}()

	// 3. Récupérer les catégories
	go func() {
		rows, err := config.DB.Query("SELECT id, name, icon FROM categories ORDER BY name")
		if err != nil {
			errChan <- err
			return
		}
		defer rows.Close()
		for rows.Next() {
			var cat models.Category
			if err := rows.Scan(&cat.ID, &cat.Name, &cat.Icon); err != nil {
				errChan <- err
				return
			}
			categories = append(categories, cat)
		}
		errChan <- rows.Err()
	}()

	// 4. Récupérer les sous-catégories
	go func() {
		rows, err := config.DB.Query("SELECT id, name, icon, category_id FROM sub_categories ORDER BY name")
		if err != nil {
			errChan <- err
			return
		}
		defer rows.Close()
		for rows.Next() {
			var subCat models.SubCategory
			if err := rows.Scan(&subCat.ID, &subCat.Name, &subCat.Icon, &subCat.CategoryID); err != nil {
				errChan <- err
				return
			}
			subCategories = append(subCategories, subCat)
		}
		errChan <- rows.Err()
	}()

	// Attendre que toutes les goroutines se terminent et vérifier les erreurs
	for i := 0; i < 4; i++ {
		if err := <-errChan; err != nil {
			log.Printf("Erreur lors de la récupération des données pour le panel admin: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
			return
		}
	}

	// --- Préparation de la réponse ---
	response := struct {
		Ads           []models.Ad          `json:"ads"`
		Categories    []models.Category    `json:"categories"`
		SubCategories []models.SubCategory `json:"subCategories"`
		Pagination    struct {
			CurrentPage  int `json:"currentPage"`
			TotalItems   int `json:"totalItems"`
			ItemsPerPage int `json:"itemsPerPage"`
			TotalPages   int `json:"totalPages"`
		} `json:"pagination"`
	}{
		Ads:           ads,
		Categories:    categories,
		SubCategories: subCategories,
	}
	response.Pagination.CurrentPage = page
	response.Pagination.TotalItems = totalAds
	response.Pagination.ItemsPerPage = limit
	response.Pagination.TotalPages = (totalAds + limit - 1) / limit

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Erreur lors de l'encodage de la réponse JSON: %v", err)
	}
	log.Println("Annonces pour le panel admin récupérées avec succès.")
}

// GetAdDetailsForAdminHandler récupère les détails complets d'une annonce spécifique pour l'admin.
// GetAdDetailsForAdminHandler récupère les détails complets d'une annonce spécifique pour l'admin.
func GetAdDetailsForAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID, err := strconv.Atoi(vars["adID"])
	if err != nil {
		// Log de l'erreur si l'ID n'est pas un nombre valide
		log.Printf("ERREUR: ID d'annonce invalide fourni: %s. Erreur: %v", vars["adID"], err)
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Début de la récupération des détails pour l'annonce ID: %d", adID)

	query := `
		SELECT 
			a.id, a.title, a.description, a.price, a.images, a.form_data, 
			a.city, a.phone_number, a.is_phone_visible, a.latitude, a.longitude,
			a.is_validated, a.is_deactivated, a.is_rejected, a.is_delivery_available, 
			a.is_sold, a.created_at, a.updated_at, a.views_count,
			u.id as user_id, u.first_name, u.last_name, u.shop_name, u.account_type, u.avatar_url,
			sc.name as sub_category_name, c.name as category_name
		FROM ads a
		JOIN users u ON a.user_id = u.id
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.id = $1
	`

	var ad models.Ad
	var images pq.StringArray
	var formDataStr, shopName, avatarURL sql.NullString
	var latitude, longitude sql.NullFloat64
	var firstName, lastName, accountType string

	err = config.DB.QueryRow(query, adID).Scan(
		&ad.ID, &ad.Title, &ad.Description, &ad.Price, &images, &formDataStr,
		&ad.City, &ad.PhoneNumber, &ad.IsPhoneVisible, &latitude, &longitude,
		&ad.IsValidated, &ad.IsDeactivated, &ad.IsRejected, &ad.IsDeliveryAvailable,
		&ad.IsSold, &ad.CreatedAt, &ad.UpdatedAt, &ad.ViewsCount,
		&ad.User.ID, &firstName, &lastName, &shopName, &accountType, &avatarURL,
		&ad.SubCategoryName, &ad.CategoryName,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Log si l'annonce n'est pas trouvée dans la base de données
			log.Printf("INFO: Aucune annonce trouvée avec l'ID: %d", adID)
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			// Log de l'erreur lors de l'exécution de la requête SQL
			log.Printf("ERREUR DB: Impossible de récupérer les détails de l'annonce %d: %v", adID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// Hydratation des champs complexes de l'objet 'ad'
	ad.Images = []string(images)
	if formDataStr.Valid {
		if err := json.Unmarshal([]byte(formDataStr.String), &ad.FormData); err != nil {
			log.Printf("AVERTISSEMENT: Erreur lors du unmarshal de form_data pour l'annonce %d: %v", adID, err)
		}
	}
	ad.Latitude = latitude
	ad.Longitude = longitude

	// 1. Définir une structure de réponse qui correspond à ce que le frontend attend (avec des types string)
	type UserResponse struct {
		ID           int    `json:"id"`
		DisplayName  string `json:"display_name"`
		IsProAccount bool   `json:"is_pro_account"`
		AvatarURL    string `json:"avatar_url,omitempty"` // Type `string` et non `sql.NullString`
		ShopName     string `json:"shop_name,omitempty"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
	}

	type AdDetailResponse struct {
		models.Ad              // Intègre tous les champs de models.Ad
		User      UserResponse `json:"user"` // Surcharge le champ User pour utiliser notre nouvelle structure
	}

	// 2. Créer la réponse finale et la peupler avec les bonnes données et les bons types
	response := AdDetailResponse{
		Ad: ad, // Copie tous les champs de 'ad' vers la réponse
	}

	// 3. Peupler manuellement la partie User de la réponse en convertissant les types si nécessaire
	response.User.ID = ad.User.ID
	response.User.FirstName = firstName
	response.User.LastName = lastName
	response.User.IsProAccount = accountType == "Professionnel"

	if shopName.Valid {
		response.User.ShopName = shopName.String
	}
	if avatarURL.Valid {
		response.User.AvatarURL = avatarURL.String // C'est la correction clé !
	}

	// Calculer le DisplayName pour la réponse
	if response.User.IsProAccount && shopName.Valid {
		response.User.DisplayName = shopName.String
	} else {
		response.User.DisplayName = firstName + " " + lastName
	}

	// Log de la structure de réponse complète qui va être envoyée
	log.Printf("REPONSE: Envoi des données suivantes pour l'annonce ID %d: %+v", adID, response)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 4. Encoder la nouvelle structure 'response' (et non plus 'ad')
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("ERREUR JSON: Impossible d'encoder la réponse pour l'annonce %d: %v", adID, err)
	} else {
		log.Printf("SUCCÈS: Réponse pour l'annonce %d envoyée avec succès.", adID)
	}
}

// EditAdForAdminHandler gère la modification d'une annonce par un administrateur.
// Cette fonction permet à un admin de modifier tous les champs, y compris les images,
// sans vérifier la propriété de l'annonce et sans réinitialiser son statut de validation.
func EditAdForAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Récupérer l'ID de l'annonce depuis les variables de la requête
	vars := mux.Vars(r)
	adIDStr, exists := vars["adID"]
	if !exists {
		log.Println("Erreur admin: ID d'annonce manquant dans l'URL")
		http.Error(w, "ID d'annonce manquant", http.StatusBadRequest)
		return
	}

	adID, err := strconv.Atoi(adIDStr)
	if err != nil {
		log.Printf("Erreur admin: Conversion de l'ID d'annonce: %v", err)
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	log.Printf("Admin modifie l'annonce ID: %d", adID)

	// 2. Décoder le corps de la requête (utilise la struct AdUpdateRequest de ad_handler.go)
	var req AdUpdateRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Erreur admin: Décodage JSON: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// 3. Récupérer les images actuelles de l'annonce (sans vérifier le propriétaire)
	var currentImagesArray pq.StringArray
	err = config.DB.QueryRow("SELECT images FROM ads WHERE id = $1", adID).Scan(&currentImagesArray)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Erreur admin: Annonce non trouvée: %d", adID)
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur admin: Vérification de l'annonce: %v", err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	currentImages := []string(currentImagesArray)
	log.Printf("Annonce %d. Images actuelles: %v", adID, currentImages)

	// 4. Initialiser le service AWS
	awsService, err := services.NewAWSService()
	if err != nil {
		log.Printf("Erreur admin: Initialisation du service AWS: %v", err)
		http.Error(w, "Erreur de configuration du service de stockage", http.StatusInternalServerError)
		return
	}

	// 5. Traitement des nouvelles images à uploader (base64)
	var newUploadedImages []string
	if len(req.NewImages) > 0 {
		log.Printf("Admin upload de %d nouvelles images (base64) pour l'annonce %d", len(req.NewImages), adID)
		newUploadedImages, err = awsService.UploadAdImages(req.NewImages)
		if err != nil {
			log.Printf("Erreur admin: Upload des nouvelles images: %v", err)
			http.Error(w, "Erreur lors de l'upload des nouvelles images", http.StatusInternalServerError)
			return
		}
	}

	// 6. Identifier les images à supprimer réellement de S3
	var imagesToDeleteFromS3 []string
	for _, removedImg := range req.RemovedImages {
		for _, currentImg := range currentImages {
			if currentImg == removedImg {
				imagesToDeleteFromS3 = append(imagesToDeleteFromS3, removedImg)
				break
			}
		}
	}

	// 7. Supprimer les images de S3
	if len(imagesToDeleteFromS3) > 0 {
		log.Printf("Admin supprime %d images de S3 pour l'annonce %d", len(imagesToDeleteFromS3), adID)
		err = awsService.DeleteImages(imagesToDeleteFromS3)
		if err != nil {
			log.Printf("Erreur admin: Suppression des images S3: %v", err)
			// Ne pas bloquer la requête pour cette erreur
		}
	}

	// 8. Construire la liste finale des images
	finalImages := make([]string, 0)
	for _, img := range req.Images { // Images existantes à conserver
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
	finalImages = append(finalImages, newUploadedImages...) // Ajouter les nouvelles
	log.Printf("Annonce %d. Images finales: %v", adID, finalImages)

	// Vérifier qu'il reste au moins une image
	if len(finalImages) == 0 {
		log.Println("Erreur admin: Aucune image restante après mise à jour")
		http.Error(w, "Au moins une image est requise", http.StatusBadRequest)
		return
	}

	// 9. Mise à jour de l'annonce dans la base de données
	// Note: Ne réinitialise PAS is_validated, is_deactivated, is_rejected
	updateQuery := `
        UPDATE ads 
        SET title = $1, 
            description = $2, 
            images = $3, 
            phone_number = $4, 
            is_phone_visible = $5, 
            city = $6, 
            price = $7, 
            updated_at = NOW()
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
		log.Printf("Erreur admin: Mise à jour de l'annonce %d: %v", adID, err)
		// Rollback des images uploadées si la DB échoue
		if len(newUploadedImages) > 0 {
			awsService.DeleteImages(newUploadedImages)
		}
		http.Error(w, "Échec de la mise à jour de l'annonce", http.StatusInternalServerError)
		return
	}

	log.Printf("Annonce %d mise à jour avec succès par l'admin.", adID)

	// 10. Renvoyer une réponse de succès
	response := struct {
		Message string   `json:"message"`
		Images  []string `json:"images"`
	}{
		Message: "Annonce mise à jour avec succès par l'administrateur",
		Images:  finalImages,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetCategoriesForAdminHandler renvoie les catégories et leurs sous-catégories.
// Note : Cette fonction est identique à GetCategoriesWithSubCategories dans ad_handler.go
func GetCategoriesForAdminHandler(w http.ResponseWriter, r *http.Request) {
	// Création des tranches et cartes pour garantir qu'elles ne sont jamais nil
	categories := []models.Category{}
	subCategories := make(map[int][]models.SubCategory)

	rows, err := config.DB.Query(`
		SELECT id, name, icon FROM categories ORDER BY name
	`)
	if err != nil {
		log.Printf("Erreur admin lors de la récupération des catégories: %v", err)
		http.Error(w, "Erreur lors de la récupération des catégories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var cat models.Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Icon); err != nil {
			log.Printf("Erreur admin lors de la lecture des catégories: %v", err)
			http.Error(w, "Erreur lors de la lecture des catégories", http.StatusInternalServerError)
			return
		}
		categories = append(categories, cat)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Erreur admin lors de l'itération des catégories: %v", err)
		http.Error(w, "Erreur lors de l'itération des catégories", http.StatusInternalServerError)
		return
	}

	subCatRows, err := config.DB.Query(`
		SELECT id, name, icon, category_id FROM sub_categories ORDER BY name
	`)
	if err != nil {
		log.Printf("Erreur admin lors de la récupération des sous-catégories: %v", err)
		http.Error(w, "Erreur lors de la récupération des sous-catégories", http.StatusInternalServerError)
		return
	}
	defer subCatRows.Close()

	for subCatRows.Next() {
		var sub models.SubCategory
		if err := subCatRows.Scan(&sub.ID, &sub.Name, &sub.Icon, &sub.CategoryID); err != nil {
			log.Printf("Erreur admin lors de la lecture des sous-catégories: %v", err)
			http.Error(w, "Erreur lors de la lecture des sous-catégories", http.StatusInternalServerError)
			return
		}
		subCategories[sub.CategoryID] = append(subCategories[sub.CategoryID], sub)
	}
	if err = subCatRows.Err(); err != nil {
		log.Printf("Erreur admin lors de l'itération des sous-catégories: %v", err)
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
		log.Printf("Erreur admin lors de l'encodage de la réponse JSON: %v", err)
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
		return
	}
}

// ============== GESTION DES CATÉGORIES ==============

// CreateCategoryHandler crée une nouvelle catégorie
func CreateCategoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name string `json:"name"`
		Icon string `json:"icon"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour création de catégorie: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Validation
	if req.Name == "" {
		http.Error(w, "Le nom de la catégorie est requis", http.StatusBadRequest)
		return
	}
	if req.Icon == "" {
		http.Error(w, "L'icône de la catégorie est requise", http.StatusBadRequest)
		return
	}

	// Vérifier si la catégorie existe déjà
	var exists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE name = $1)", req.Name).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de la catégorie: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Une catégorie avec ce nom existe déjà", http.StatusConflict)
		return
	}

	// Insertion
	var categoryID int
	query := "INSERT INTO categories (name, icon) VALUES ($1, $2) RETURNING id"
	err = config.DB.QueryRow(query, req.Name, req.Icon).Scan(&categoryID)
	if err != nil {
		log.Printf("Erreur admin: Insertion de la catégorie: %v", err)
		http.Error(w, "Erreur lors de la création de la catégorie", http.StatusInternalServerError)
		return
	}

	log.Printf("Catégorie créée avec succès: ID=%d, Name=%s", categoryID, req.Name)

	response := struct {
		Message  string          `json:"message"`
		Category models.Category `json:"category"`
	}{
		Message: "Catégorie créée avec succès",
		Category: models.Category{
			ID:   categoryID,
			Name: req.Name,
			Icon: req.Icon,
		},
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateCategoryHandler modifie une catégorie existante
func UpdateCategoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	categoryID, err := strconv.Atoi(vars["categoryID"])
	if err != nil {
		http.Error(w, "ID de catégorie invalide", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
		Icon string `json:"icon"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour modification de catégorie: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Validation
	if req.Name == "" {
		http.Error(w, "Le nom de la catégorie est requis", http.StatusBadRequest)
		return
	}
	if req.Icon == "" {
		http.Error(w, "L'icône de la catégorie est requise", http.StatusBadRequest)
		return
	}

	// Vérifier si une autre catégorie avec ce nom existe déjà
	var exists bool
	err = config.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM categories WHERE name = $1 AND id != $2)",
		req.Name, categoryID,
	).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de la catégorie: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Une catégorie avec ce nom existe déjà", http.StatusConflict)
		return
	}

	// Mise à jour
	query := "UPDATE categories SET name = $1, icon = $2 WHERE id = $3"
	result, err := config.DB.Exec(query, req.Name, req.Icon, categoryID)
	if err != nil {
		log.Printf("Erreur admin: Mise à jour de la catégorie %d: %v", categoryID, err)
		http.Error(w, "Erreur lors de la mise à jour de la catégorie", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Catégorie non trouvée", http.StatusNotFound)
		return
	}

	log.Printf("Catégorie %d mise à jour avec succès", categoryID)

	response := struct {
		Message  string          `json:"message"`
		Category models.Category `json:"category"`
	}{
		Message: "Catégorie mise à jour avec succès",
		Category: models.Category{
			ID:   categoryID,
			Name: req.Name,
			Icon: req.Icon,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeleteCategoryHandler supprime une catégorie et ses sous-catégories
func DeleteCategoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	categoryID, err := strconv.Atoi(vars["categoryID"])
	if err != nil {
		http.Error(w, "ID de catégorie invalide", http.StatusBadRequest)
		return
	}

	// Vérifier s'il y a des annonces liées à cette catégorie
	var adCount int
	err = config.DB.QueryRow(`
		SELECT COUNT(*) FROM ads a
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		WHERE sc.category_id = $1
	`, categoryID).Scan(&adCount)
	if err != nil {
		log.Printf("Erreur admin: Vérification des annonces liées à la catégorie %d: %v", categoryID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	if adCount > 0 {
		http.Error(w, fmt.Sprintf("Impossible de supprimer la catégorie: %d annonce(s) y sont liées", adCount), http.StatusConflict)
		return
	}

	// Suppression (CASCADE supprimera automatiquement les sous-catégories)
	query := "DELETE FROM categories WHERE id = $1"
	result, err := config.DB.Exec(query, categoryID)
	if err != nil {
		log.Printf("Erreur admin: Suppression de la catégorie %d: %v", categoryID, err)
		http.Error(w, "Erreur lors de la suppression de la catégorie", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Catégorie non trouvée", http.StatusNotFound)
		return
	}

	log.Printf("Catégorie %d supprimée avec succès", categoryID)

	response := map[string]string{
		"message": "Catégorie supprimée avec succès",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ============== GESTION DES SOUS-CATÉGORIES ==============

// CreateSubCategoryHandler crée une nouvelle sous-catégorie
func CreateSubCategoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		CategoryID int    `json:"category_id"`
		Name       string `json:"name"`
		Icon       string `json:"icon"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour création de sous-catégorie: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Validation
	if req.CategoryID == 0 {
		http.Error(w, "L'ID de la catégorie parent est requis", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Le nom de la sous-catégorie est requis", http.StatusBadRequest)
		return
	}
	if req.Icon == "" {
		http.Error(w, "L'icône de la sous-catégorie est requise", http.StatusBadRequest)
		return
	}

	// Vérifier que la catégorie parent existe
	var categoryExists bool
	err := config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)", req.CategoryID).Scan(&categoryExists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de la catégorie parent: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if !categoryExists {
		http.Error(w, "Catégorie parent non trouvée", http.StatusNotFound)
		return
	}

	// Vérifier si la sous-catégorie existe déjà dans cette catégorie
	var exists bool
	err = config.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM sub_categories WHERE name = $1 AND category_id = $2)",
		req.Name, req.CategoryID,
	).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de la sous-catégorie: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Une sous-catégorie avec ce nom existe déjà dans cette catégorie", http.StatusConflict)
		return
	}

	// Insertion
	var subCategoryID int
	query := "INSERT INTO sub_categories (category_id, name, icon) VALUES ($1, $2, $3) RETURNING id"
	err = config.DB.QueryRow(query, req.CategoryID, req.Name, req.Icon).Scan(&subCategoryID)
	if err != nil {
		log.Printf("Erreur admin: Insertion de la sous-catégorie: %v", err)
		http.Error(w, "Erreur lors de la création de la sous-catégorie", http.StatusInternalServerError)
		return
	}

	log.Printf("Sous-catégorie créée avec succès: ID=%d, Name=%s, CategoryID=%d", subCategoryID, req.Name, req.CategoryID)

	response := struct {
		Message     string             `json:"message"`
		SubCategory models.SubCategory `json:"subCategory"`
	}{
		Message: "Sous-catégorie créée avec succès",
		SubCategory: models.SubCategory{
			ID:         subCategoryID,
			CategoryID: req.CategoryID,
			Name:       req.Name,
			Icon:       req.Icon,
		},
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UpdateSubCategoryHandler modifie une sous-catégorie existante
func UpdateSubCategoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	subCategoryID, err := strconv.Atoi(vars["subCategoryID"])
	if err != nil {
		http.Error(w, "ID de sous-catégorie invalide", http.StatusBadRequest)
		return
	}

	var req struct {
		CategoryID int    `json:"category_id"`
		Name       string `json:"name"`
		Icon       string `json:"icon"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Erreur admin: Décodage JSON pour modification de sous-catégorie: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Validation
	if req.CategoryID == 0 {
		http.Error(w, "L'ID de la catégorie parent est requis", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Le nom de la sous-catégorie est requis", http.StatusBadRequest)
		return
	}
	if req.Icon == "" {
		http.Error(w, "L'icône de la sous-catégorie est requise", http.StatusBadRequest)
		return
	}

	// Vérifier que la catégorie parent existe
	var categoryExists bool
	err = config.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)", req.CategoryID).Scan(&categoryExists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de la catégorie parent: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if !categoryExists {
		http.Error(w, "Catégorie parent non trouvée", http.StatusNotFound)
		return
	}

	// Vérifier si une autre sous-catégorie avec ce nom existe dans la même catégorie
	var exists bool
	err = config.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM sub_categories WHERE name = $1 AND category_id = $2 AND id != $3)",
		req.Name, req.CategoryID, subCategoryID,
	).Scan(&exists)
	if err != nil {
		log.Printf("Erreur admin: Vérification de l'existence de la sous-catégorie: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Une sous-catégorie avec ce nom existe déjà dans cette catégorie", http.StatusConflict)
		return
	}

	// Mise à jour
	query := "UPDATE sub_categories SET category_id = $1, name = $2, icon = $3 WHERE id = $4"
	result, err := config.DB.Exec(query, req.CategoryID, req.Name, req.Icon, subCategoryID)
	if err != nil {
		log.Printf("Erreur admin: Mise à jour de la sous-catégorie %d: %v", subCategoryID, err)
		http.Error(w, "Erreur lors de la mise à jour de la sous-catégorie", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Sous-catégorie non trouvée", http.StatusNotFound)
		return
	}

	log.Printf("Sous-catégorie %d mise à jour avec succès", subCategoryID)

	response := struct {
		Message     string             `json:"message"`
		SubCategory models.SubCategory `json:"subCategory"`
	}{
		Message: "Sous-catégorie mise à jour avec succès",
		SubCategory: models.SubCategory{
			ID:         subCategoryID,
			CategoryID: req.CategoryID,
			Name:       req.Name,
			Icon:       req.Icon,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeleteSubCategoryHandler supprime une sous-catégorie
func DeleteSubCategoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	subCategoryID, err := strconv.Atoi(vars["subCategoryID"])
	if err != nil {
		http.Error(w, "ID de sous-catégorie invalide", http.StatusBadRequest)
		return
	}

	// Vérifier s'il y a des annonces liées à cette sous-catégorie
	var adCount int
	err = config.DB.QueryRow("SELECT COUNT(*) FROM ads WHERE sub_category_id = $1", subCategoryID).Scan(&adCount)
	if err != nil {
		log.Printf("Erreur admin: Vérification des annonces liées à la sous-catégorie %d: %v", subCategoryID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	if adCount > 0 {
		http.Error(w, fmt.Sprintf("Impossible de supprimer la sous-catégorie: %d annonce(s) y sont liées", adCount), http.StatusConflict)
		return
	}

	// Suppression
	query := "DELETE FROM sub_categories WHERE id = $1"
	result, err := config.DB.Exec(query, subCategoryID)
	if err != nil {
		log.Printf("Erreur admin: Suppression de la sous-catégorie %d: %v", subCategoryID, err)
		http.Error(w, "Erreur lors de la suppression de la sous-catégorie", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Sous-catégorie non trouvée", http.StatusNotFound)
		return
	}

	log.Printf("Sous-catégorie %d supprimée avec succès", subCategoryID)

	response := map[string]string{
		"message": "Sous-catégorie supprimée avec succès",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ============== NOUVELLES FONCTIONS DE MODÉRATION (MISES À JOUR) ==============

// getAdInfoForNotification récupère l'ID de l'utilisateur et le titre d'une annonce.
func getAdInfoForNotification(adID int) (userID int, adTitle string, err error) {
	err = config.DB.QueryRow("SELECT user_id, title FROM ads WHERE id = $1", adID).Scan(&userID, &adTitle)
	return
}

// ValidateAdHandler valide une annonce et notifie l'utilisateur.
func ValidateAdHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID, err := strconv.Atoi(vars["adID"])
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// Récupérer les infos pour la notification avant la mise à jour
	userID, adTitle, err := getAdInfoForNotification(adID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la récupération des infos de l'annonce %d: %v", adID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// Assure que les autres statuts sont bien à FALSE
	query := `
		UPDATE ads 
		SET is_validated = TRUE, is_rejected = FALSE, is_deactivated = FALSE, updated_at = NOW()
		WHERE id = $1
	`
	result, err := config.DB.Exec(query, adID)
	if err != nil {
		log.Printf("Erreur lors de la validation de l'annonce %d: %v", adID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		return
	}

	// Envoyer la notification (in-app)
	notificationTitle := "Votre annonce a été approuvée !"
	notificationMessage := fmt.Sprintf("Bonne nouvelle ! Votre annonce « %s » a été validée et est maintenant visible par tous.", adTitle)
	go services.CreateNotification(userID, "ad_validated", notificationTitle, notificationMessage, map[string]interface{}{"adId": adID})

	// Envoyer la notification (push)
	if services.PushSvc != nil {
		services.PushSvc.SendAdValidatedPush(r.Context(), userID, adTitle, adID)
	}

	log.Printf("Annonce %d validée avec succès. Notification envoyée à l'utilisateur %d.", adID, userID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Annonce validée avec succès"})
}

// RejectAdHandler rejette une annonce et notifie l'utilisateur avec la raison.
func RejectAdHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID, err := strconv.Atoi(vars["adID"])
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Corps de la requête invalide", http.StatusBadRequest)
		return
	}
	if payload.Reason == "" {
		payload.Reason = "non spécifiée" // Raison par défaut
	}

	// Récupérer les infos pour la notification
	userID, adTitle, err := getAdInfoForNotification(adID)
	if err != nil {
		http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		return
	}

	// Mise à jour du statut, SANS sauvegarder la raison dans la DB
	query := `
		UPDATE ads 
		SET is_validated = FALSE, is_rejected = TRUE, is_deactivated = FALSE, updated_at = NOW()
		WHERE id = $1
	`
	_, err = config.DB.Exec(query, adID)
	if err != nil {
		log.Printf("Erreur lors du rejet de l'annonce %d: %v", adID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Envoyer la notification (in-app)
	notificationTitle := "Votre annonce a été rejetée"
	notificationMessage := fmt.Sprintf("Malheureusement, votre annonce « %s » n'a pas pu être validée. Raison : %s", adTitle, payload.Reason)
	go services.CreateNotification(userID, "ad_rejected", notificationTitle, notificationMessage, map[string]interface{}{"adId": adID})

	// Envoyer la notification (push)
	if services.PushSvc != nil {
		services.PushSvc.SendAdRejectedPush(r.Context(), userID, adTitle, adID, payload.Reason)
	}

	log.Printf("Annonce %d rejetée. Raison: %s. Notification envoyée.", adID, payload.Reason)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Annonce rejetée avec succès"})
}

// DeactivateAdHandler désactive une annonce et notifie l'utilisateur.
func DeactivateAdHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID, err := strconv.Atoi(vars["adID"])
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
		return
	}

	// Récupérer les infos pour la notification
	userID, adTitle, err := getAdInfoForNotification(adID)
	if err != nil {
		http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		return
	}

	// La désactivation rend les autres états invalides
	query := `
		UPDATE ads 
		SET is_deactivated = TRUE, is_validated = FALSE, is_rejected = FALSE, updated_at = NOW()
		WHERE id = $1
	`
	_, err = config.DB.Exec(query, adID)
	if err != nil {
		log.Printf("Erreur lors de la désactivation de l'annonce %d: %v", adID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Envoyer la notification (in-app)
	notificationTitle := "Votre annonce a été désactivée"
	notificationMessage := fmt.Sprintf("Votre annonce « %s » a été désactivée par un administrateur. Elle n'est plus visible sur la plateforme.", adTitle)
	go services.CreateNotification(userID, "ad_deactivated", notificationTitle, notificationMessage, map[string]interface{}{"adId": adID})

	// Envoyer la notification (push)
	if services.PushSvc != nil {
		services.PushSvc.SendAdDeactivatedPush(r.Context(), userID, adTitle, adID)
	}

	log.Printf("Annonce %d désactivée avec succès. Notification envoyée.", adID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Annonce désactivée avec succès"})
}

// DeleteAdForAdminHandler supprime une annonce, ses images et notifie l'utilisateur.
func DeleteAdForAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID, err := strconv.Atoi(vars["adID"])
	if err != nil {
		http.Error(w, "ID d'annonce invalide", http.StatusBadRequest)
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

	// 1. Récupérer les infos nécessaires avant de supprimer l'annonce
	var images pq.StringArray
	var userID int
	var adTitle string
	err = tx.QueryRow("SELECT user_id, title, images FROM ads WHERE id = $1", adID).Scan(&userID, &adTitle, &images)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Annonce non trouvée", http.StatusNotFound)
		} else {
			log.Printf("Erreur lors de la récupération des infos de l'annonce %d: %v", adID, err)
			http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		}
		return
	}

	// 2. Supprimer l'annonce de la base de données
	_, err = tx.Exec("DELETE FROM ads WHERE id = $1", adID)
	if err != nil {
		log.Printf("Erreur lors de la suppression de l'annonce %d: %v", adID, err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 3. Valider la transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Erreur lors de la validation de la transaction: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// 4. Actions post-transaction (notifications, suppression d'images)
	// Lancer en goroutines pour ne pas bloquer la réponse HTTP

	// Envoyer la notification (in-app)
	notificationTitle := "Votre annonce a été supprimée"
	notificationMessage := fmt.Sprintf("Votre annonce « %s » a été supprimée par un administrateur car elle ne respectait pas nos conditions d'utilisation.", adTitle)
	go services.CreateNotification(userID, "ad_deleted", notificationTitle, notificationMessage, nil) // Pas de data supplémentaire nécessaire

	// Envoyer la notification (push)
	if services.PushSvc != nil {
		services.PushSvc.SendAdDeletedPush(r.Context(), userID, adTitle)
	}

	// Supprimer les images de S3
	if len(images) > 0 {
		go func() {
			awsService, err := services.NewAWSService()
			if err != nil {
				log.Printf("CRITIQUE: Impossible d'initier le service AWS pour l'annonce %d. Erreur: %v", adID, err)
				return
			}
			if err := awsService.DeleteImages([]string(images)); err != nil {
				log.Printf("ERREUR S3: La suppression des images de l'annonce %d a échoué: %v", adID, err)
			} else {
				log.Printf("Images de l'annonce %d supprimées de S3.", adID)
			}
		}()
	}

	log.Printf("Annonce %d supprimée. Notification et suppression S3 initiées.", adID)
	w.WriteHeader(http.StatusNoContent)
}
