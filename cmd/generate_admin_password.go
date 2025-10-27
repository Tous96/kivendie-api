package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// Utilitaire pour générer un hash bcrypt pour le mot de passe admin
// Usage: go run generate_admin_password.go "VotreMotDePasse"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run generate_admin_password.go \"VotreMotDePasse\"")
	}

	password := os.Args[1]

	// Générer le hash avec bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Erreur lors de la génération du hash: %v", err)
	}

	fmt.Println("\n========================================")
	fmt.Println("Hash généré avec succès!")
	fmt.Println("========================================")
	fmt.Printf("\nMot de passe: %s\n", password)
	fmt.Printf("Hash bcrypt:  %s\n\n", string(hashedPassword))
	fmt.Println("Copiez ce hash dans votre fichier SQL ou utilisez-le pour créer un admin.")
	fmt.Println("\nExemple SQL:")
	fmt.Println("INSERT INTO admins (email, password_hash, first_name, last_name, role, is_active)")
	fmt.Printf("VALUES ('admin@kivendi.com', '%s', 'Super', 'Admin', 'admin', TRUE);\n\n", string(hashedPassword))
}
