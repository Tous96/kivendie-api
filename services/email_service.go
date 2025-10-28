package services

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"os"
)

// EmailConfig contient la configuration SMTP
type EmailConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// GetEmailConfig récupère la configuration depuis les variables d'environnement
func GetEmailConfig() *EmailConfig {
	return &EmailConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASS"),
		From:     os.Getenv("SMTP_FROM"),
	}
}

// SendVerificationEmail envoie un email de vérification avec le code
func SendVerificationEmail(email, code string) error {
	config := GetEmailConfig()

	// Validation de la configuration
	if config.Host == "" || config.Port == "" || config.User == "" || config.Password == "" {
		log.Println("[ERREUR] Configuration SMTP incomplète")
		return fmt.Errorf("configuration SMTP incomplète")
	}

	log.Printf("[INFO] Envoi d'email de VÉRIFICATION à %s via %s:%s", email, config.Host, config.Port)

	subject := "Votre code de vérification Kivendi"
	// Utilisation du nouveau template HTML simplifié
	htmlBody := generateVerificationEmailHTML(code)

	// Construire le message
	message := buildEmailMessage(config.From, email, subject, htmlBody)

	// Envoyer l'email selon le port
	var err error
	if config.Port == "465" {
		// Port 465 : SSL/TLS direct
		err = sendEmailWithSSL(config, email, message)
	} else {
		// Port 587 : STARTTLS
		err = sendEmailWithSTARTTLS(config, email, message)
	}

	if err != nil {
		log.Printf("[ERREUR] Erreur lors de l'envoi de l'email de VÉRIFICATION à %s: %v", email, err)
		return err
	}

	log.Printf("[OK] Email de VÉRIFICATION envoyé avec succès à %s", email)
	return nil
}

// SendPasswordResetEmail envoie un email avec le lien de réinitialisation.
// (Ceci est la version corrigée qui envoie réellement l'email)
func SendPasswordResetEmail(toEmail string, token string) error {
	config := GetEmailConfig()

	// Validation de la configuration
	if config.Host == "" || config.Port == "" || config.User == "" || config.Password == "" {
		log.Println("[ERREUR] Configuration SMTP incomplète")
		return fmt.Errorf("configuration SMTP incomplète")
	}

	// IMPORTANT: Récupérer l'URL du frontend depuis les variables d'environnement
	// Assurez-vous de définir FRONTEND_URL dans votre environnement (ex: "https://votre-site.com")
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		log.Println("[ERREUR] Variable d'environnement FRONTEND_URL non définie")
		frontendURL = "https://api.kivendie.com/api/v1" // Fallback pour le développement local
	}

	// Construisez le lien de réinitialisation complet
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, token)

	subject := "Réinitialisation de votre mot de passe Kivendi"
	htmlBody := generatePasswordResetEmailHTML(resetLink)

	log.Printf("[INFO] Envoi d'email de RESET MOT DE PASSE à %s via %s:%s", toEmail, config.Host, config.Port)

	// Construire le message
	message := buildEmailMessage(config.From, toEmail, subject, htmlBody)

	// Envoyer l'email selon le port
	var err error
	if config.Port == "465" {
		// Port 465 : SSL/TLS direct
		err = sendEmailWithSSL(config, toEmail, message)
	} else {
		// Port 587 : STARTTLS
		err = sendEmailWithSTARTTLS(config, toEmail, message)
	}

	if err != nil {
		log.Printf("[ERREUR] Erreur lors de l'envoi de l'email de RESET MOT DE PASSE à %s: %v", toEmail, err)
		return err
	}

	log.Printf("[OK] Email de RESET MOT DE PASSE envoyé avec succès à %s", toEmail)
	return nil
}

