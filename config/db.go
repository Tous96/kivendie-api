package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	_ "github.com/lib/pq"
)

// Déclaration de la variable de la base de données
var DB *sql.DB

// InitDB initialise la connexion à la base de données
// en utilisant les variables d'environnement.
func InitDB() {
	var err error
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")
	dbPortStr := os.Getenv("DB_PORT")
	dbPort, err := strconv.Atoi(dbPortStr)
	if err != nil {
		log.Fatalf("Le port de la base de données n'est pas un nombre valide : %s", err)
	}

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=disable",
		dbUser, dbPassword, dbName, dbHost, dbPort)

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Erreur de connexion à la base de données : %s", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Impossible de se connecter à la base de données : %s", err)
	}

	fmt.Println("Connexion à la base de données établie avec succès !")

	//createTables()
}

// createTables gère la création des tables dans la base de données
// si elles n'existent pas déjà.
func createTables() {
	// ========================================
	// TABLE USERS (DOIT ÊTRE CRÉÉE EN PREMIER)
	// ========================================
	log.Println("Création de la table users...")
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			first_name VARCHAR(100) NOT NULL,
			last_name VARCHAR(100) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			account_type VARCHAR(20) NOT NULL CHECK (account_type IN ('Personnel', 'Professionnel')),
			shop_name VARCHAR(255),
			avatar_url VARCHAR(500),
			verification_code VARCHAR(10),
			is_verified BOOLEAN DEFAULT FALSE NOT NULL,
			is_blocked BOOLEAN DEFAULT FALSE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
			
			-- Contrainte pour vérifier le format de l'email
			CONSTRAINT valid_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table users : %s", err)
	}

	// Migration: Ajouter les colonnes manquantes à la table users si elle existe déjà
	_, err = DB.Exec(`
		DO $$ 
		BEGIN
			-- Ajouter is_blocked si elle n'existe pas
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'users' AND column_name = 'is_blocked'
			) THEN
				ALTER TABLE users ADD COLUMN is_blocked BOOLEAN DEFAULT FALSE NOT NULL;
				RAISE NOTICE 'Colonne is_blocked ajoutée à la table users';
			END IF;

			-- Ajouter created_at si elle n'existe pas
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'users' AND column_name = 'created_at'
			) THEN
				ALTER TABLE users ADD COLUMN created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL;
				RAISE NOTICE 'Colonne created_at ajoutée à la table users';
			END IF;

			-- Ajouter updated_at si elle n'existe pas
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'users' AND column_name = 'updated_at'
			) THEN
				ALTER TABLE users ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL;
				RAISE NOTICE 'Colonne updated_at ajoutée à la table users';
			END IF;

			-- Vérifier et modifier la longueur de avatar_url si nécessaire
			IF EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'users' 
				AND column_name = 'avatar_url'
				AND character_maximum_length < 500
			) THEN
				ALTER TABLE users ALTER COLUMN avatar_url TYPE VARCHAR(500);
				RAISE NOTICE 'Colonne avatar_url étendue à 500 caractères';
			END IF;
		END $$;
	`)
	if err != nil {
		log.Printf("Attention lors de la migration de la table users : %s", err)
	}

	// Index pour améliorer les performances de la table users
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_is_verified ON users(is_verified);
		CREATE INDEX IF NOT EXISTS idx_users_is_blocked ON users(is_blocked);
		CREATE INDEX IF NOT EXISTS idx_users_account_type ON users(account_type);
		CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour users : %s", err)
	}

	// Trigger pour mettre à jour automatiquement updated_at sur la table users
	_, err = DB.Exec(`
		CREATE OR REPLACE FUNCTION update_users_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		DROP TRIGGER IF EXISTS trigger_update_users_updated_at ON users;
		CREATE TRIGGER trigger_update_users_updated_at
			BEFORE UPDATE ON users
			FOR EACH ROW
			EXECUTE FUNCTION update_users_updated_at();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour users : %s", err)
	}

	// Commentaires sur la table users
	_, err = DB.Exec(`
		COMMENT ON TABLE users IS 'Table des utilisateurs de la plateforme';
		COMMENT ON COLUMN users.account_type IS 'Type de compte: Personnel ou Professionnel';
		COMMENT ON COLUMN users.shop_name IS 'Nom de la boutique (uniquement pour les comptes professionnels)';
		COMMENT ON COLUMN users.is_verified IS 'Indique si l''email de l''utilisateur a été vérifié';
		COMMENT ON COLUMN users.is_blocked IS 'Indique si le compte utilisateur est bloqué par un admin';
		COMMENT ON COLUMN users.verification_code IS 'Code de vérification temporaire envoyé par email';
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'ajouter les commentaires à la table users : %s", err)
	}

	log.Println("✓ Table users créée avec succès")

	// ========================================
	// RESTE DES TABLES
	// ========================================

	// Table des catégories
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			icon VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des catégories : %s", err)
	}

	// Table des sous-catégories
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS sub_categories (
			id SERIAL PRIMARY KEY,
			category_id INTEGER REFERENCES categories(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			icon VARCHAR(255) NOT NULL
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des sous-catégories : %s", err)
	}

	// Table des annonces
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS ads (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id),
			title VARCHAR(255) NOT NULL,
			description TEXT NOT NULL,
			price DECIMAL(10, 2) NOT NULL,
			sub_category_id INTEGER REFERENCES sub_categories(id),
			is_validated BOOLEAN DEFAULT FALSE,
			is_deactivated BOOLEAN DEFAULT FALSE,
			is_rejected BOOLEAN DEFAULT FALSE,
			images TEXT[] DEFAULT '{}',
			form_data JSONB,
			city VARCHAR(255),
			phone_number VARCHAR(255),
			is_phone_visible BOOLEAN DEFAULT FALSE,
			is_delivery_available BOOLEAN DEFAULT FALSE,
			latitude DOUBLE PRECISION,
			longitude DOUBLE PRECISION,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des annonces : %s", err)
	}

	// Ajout des nouvelles colonnes si elles n'existent pas
	_, err = DB.Exec(`
		DO $$ 
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='ads' AND column_name='city') THEN
				ALTER TABLE ads ADD COLUMN city VARCHAR(255);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='ads' AND column_name='phone_number') THEN
				ALTER TABLE ads ADD COLUMN phone_number VARCHAR(255);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='ads' AND column_name='is_phone_visible') THEN
				ALTER TABLE ads ADD COLUMN is_phone_visible BOOLEAN DEFAULT FALSE;
			END IF;
		END $$;
	`)
	if err != nil {
		log.Fatalf("Impossible d'ajouter les nouvelles colonnes à la table des annonces : %s", err)
	}

	// Ajout de la table des favoris (Favorites)
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS favorites (
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			ad_id INTEGER REFERENCES ads(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, ad_id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des favoris : %s", err)
	}

	// Création de la table 'conversations'
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id SERIAL PRIMARY KEY,
			ad_id INTEGER REFERENCES ads(id) ON DELETE CASCADE,
			seller_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			buyer_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des conversations : %s", err)
	}

	// Création de la table 'messages'
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			conversation_id INTEGER REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			text TEXT,
			offer_amount DECIMAL(10, 2),
			type VARCHAR(50) NOT NULL,
			is_read BOOLEAN DEFAULT FALSE,
			image_urls TEXT[] DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des messages : %s", err)
	}

	// Table pour les blocages d'utilisateurs
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_blocks (
			id SERIAL PRIMARY KEY,
			blocker_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			blocked_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(blocker_id, blocked_id)
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des blocages d'utilisateurs : %s", err)
	}

	// Table pour les signalements d'utilisateurs
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_reports (
			id SERIAL PRIMARY KEY,
			reporter_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			reported_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			reason TEXT NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			admin_notes TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(reporter_id, reported_id, conversation_id)
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des signalements d'utilisateurs : %s", err)
	}

	// Index pour améliorer les performances
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_user_blocks_blocker_id ON user_blocks(blocker_id);
		CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked_id ON user_blocks(blocked_id);
		CREATE INDEX IF NOT EXISTS idx_user_reports_reporter_id ON user_reports(reporter_id);
		CREATE INDEX IF NOT EXISTS idx_user_reports_reported_id ON user_reports(reported_id);
		CREATE INDEX IF NOT EXISTS idx_user_reports_conversation_id ON user_reports(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_user_reports_status ON user_reports(status);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index : %s", err)
	}

	// Table pour les annonces vendues
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS sold_ads (
			id SERIAL PRIMARY KEY,
			ad_id INTEGER UNIQUE NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			sold_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			sale_price DECIMAL(10, 2),
			buyer_contact VARCHAR(255),
			notes TEXT
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table des annonces vendues : %s", err)
	}

	// Ajouter la colonne is_sold à la table ads si elle n'existe pas
	_, err = DB.Exec(`
		DO $$ 
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='ads' AND column_name='is_sold') THEN
				ALTER TABLE ads ADD COLUMN is_sold BOOLEAN DEFAULT FALSE;
			END IF;
		END $$;
	`)
	if err != nil {
		log.Fatalf("Impossible d'ajouter la colonne is_sold à la table des annonces : %s", err)
	}

	// Index pour améliorer les performances
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_sold_ads_ad_id ON sold_ads(ad_id);
		CREATE INDEX IF NOT EXISTS idx_sold_ads_user_id ON sold_ads(user_id);
		CREATE INDEX IF NOT EXISTS idx_ads_is_sold ON ads(is_sold);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour les annonces vendues : %s", err)
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS notification_preferences (
			user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			notifications_enabled BOOLEAN DEFAULT true,
			email_notifications BOOLEAN DEFAULT true,
			push_notifications BOOLEAN DEFAULT true,
			message_notifications BOOLEAN DEFAULT true,
			ad_notifications BOOLEAN DEFAULT true,
			favorite_notifications BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table notification_preferences : %s", err)
	}

	// Table des notifications
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			title VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			data JSONB,
			is_read BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table notifications : %s", err)
	}

	// Index pour améliorer les performances
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
		CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);
		CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour les notifications : %s", err)
	}

	// ============================================
	// TABLE DES TOKENS DE NOTIFICATION PUSH
	// ============================================
	log.Println("Création de la table device_tokens...")
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS device_tokens (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT NOT NULL UNIQUE,
			device_type VARCHAR(20) NOT NULL CHECK (device_type IN ('android', 'ios', 'web')),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table device_tokens : %s", err)
	}

	// Index pour améliorer les performances
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id ON device_tokens(user_id);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour device_tokens : %s", err)
	}

	// Commentaire sur la table
	_, err = DB.Exec(`
		COMMENT ON TABLE device_tokens IS 'Stocke les tokens des appareils (FCM, APNS) pour les notifications push';
		COMMENT ON COLUMN device_tokens.token IS 'Le token unique du device';
		COMMENT ON COLUMN device_tokens.device_type IS 'Type d''appareil: android, ios, ou web';
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'ajouter les commentaires à la table device_tokens : %s", err)
	}

	_, err = DB.Exec(`
		CREATE OR REPLACE FUNCTION update_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la fonction update_updated_at() : %s", err)
	}

	// Table des tickets de support
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS support_tickets (
			id SERIAL PRIMARY KEY,
			user_name VARCHAR(100) NOT NULL,
			user_email VARCHAR(100) NOT NULL,
			message TEXT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'Nouveau',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table support_tickets : %s", err)
	}

	// Table des informations de contact/support du site
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS support_contact (
			id SERIAL PRIMARY KEY,
			support_email VARCHAR(255),
			contact_phone VARCHAR(50),
			whatsapp_number VARCHAR(50),
			facebook_url VARCHAR(255),
			instagram_url VARCHAR(255),
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table support_contact : %s", err)
	}

	// Table des offres de boost
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS boost_offers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			duration_days INTEGER NOT NULL,
			price DECIMAL(10, 2) NOT NULL,
			position_priority INTEGER DEFAULT 1,
			features JSONB,
			color VARCHAR(50),
			is_active BOOLEAN DEFAULT true,
			display_order INTEGER DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table boost_offers : %s", err)
	}

	// Table pour enregistrer les boosts actifs sur les annonces
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS ad_boosts (
			id SERIAL PRIMARY KEY,
			ad_id INTEGER NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
			boost_offer_id INTEGER NOT NULL REFERENCES boost_offers(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			start_date TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			end_date TIMESTAMP WITH TIME ZONE NOT NULL,
			is_active BOOLEAN DEFAULT true,
			payment_status VARCHAR(50) DEFAULT 'pending',
			payment_method VARCHAR(50),
			transaction_id VARCHAR(255),
			amount_paid DECIMAL(10, 2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table ad_boosts : %s", err)
	}

	// Index pour améliorer les performances des requêtes de boost
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_boost_offers_is_active ON boost_offers(is_active);
		CREATE INDEX IF NOT EXISTS idx_boost_offers_display_order ON boost_offers(display_order);
		CREATE INDEX IF NOT EXISTS idx_ad_boosts_ad_id ON ad_boosts(ad_id);
		CREATE INDEX IF NOT EXISTS idx_ad_boosts_user_id ON ad_boosts(user_id);
		CREATE INDEX IF NOT EXISTS idx_ad_boosts_is_active ON ad_boosts(is_active);
		CREATE INDEX IF NOT EXISTS idx_ad_boosts_end_date ON ad_boosts(end_date);
		CREATE INDEX IF NOT EXISTS idx_ad_boosts_active_dates ON ad_boosts(is_active, start_date, end_date);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour les boosts : %s", err)
	}

	// Ajouter une colonne is_boosted à la table ads
	_, err = DB.Exec(`
		DO $$ 
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='ads' AND column_name='is_boosted') THEN
				ALTER TABLE ads ADD COLUMN is_boosted BOOLEAN DEFAULT FALSE;
			END IF;
		END $$;
	`)
	if err != nil {
		log.Fatalf("Impossible d'ajouter la colonne is_boosted à la table ads : %s", err)
	}

	// Index pour la colonne is_boosted
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_ads_is_boosted ON ads(is_boosted);
		CREATE INDEX IF NOT EXISTS idx_ads_boosted_validated ON ads(is_boosted, is_validated) WHERE is_validated = true AND is_boosted = true;
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour is_boosted : %s", err)
	}

	// Ajouter la colonne payment_provider à ad_boosts
	_, err = DB.Exec(`
		DO $$ 
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='ad_boosts' AND column_name='payment_provider') THEN
				ALTER TABLE ad_boosts ADD COLUMN payment_provider VARCHAR(50) DEFAULT 'kkiapay';
			END IF;
		END $$;
	`)
	if err != nil {
		log.Fatalf("Impossible d'ajouter la colonne payment_provider à ad_boosts : %s", err)
	}

	// Ajouter la contrainte unique sur transaction_id
	_, err = DB.Exec(`
		DO $$ 
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint 
				WHERE conname = 'unique_transaction_id'
			) THEN
				ALTER TABLE ad_boosts ADD CONSTRAINT unique_transaction_id UNIQUE (transaction_id);
			END IF;
		END $$;
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'ajouter la contrainte unique sur transaction_id : %s", err)
	}

	// TABLE POUR LES LOGS DE TRANSACTIONS KKIAPAY
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS kkiapay_transactions (
			id SERIAL PRIMARY KEY,
			transaction_id VARCHAR(255) UNIQUE NOT NULL,
			boost_id INTEGER REFERENCES ad_boosts(id) ON DELETE SET NULL,
			ad_id INTEGER REFERENCES ads(id) ON DELETE CASCADE,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			amount DECIMAL(10, 2) NOT NULL,
			status VARCHAR(50) NOT NULL,
			state VARCHAR(50),
			raw_response JSONB,
			verified_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table kkiapay_transactions : %s", err)
	}

	// Créer la fonction de mise à jour automatique du timestamp
	_, err = DB.Exec(`
		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ language 'plpgsql';
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer la fonction update_updated_at_column : %s", err)
	}

	// Créer les triggers pour updated_at
	_, err = DB.Exec(`
		DROP TRIGGER IF EXISTS update_kkiapay_transactions_updated_at ON kkiapay_transactions;
		CREATE TRIGGER update_kkiapay_transactions_updated_at
		BEFORE UPDATE ON kkiapay_transactions
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour kkiapay_transactions : %s", err)
	}

	// Index pour améliorer les performances
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_ad_boosts_transaction_id ON ad_boosts(transaction_id);
		CREATE INDEX IF NOT EXISTS idx_kkiapay_transactions_user_id ON kkiapay_transactions(user_id);
		CREATE INDEX IF NOT EXISTS idx_kkiapay_transactions_ad_id ON kkiapay_transactions(ad_id);
		CREATE INDEX IF NOT EXISTS idx_kkiapay_transactions_status ON kkiapay_transactions(status);
		CREATE INDEX IF NOT EXISTS idx_kkiapay_transactions_boost_id ON kkiapay_transactions(boost_id);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour kkiapay_transactions : %s", err)
	}

	// Créer une vue pour les statistiques des paiements
	_, err = DB.Exec(`
		CREATE OR REPLACE VIEW boost_payment_stats AS
		SELECT 
			DATE_TRUNC('day', created_at) as payment_date,
			payment_provider,
			COUNT(*) as total_transactions,
			SUM(amount_paid) as total_amount,
			COUNT(CASE WHEN payment_status = 'completed' THEN 1 END) as successful_payments
		FROM ad_boosts
		WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY DATE_TRUNC('day', created_at), payment_provider
		ORDER BY payment_date DESC;
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer la vue boost_payment_stats : %s", err)
	}

	// Table des administrateurs
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS admins (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			first_name VARCHAR(100) NOT NULL,
			last_name VARCHAR(100) NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'moderateur')),
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP WITH TIME ZONE,
			created_by INTEGER REFERENCES admins(id) ON DELETE SET NULL,
			
			CONSTRAINT valid_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table admins : %s", err)
	}

	// Index pour améliorer les performances de la table admins
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_admins_email ON admins(email);
		CREATE INDEX IF NOT EXISTS idx_admins_role ON admins(role);
		CREATE INDEX IF NOT EXISTS idx_admins_is_active ON admins(is_active);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour admins : %s", err)
	}

	// Trigger pour mettre à jour automatiquement updated_at sur la table admins
	_, err = DB.Exec(`
		CREATE OR REPLACE FUNCTION update_admins_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		DROP TRIGGER IF EXISTS trigger_update_admins_updated_at ON admins;
		CREATE TRIGGER trigger_update_admins_updated_at
			BEFORE UPDATE ON admins
			FOR EACH ROW
			EXECUTE FUNCTION update_admins_updated_at();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour admins : %s", err)
	}

	// Commentaires sur la table admins
	_, err = DB.Exec(`
		COMMENT ON TABLE admins IS 'Table des administrateurs et modérateurs du système';
		COMMENT ON COLUMN admins.role IS 'Rôle de l''utilisateur admin: admin (tous les droits) ou moderateur (droits limités)';
		COMMENT ON COLUMN admins.is_active IS 'Indique si le compte admin est actif. Les comptes désactivés ne peuvent pas se connecter';
		COMMENT ON COLUMN admins.created_by IS 'ID de l''admin qui a créé ce compte';
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'ajouter les commentaires à la table admins : %s", err)
	}

	// Table des termes et conditions
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS terms_and_conditions (
			id SERIAL PRIMARY KEY,
			version VARCHAR(50) NOT NULL UNIQUE,
			title VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			effective_date TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table terms_and_conditions : %s", err)
	}

	// Index pour améliorer les performances
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_terms_is_active ON terms_and_conditions(is_active);
		CREATE INDEX IF NOT EXISTS idx_terms_version ON terms_and_conditions(version);
		CREATE INDEX IF NOT EXISTS idx_terms_effective_date ON terms_and_conditions(effective_date DESC);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour les termes et conditions : %s", err)
	}

	// Trigger pour s'assurer qu'une seule version est active à la fois
	_, err = DB.Exec(`
		CREATE OR REPLACE FUNCTION ensure_single_active_terms()
		RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.is_active = TRUE THEN
				UPDATE terms_and_conditions 
				SET is_active = FALSE 
				WHERE id != NEW.id AND is_active = TRUE;
			END IF;
			RETURN NEW;
		END;
		$$ language 'plpgsql';
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer la fonction ensure_single_active_terms : %s", err)
	}

	_, err = DB.Exec(`
		DROP TRIGGER IF EXISTS trigger_ensure_single_active_terms ON terms_and_conditions;
		CREATE TRIGGER trigger_ensure_single_active_terms
		BEFORE INSERT OR UPDATE ON terms_and_conditions
		FOR EACH ROW
		WHEN (NEW.is_active = TRUE)
		EXECUTE FUNCTION ensure_single_active_terms();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour les termes actifs : %s", err)
	}

	// Trigger pour mettre à jour automatiquement updated_at
	_, err = DB.Exec(`
		DROP TRIGGER IF EXISTS update_terms_updated_at ON terms_and_conditions;
		CREATE TRIGGER update_terms_updated_at
		BEFORE UPDATE ON terms_and_conditions
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour updated_at des termes : %s", err)
	}

	// Insertion d'une version initiale des termes et conditions
	_, err = DB.Exec(`
		INSERT INTO terms_and_conditions (version, title, content, is_active, effective_date)
		VALUES (
			'1.0.0',
			'Conditions Générales d''Utilisation',
			'# Introduction

Bienvenue sur Kivendi. En utilisant notre plateforme, vous acceptez les présentes conditions générales d''utilisation. Veuillez les lire attentivement avant d''utiliser nos services.

# Utilisation du Service

Vous vous engagez à utiliser Kivendi de manière responsable et conformément aux lois en vigueur. Vous ne devez pas utiliser notre plateforme pour des activités illégales, frauduleuses ou nuisibles.

# Compte Utilisateur

Vous êtes responsable de maintenir la confidentialité de votre compte et de votre mot de passe. Vous acceptez de nous informer immédiatement de toute utilisation non autorisée de votre compte.

# Publication d''Annonces

Les annonces publiées doivent être véridiques et conformes à la réalité. Vous êtes responsable du contenu que vous publiez. Nous nous réservons le droit de supprimer toute annonce qui viole nos politiques.

# Propriété Intellectuelle

Tout le contenu présent sur Kivendi, y compris les textes, graphiques, logos et logiciels, est la propriété de Kivendi ou de ses fournisseurs de contenu et est protégé par les lois sur la propriété intellectuelle.

# Protection des Données

Nous nous engageons à protéger vos données personnelles. Pour plus d''informations sur la manière dont nous collectons et utilisons vos données, veuillez consulter notre Politique de Confidentialité.

# Transactions

Kivendi facilite la mise en relation entre acheteurs et vendeurs. Nous ne sommes pas partie aux transactions et ne pouvons être tenus responsables des litiges entre utilisateurs.

# Limitation de Responsabilité

Kivendi ne peut être tenu responsable des dommages directs ou indirects résultant de l''utilisation ou de l''impossibilité d''utiliser notre plateforme. Nous nous réservons le droit de modifier ou d''interrompre le service à tout moment.

# Modifications des Conditions

Nous nous réservons le droit de modifier ces conditions à tout moment. Les modifications prendront effet dès leur publication sur la plateforme. Votre utilisation continue de Kivendi constitue votre acceptation des nouvelles conditions.

# Résiliation

Nous nous réservons le droit de suspendre ou de résilier votre compte à tout moment, sans préavis, en cas de violation de ces conditions ou pour toute autre raison que nous jugeons nécessaire.

# Droit Applicable

Ces conditions générales sont régies par les lois en vigueur au Bénin. Tout litige relatif à ces conditions sera soumis à la juridiction exclusive des tribunaux du Bénin.

# Contact

Si vous avez des questions concernant ces conditions, n''hésitez pas à nous contacter à support@kivendi.com',
			true,
			CURRENT_TIMESTAMP
		)
		ON CONFLICT (version) DO NOTHING;
	`)
	if err != nil {
		log.Printf("Attention: Impossible d''insérer la version initiale des termes : %s", err)
	}

	// ========================================
	// TABLE ABOUT_PAGES (PAGES "À PROPOS")
	// ========================================
	log.Println("Création de la table about_pages...")

	// Table des pages "À propos"
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS about_pages (
			id SERIAL PRIMARY KEY,
			
			-- Informations principales
			title VARCHAR(255) NOT NULL,
			subtitle VARCHAR(255),
			content TEXT NOT NULL,
			
			-- Section "Notre Mission"
			mission_title VARCHAR(255),
			mission_content TEXT,
			
			-- Section "Notre Vision"
			vision_title VARCHAR(255),
			vision_content TEXT,
			
			-- Section "Nos Valeurs"
			values_title VARCHAR(255),
			values_content TEXT,
			
			-- Section "Notre Équipe" (optionnel)
			team_title VARCHAR(255),
			team_content TEXT,
			
			-- Images/Médias
			hero_image_url TEXT,
			mission_image_url TEXT,
			vision_image_url TEXT,
			
			-- Métadonnées
			is_active BOOLEAN DEFAULT FALSE,
			created_by INTEGER REFERENCES admins(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table about_pages : %s", err)
	}

	// Index pour optimiser les requêtes
	_, err = DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_about_pages_active ON about_pages(is_active);
		CREATE INDEX IF NOT EXISTS idx_about_pages_created_at ON about_pages(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_about_pages_created_by ON about_pages(created_by);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour about_pages : %s", err)
	}

	// Trigger pour s'assurer qu'une seule page est active à la fois
	_, err = DB.Exec(`
		CREATE OR REPLACE FUNCTION ensure_single_active_about_page()
		RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.is_active = TRUE THEN
				UPDATE about_pages 
				SET is_active = FALSE 
				WHERE id != NEW.id AND is_active = TRUE;
			END IF;
			RETURN NEW;
		END;
		$$ language 'plpgsql';
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer la fonction ensure_single_active_about_page : %s", err)
	}

	_, err = DB.Exec(`
		DROP TRIGGER IF EXISTS trigger_ensure_single_active_about_page ON about_pages;
		CREATE TRIGGER trigger_ensure_single_active_about_page
		BEFORE INSERT OR UPDATE ON about_pages
		FOR EACH ROW
		WHEN (NEW.is_active = TRUE)
		EXECUTE FUNCTION ensure_single_active_about_page();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour les pages À propos actives : %s", err)
	}

	// Trigger pour mettre à jour automatiquement updated_at
	_, err = DB.Exec(`
		DROP TRIGGER IF EXISTS update_about_pages_updated_at ON about_pages;
		CREATE TRIGGER update_about_pages_updated_at
		BEFORE UPDATE ON about_pages
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer le trigger pour updated_at des pages À propos : %s", err)
	}

	// Commentaires pour la documentation
	_, err = DB.Exec(`
		COMMENT ON TABLE about_pages IS 'Table pour stocker les différentes versions de la page "À propos"';
		COMMENT ON COLUMN about_pages.is_active IS 'Une seule page peut être active à la fois';
		COMMENT ON COLUMN about_pages.content IS 'Contenu principal de la page "À propos"';
		COMMENT ON COLUMN about_pages.hero_image_url IS 'Image principale affichée en haut de la page';
		COMMENT ON COLUMN about_pages.mission_content IS 'Contenu de la section Mission';
		COMMENT ON COLUMN about_pages.vision_content IS 'Contenu de la section Vision';
		COMMENT ON COLUMN about_pages.values_content IS 'Contenu de la section Valeurs';
		COMMENT ON COLUMN about_pages.team_content IS 'Contenu de la section Équipe';
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'ajouter les commentaires à la table about_pages : %s", err)
	}

	// Insertion d'une page "À propos" initiale
	_, err = DB.Exec(`
		INSERT INTO about_pages (
			title, 
			subtitle,
			content,
			mission_title,
			mission_content,
			vision_title,
			vision_content,
			values_title,
			values_content,
			is_active
		) VALUES (
			'À propos de Kivendi',
			'Découvrez qui nous sommes et ce qui nous motive',
			'Bienvenue sur Kivendi, votre plateforme de petites annonces de confiance. Nous connectons acheteurs et vendeurs dans un environnement sécurisé et convivial, facilitant les échanges au quotidien.

Notre plateforme a été créée avec une vision simple : rendre les transactions entre particuliers et professionnels plus simples, plus sûres et plus accessibles pour tous. Que vous cherchiez à vendre un objet, à acheter un service ou à dénicher une bonne affaire, Kivendi est là pour vous accompagner.',
			'Notre Mission',
			'Faciliter les échanges entre particuliers et professionnels en offrant une plateforme simple, sûre et accessible à tous. Nous nous engageons à créer un espace de confiance où chacun peut acheter et vendre en toute sérénité.',
			'Notre Vision',
			'Devenir la plateforme de référence pour les petites annonces dans toute la région, en privilégiant la confiance, la transparence et l''innovation. Nous aspirons à créer une communauté dynamique où les transactions sont fluides et sécurisées.',
			'Nos Valeurs',
			'🔒 Confiance : Nous créons un environnement sécurisé pour tous nos utilisateurs

✨ Simplicité : Une plateforme facile à utiliser pour tous, quel que soit votre niveau technique

🤝 Transparence : Des règles claires et une communication honnête avec notre communauté

🚀 Innovation : Nous améliorons constamment notre service pour répondre à vos besoins

💚 Respect : Nous valorisons chaque membre de notre communauté',
			TRUE
		)
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'insérer la page À propos initiale : %s", err)
	}

	log.Println("✓ Table about_pages créée avec succès")

	// ============================================
	// DÉBUT DE VOS AJOUTS
	// ============================================

	log.Println("Création de la table app_settings...")
	_, err = DB.Exec(`
		-- ============================================
		-- TABLE DES PARAMÈTRES D'APPLICATION
		-- ============================================
		CREATE TABLE IF NOT EXISTS app_settings (
			id SERIAL PRIMARY KEY,
			
			-- Informations générales de l'application
			app_name VARCHAR(255) NOT NULL DEFAULT 'Kivendi',
			app_tagline VARCHAR(500),
			app_description TEXT,
			
			-- Logos et icônes
			logo_url VARCHAR(500),
			favicon_url VARCHAR(500),
			logo_dark_url VARCHAR(500), -- Pour le mode sombre
			
			-- Informations de contact
			support_email VARCHAR(255),
			contact_phone VARCHAR(50),
			whatsapp_number VARCHAR(50),
			
			-- Réseaux sociaux
			facebook_url VARCHAR(255),
			instagram_url VARCHAR(255),
			twitter_url VARCHAR(255),
			linkedin_url VARCHAR(255),
			youtube_url VARCHAR(255),
			tiktok_url VARCHAR(255),
			
			-- Adresse physique
			physical_address TEXT,
			city VARCHAR(100),
			country VARCHAR(100) DEFAULT 'Bénin',
			
			-- Informations légales
			company_name VARCHAR(255),
			registration_number VARCHAR(100),
			tax_id VARCHAR(100),
			
			-- Paramètres SEO
			meta_title VARCHAR(255),
			meta_description TEXT,
			meta_keywords TEXT,
			
			-- Paramètres de l'application
			default_language VARCHAR(10) DEFAULT 'fr',
			currency VARCHAR(10) DEFAULT 'XOF',
			timezone VARCHAR(50) DEFAULT 'Africa/Porto-Novo',
			
			-- Paramètres de modération
			auto_validate_ads BOOLEAN DEFAULT false,
			require_phone_verification BOOLEAN DEFAULT true,
			max_images_per_ad INTEGER DEFAULT 8,
			max_ad_duration_days INTEGER DEFAULT 90,
			
			-- Paramètres d'email
			smtp_host VARCHAR(255),
			smtp_port INTEGER,
			smtp_username VARCHAR(255),
			smtp_password VARCHAR(255),
			smtp_from_email VARCHAR(255),
			smtp_from_name VARCHAR(255),
			
			-- Paramètres de paiement
			kkiapay_public_key VARCHAR(255),
			kkiapay_private_key VARCHAR(255),
			kkiapay_secret VARCHAR(255),
			payment_enabled BOOLEAN DEFAULT true,
			
			-- Métadonnées
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_by INTEGER REFERENCES admins(id)
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table app_settings : %s", err)
	}

	_, err = DB.Exec(`
		-- Index pour améliorer les performances
		CREATE INDEX IF NOT EXISTS idx_app_settings_updated_at ON app_settings(updated_at DESC);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer l'index pour app_settings : %s", err)
	}

	_, err = DB.Exec(`
		-- Insérer les paramètres par défaut si la table est vide
		INSERT INTO app_settings (
			app_name, 
			app_tagline, 
			app_description,
			support_email,
			contact_phone,
			country
		) 
		SELECT 
			'Kivendi',
			'Votre plateforme de petites annonces au Bénin',
			'Kivendi est la plateforme leader de petites annonces au Bénin. Achetez et vendez facilement !',
			'support@kivendi.com',
			'+229 XX XX XX XX',
			'Bénin'
		WHERE NOT EXISTS (SELECT 1 FROM app_settings);
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'insérer les paramètres par défaut pour app_settings : %s", err)
	}
	log.Println("✓ Table app_settings créée avec succès")

	log.Println("Création de la table maintenance_mode...")
	_, err = DB.Exec(`
		-- ============================================
		-- TABLE DE MAINTENANCE
		-- ============================================
		CREATE TABLE IF NOT EXISTS maintenance_mode (
			id SERIAL PRIMARY KEY,
			
			-- Statut de la maintenance
			is_active BOOLEAN DEFAULT false,
			
			-- Messages
			title VARCHAR(255) DEFAULT 'Maintenance en cours',
			message TEXT DEFAULT 'Notre plateforme est actuellement en maintenance. Nous serons de retour bientôt !',
			
			-- Planification
			scheduled_start TIMESTAMP WITH TIME ZONE,
			scheduled_end TIMESTAMP WITH TIME ZONE,
			estimated_duration_minutes INTEGER,
			
			-- Notifications
			notify_users BOOLEAN DEFAULT true,
			show_countdown BOOLEAN DEFAULT true,
			
			-- Accès pendant la maintenance
			allow_admin_access BOOLEAN DEFAULT true,
			allowed_ip_addresses TEXT[], -- Liste des IPs autorisées
			
			-- Contact d'urgence
			emergency_contact_email VARCHAR(255),
			emergency_contact_phone VARCHAR(50),
			
			-- Métadonnées
			reason TEXT, -- Raison de la maintenance
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			activated_by INTEGER REFERENCES admins(id),
			activated_at TIMESTAMP WITH TIME ZONE,
			deactivated_at TIMESTAMP WITH TIME ZONE
		);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer la table maintenance_mode : %s", err)
	}

	_, err = DB.Exec(`
		-- Index
		CREATE INDEX IF NOT EXISTS idx_maintenance_active ON maintenance_mode(is_active);
		CREATE INDEX IF NOT EXISTS idx_maintenance_scheduled ON maintenance_mode(scheduled_start, scheduled_end);
	`)
	if err != nil {
		log.Fatalf("Impossible de créer les index pour maintenance_mode : %s", err)
	}

	_, err = DB.Exec(`
		-- Insérer l'enregistrement par défaut (maintenance désactivée)
		INSERT INTO maintenance_mode (
			is_active,
			title,
			message
		)
		SELECT 
			false,
			'Maintenance en cours',
			'Notre plateforme est actuellement en maintenance. Nous serons de retour bientôt !'
		WHERE NOT EXISTS (SELECT 1 FROM maintenance_mode);
	`)
	if err != nil {
		log.Printf("Attention: Impossible d'insérer l'enregistrement par défaut pour maintenance_mode : %s", err)
	}
	log.Println("✓ Table maintenance_mode créée avec succès")

	_, err = DB.Exec(`
		-- ============================================
		-- FONCTION POUR VÉRIFIER SI L'APP EST EN MAINTENANCE
		-- ============================================
		CREATE OR REPLACE FUNCTION is_app_in_maintenance()
		RETURNS BOOLEAN AS $$
		DECLARE
			maintenance_active BOOLEAN;
		BEGIN
			SELECT is_active INTO maintenance_active
			FROM maintenance_mode
			ORDER BY id DESC
			LIMIT 1;
			
			RETURN COALESCE(maintenance_active, false);
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer la fonction is_app_in_maintenance : %s", err)
	}

	_, err = DB.Exec(`
		-- ============================================
		-- TRIGGER POUR METTRE À JOUR updated_at
		-- ============================================
		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		log.Printf("Attention: Impossible de (re)créer la fonction update_updated_at_column : %s", err)
	}

	_, err = DB.Exec(`
		CREATE TRIGGER update_app_settings_updated_at
			BEFORE UPDATE ON app_settings
			FOR EACH ROW
			EXECUTE FUNCTION update_updated_at_column();

		CREATE TRIGGER update_maintenance_mode_updated_at
			BEFORE UPDATE ON maintenance_mode
			FOR EACH ROW
			EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		log.Printf("Attention: Impossible de créer les triggers pour app_settings ou maintenance_mode : %s", err)
	}

}
