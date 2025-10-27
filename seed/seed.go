package seed

import (
	"database/sql"
	"log"
)

// SubCategoryData représente la structure d'une sous-catégorie
type SubCategoryData struct {
	Name string
	Icon string
}

// CategoryData représente la structure d'une catégorie avec ses sous-catégories
type CategoryData struct {
	Name          string
	Icon          string
	SubCategories []SubCategoryData
}

// categories contient toutes les données de catégorie et de sous-catégorie à insérer
var categories = []CategoryData{
	{
		Name: "Immobilier",
		Icon: "fa-solid fa-house",
		SubCategories: []SubCategoryData{
			{Name: "Vente immobilière", Icon: "fa-solid fa-building"},
			{Name: "Locations", Icon: "fa-solid fa-building-user"},
			{Name: "Colocations", Icon: "fa-solid fa-people-roof"},
			{Name: "Terrains & fermes", Icon: "fa-solid fa-mountain-sun"},
			{Name: "Bureaux, commerces & entrepôts", Icon: "fa-solid fa-store"},
		},
	},
	{
		Name: "Véhicules",
		Icon: "fa-solid fa-car",
		SubCategories: []SubCategoryData{
			{Name: "Voitures", Icon: "fa-solid fa-car"},
			{Name: "Motos", Icon: "fa-solid fa-motorcycle"},
			{Name: "Véhicules utilitaires", Icon: "fa-solid fa-truck"},
			{Name: "Camions & bus", Icon: "fa-solid fa-bus"},
			{Name: "Pièces & accessoires", Icon: "fa-solid fa-gears"},
		},
	},
	{
		Name: "Électronique & High-tech",
		Icon: "fa-solid fa-mobile-screen-button",
		SubCategories: []SubCategoryData{
			{Name: "Téléphones", Icon: "fa-solid fa-mobile-screen-button"},
			{Name: "Ordinateurs & tablettes", Icon: "fa-solid fa-laptop"},
			{Name: "Consoles de jeux & jeux vidéo", Icon: "fa-solid fa-gamepad"},
			{Name: "TV & vidéo", Icon: "fa-solid fa-tv"},
			{Name: "Appareils photo & caméras", Icon: "fa-solid fa-camera"},
			{Name: "Casques & écouteurs", Icon: "fa-solid fa-headphones"},
			{Name: "Accessoires & divers", Icon: "fa-solid fa-plug"},
		},
	},
	{
		Name: "Services",
		Icon: "fa-solid fa-handshake",
		SubCategories: []SubCategoryData{
			{Name: "Événements & mariage", Icon: "fa-solid fa-calendar-days"},
			{Name: "Formations & cours", Icon: "fa-solid fa-chalkboard-user"},
			{Name: "Assurances & finance", Icon: "fa-solid fa-money-bill-transfer"},
			{Name: "Réparations & bricolage", Icon: "fa-solid fa-screwdriver-wrench"},
			{Name: "Santé & bien-être", Icon: "fa-solid fa-hand-holding-heart"},
		},
	},
	{
		Name: "Mode & Beauté",
		Icon: "fa-solid fa-shirt",
		SubCategories: []SubCategoryData{
			{Name: "Vêtements & chaussures", Icon: "fa-solid fa-shirt"},
			{Name: "Sacs & accessoires", Icon: "fa-solid fa-bag-shopping"},
			{Name: "Bijoux & montres", Icon: "fa-solid fa-gem"},
			{Name: "Produits de beauté", Icon: "fa-solid fa-spray-can"},
			{Name: "Maquillage & soins", Icon: "fa-solid fa-mask-face"},
		},
	},
	{
		Name: "Maison & Jardin",
		Icon: "fa-solid fa-couch",
		SubCategories: []SubCategoryData{
			{Name: "Ameublement", Icon: "fa-solid fa-chair"},
			{Name: "Décoration", Icon: "fa-solid fa-paint-roller"},
			{Name: "Appareils électroménagers", Icon: "fa-solid fa-blender-phone"},
			{Name: "Jardin & extérieur", Icon: "fa-solid fa-seedling"},
			{Name: "Matériaux de construction", Icon: "fa-solid fa-hard-hat"},
		},
	},
	{
		Name: "Animaux",
		Icon: "fa-solid fa-paw",
		SubCategories: []SubCategoryData{
			{Name: "Chats", Icon: "fa-solid fa-cat"},
			{Name: "Chiens", Icon: "fa-solid fa-dog"},
			{Name: "Oiseaux", Icon: "fa-solid fa-dove"},
			{Name: "Poissons & aquariums", Icon: "fa-solid fa-fish"},
			{Name: "Accessoires & nourriture", Icon: "fa-solid fa-bone"},
		},
	},
	{
		Name: "Loisirs & Divertissement",
		Icon: "fa-solid fa-palette",
		SubCategories: []SubCategoryData{
			{Name: "Livres & magazines", Icon: "fa-solid fa-book"},
			{Name: "Instruments de musique", Icon: "fa-solid fa-guitar"},
			{Name: "Articles de sport", Icon: "fa-solid fa-basketball"},
			{Name: "Art & Artisanat", Icon: "fa-solid fa-palette"},
			{Name: "Voyages & billets", Icon: "fa-solid fa-plane-up"},
		},
	},
	{
		Name: "Enfant & Bébé",
		Icon: "fa-solid fa-child",
		SubCategories: []SubCategoryData{
			{Name: "Vêtements pour enfants", Icon: "fa-solid fa-child-dress"},
			{Name: "Jouets & jeux", Icon: "fa-solid fa-toy-dinosaur"},
			{Name: "Poussettes & matériel de puériculture", Icon: "fa-solid fa-baby-carriage"},
			{Name: "Mobilier & Décoration", Icon: "fa-solid fa-bed"},
		},
	},
	{
		Name: "Cuisine & Pâtisserie",
		Icon: "fa-solid fa-utensils-cross",
		SubCategories: []SubCategoryData{
			{Name: "Produits alimentaires", Icon: "fa-solid fa-carrot"},
			{Name: "Gâteaux & pâtisseries", Icon: "fa-solid fa-cake-candles"},
			{Name: "Boissons & jus", Icon: "fa-solid fa-bottle-water"},
			{Name: "Ustensiles de cuisine", Icon: "fa-solid fa-utensils"},
			{Name: "Équipements professionnels", Icon: "fa-solid fa-blender"},
		},
	},
}