// sendEmailWithSSL envoie un email via SSL/TLS direct (port 465)
func sendEmailWithSSL(config *EmailConfig, to string, message []byte) error {
	serverAddr := fmt.Sprintf("%s:%s", config.Host, config.Port)

	log.Printf("[INFO] Connexion SSL/TLS directe à %s", serverAddr)

	// Configuration TLS
	tlsConfig := &tls.Config{
		ServerName:         config.Host,
		InsecureSkipVerify: false, // Mettre à true si problème de certificat
	}

	// Connexion TLS directe
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		log.Printf("[ERREUR] Erreur de connexion TLS: %v", err)
		return fmt.Errorf("impossible de se connecter au serveur SMTP: %v", err)
	}
	defer conn.Close()

	log.Println("[OK] Connexion TLS établie")

	// Créer un client SMTP à partir de la connexion TLS
	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		log.Printf("[ERREUR] Erreur création client SMTP: %v", err)
		return fmt.Errorf("impossible de créer le client SMTP: %v", err)
	}
	defer client.Close()

	log.Println("[OK] Client SMTP créé")

	// Authentification
	auth := smtp.PlainAuth("", config.User, config.Password, config.Host)
	if err = client.Auth(auth); err != nil {
		log.Printf("[ERREUR] Erreur d'authentification: %v", err)
		return fmt.Errorf("erreur d'authentification: %v", err)
	}

	log.Println("[OK] Authentification réussie")

	// Définir l'expéditeur
	if err = client.Mail(config.From); err != nil {
		log.Printf("[ERREUR] Erreur MAIL FROM: %v", err)
		return fmt.Errorf("erreur MAIL FROM: %v", err)
	}

	log.Printf("[OK] MAIL FROM: %s", config.From)

	// Définir le destinataire
	if err = client.Rcpt(to); err != nil {
		log.Printf("[ERREUR] Erreur RCPT TO: %v", err)
		return fmt.Errorf("erreur RCPT TO: %v", err)
	}

	log.Printf("[OK] RCPT TO: %s", to)

	// Envoyer le message
	w, err := client.Data()
	if err != nil {
		log.Printf("[ERREUR] Erreur DATA: %v", err)
		return fmt.Errorf("erreur DATA: %v", err)
	}

	_, err = w.Write(message)
	if err != nil {
		log.Printf("[ERREUR] Erreur écriture message: %v", err)
		return fmt.Errorf("erreur écriture message: %v", err)
	}

	err = w.Close()
	if err != nil {
		log.Printf("[ERREUR] Erreur fermeture writer: %v", err)
		return fmt.Errorf("erreur fermeture writer: %v", err)
	}

	log.Println("[OK] Message envoyé")

	// Quit
	err = client.Quit()
	if err != nil {
		log.Printf("[WARN] Erreur QUIT (non bloquant): %v", err)
		// Ne pas retourner d'erreur ici car le message est déjà envoyé
	}

	return nil
}

// sendEmailWithSTARTTLS envoie un email via STARTTLS (port 587)
func sendEmailWithSTARTTLS(config *EmailConfig, to string, message []byte) error {
	serverAddr := fmt.Sprintf("%s:%s", config.Host, config.Port)

	log.Printf("[INFO] Connexion STARTTLS à %s", serverAddr)

	// Authentification
	auth := smtp.PlainAuth("", config.User, config.Password, config.Host)

	// Envoyer avec SendMail (gère automatiquement STARTTLS)
	err := smtp.SendMail(
		serverAddr,
		auth,
		config.From,
		[]string{to},
		message,
	)

	if err != nil {
		log.Printf("[ERREUR] Erreur STARTTLS: %v", err)
		return fmt.Errorf("erreur STARTTLS: %v", err)
	}

	return nil
}

// buildEmailMessage construit le message email avec headers MIME
func buildEmailMessage(from, to, subject, htmlBody string) []byte {
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"
	headers["Content-Transfer-Encoding"] = "8bit"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	return []byte(message)
}