// Seed insère les données de base dans la base de données
func Seed(DB *sql.DB) {
	tx, err := DB.Begin()
	if err != nil {
		log.Fatalf("Impossible de commencer la transaction : %s", err)
	}

	// Suppression des données existantes pour éviter les doublons
	_, err = tx.Exec("DELETE FROM sub_categories")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Impossible de vider la table des sous-catégories : %s", err)
	}
	_, err = tx.Exec("DELETE FROM categories")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Impossible de vider la table des catégories : %s", err)
	}

	// Préparation des requêtes d'insertion
	stmtCategory, err := tx.Prepare("INSERT INTO categories (name, icon) VALUES ($1, $2) RETURNING id")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Erreur de préparation de la requête d'insertion de catégorie : %s", err)
	}
	defer stmtCategory.Close()

	stmtSubCategory, err := tx.Prepare("INSERT INTO sub_categories (category_id, name, icon) VALUES ($1, $2, $3) RETURNING id")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Erreur de préparation de la requête d'insertion de sous-catégorie : %s", err)
	}
	defer stmtSubCategory.Close()

	for _, catData := range categories {
		var catID int
		err := stmtCategory.QueryRow(catData.Name, catData.Icon).Scan(&catID)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Impossible d'insérer la catégorie %s: %s", catData.Name, err)
		}

		for _, subCatData := range catData.SubCategories {
			_, err := stmtSubCategory.Exec(catID, subCatData.Name, subCatData.Icon)
			if err != nil {
				tx.Rollback()
				log.Fatalf("Impossible d'insérer la sous-catégorie %s: %s", subCatData.Name, err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatalf("Impossible de valider la transaction : %s", err)
	}

	log.Println("Données de base insérées avec succès (catégories et sous-catégories)")
}