// generateVerificationEmailHTML génère le HTML de l'email de vérification
func generateVerificationEmailHTML(code string) string {
	// HTML minimaliste pour une meilleure délivrabilité et éviter les filtres anti-spam
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Votre code de vérification</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 20px;">
    <div style="max-width: 600px; margin: 0 auto; border: 1px solid #ddd; border-radius: 8px; overflow: hidden;">
        
        <div style="padding: 25px 30px; background-color: #f9f9f9;">
            <h1 style="font-size: 24px; color: #111; margin: 0 0 20px 0;">Vérification de compte Kivendi</h1>
        </div>
        
        <div style="padding: 30px;">
            <p style="margin: 0 0 20px 0; font-size: 16px;">
                Bonjour,
            </p>
            
            <p style="color: #555; margin: 0 0 25px 0; font-size: 16px;">
                Merci de vous être inscrit. Pour finaliser votre inscription, veuillez utiliser le code de vérification ci-dessous :
            </p>
            
            <div style="background-color: #f4f4f4; padding: 20px; border-radius: 8px; text-align: center; margin: 30px 0;">
                <p style="margin: 0 0 10px 0; font-size: 14px; color: #666;">
                    Votre code de vérification
                </p>
                <p style="margin: 0; font-size: 32px; font-weight: 700; color: #000; letter-spacing: 8px; font-family: 'Courier New', monospace;">
                    %s
                </p>
            </div>
            
            <p style="color: #777; font-size: 15px; margin: 25px 0;">
                Ce code expirera dans 24 heures.
            </p>
            
            <p style="color: #777; margin: 20px 0 0 0; font-size: 14px; line-height: 1.6;">
                Si vous n'avez pas demandé ce code, vous pouvez ignorer cet email en toute sécurité.
            </p>
        </div>
        
        <div style="background-color: #f9f9f9; padding: 25px 30px; text-align: center; border-top: 1px solid #eee;">
            <p style="margin: 0; color: #aaa; font-size: 12px;">
                © 2025 Kivendi. Tous droits réservés.
            </p>
        </div>
    </div>
</body>
</html>`, code)
}

// --- NOUVELLE FONCTION ---
// generatePasswordResetEmailHTML génère le HTML pour l'email de réinitialisation de mot de passe
func generatePasswordResetEmailHTML(resetLink string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Réinitialisation de mot de passe</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 20px;">
    <div style="max-width: 600px; margin: 0 auto; border: 1px solid #ddd; border-radius: 8px; overflow: hidden;">
        
        <div style="padding: 25px 30px; background-color: #f9f9f9;">
            <h1 style="font-size: 24px; color: #111; margin: 0 0 20px 0;">Réinitialisation de mot de passe Kivendi</h1>
        </div>
        
        <div style="padding: 30px;">
            <p style="margin: 0 0 20px 0; font-size: 16px;">
                Bonjour,
            </p>
            
            <p style="color: #555; margin: 0 0 25px 0; font-size: 16px;">
                Vous avez demandé une réinitialisation de mot de passe pour votre compte Kivendi.
            </p>
            
            <p style="color: #555; margin: 0 0 25px 0; font-size: 16px;">
                Cliquez sur le bouton ci-dessous pour choisir un nouveau mot de passe :
            </p>
            
            <div style="text-align: center; margin: 30px 0;">
                <a href="%s" style="background-color: #007bff; color: white; padding: 14px 22px; text-decoration: none; border-radius: 8px; display: inline-block; font-size: 16px; font-weight: bold;">
                    Réinitialiser mon mot de passe
                </a>
            </div>
            
            <p style="color: #777; font-size: 15px; margin: 25px 0;">
                Ce lien expirera dans 1 heure.
            </p>
            
            <p style="color: #777; margin: 20px 0 0 0; font-size: 14px; line-height: 1.6;">
                Si vous n'avez pas demandé cette réinitialisation, veuillez ignorer cet email.
            </p>
        </div>
        
        <div style="background-color: #f9f9f9; padding: 25px 30px; text-align: center; border-top: 1px solid #eee;">
            <p style="margin: 0; color: #aaa; font-size: 12px;">
                © 2025 Kivendi. Tous droits réservés.
            </p>
        </div>
    </div>
</body>
</html>`, resetLink)
}
