package handlers

import (
	"fmt"
	"net/http"
	// Vous l'utilisiez déjà
)

// PasswordResetLinkHandler gère les liens de réinitialisation de mot de passe
func PasswordResetLinkHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer le token depuis les paramètres de l'URL (ex: /reset-password?token=...)
	token := r.URL.Query().Get("token")

	// Si aucun token n'est fourni, on ne peut rien faire
	if token == "" {
		http.Error(w, "Token de réinitialisation manquant", http.StatusBadRequest)
		return
	}

	// --- URLs ---
	// IMPORTANT : Mettez l'URL de votre site web frontend ici
	webBaseURL := "https://kivendi.com"

	// URLs des stores (vous pouvez les centraliser)
	androidStoreURL := "https://play.google.com/store/apps/details?id=bj.kivendie.app"
	iosStoreURL := "https://apps.apple.com/app/idVOTRE_APP_ID"

	// Deep link pour ouvrir l'app
	appDeepLink := fmt.Sprintf("kivendi://reset-password?token=%s", token)

	// URL de fallback pour le navigateur web
	webFallbackURL := fmt.Sprintf("%s/reset-password?token=%s", webBaseURL, token)

	// --- Page HTML ---
	// Page HTML adaptée, basée sur votre deep_link_handler.go
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Réinitialisation de mot de passe</title>
    
    <script src="https://unpkg.com/lucide@latest"></script>
    
    <link href="https://fonts.googleapis.com/css2?family=Raleway:wght@400;500;600;700;800&display=swap" rel="stylesheet">
    
    <style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
        :root { --kivendi-green: #007782; --kivendi-white: #FFFFFF; }
        body {
            font-family: 'Raleway', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex; justify-content: center; align-items: center; min-height: 100vh;
            background: linear-gradient(135deg, var(--kivendi-green) 0%%, #005F6B 100%%);
            color: var(--kivendi-white); text-align: center; padding: 20px;
        }
        .container { max-width: 480px; width: 100%%; z-index: 1; }
        .logo-container {
            width: 110px; height: 110px; margin: 0 auto 32px;
            background: rgba(255, 255, 255, 0.12); backdrop-filter: blur(12px);
            border-radius: 28px; display: flex; align-items: center; justify-content: center;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15); border: 1px solid rgba(255, 255, 255, 0.18);
        }
        .logo-container svg { width: 56px; height: 56px; stroke: var(--kivendi-white); stroke-width: 2; }
        h1 { font-size: 32px; margin-bottom: 14px; font-weight: 700; }
        p { opacity: 0.95; margin-bottom: 32px; font-size: 17px; line-height: 1.6; font-weight: 500; }
        .spinner-container { margin: 44px auto; }
        .spinner {
            width: 64px; height: 64px; margin: 0 auto; border: 4px solid rgba(255, 255, 255, 0.15);
            border-radius: 50%%; border-top-color: var(--kivendi-white); animation: spin 0.9s ease-in-out infinite;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
        
        .action-buttons {
            display: none; flex-direction: column; gap: 14px; margin-top: 44px;
        }
        .action-buttons > p { font-size: 15px; margin-bottom: 4px; opacity: 0.9; font-weight: 600; }
        .store-btn {
            display: inline-flex; align-items: center; justify-content: center; gap: 14px;
            padding: 20px 32px; background: var(--kivendi-white); color: var(--kivendi-green);
            text-decoration: none; border-radius: 16px; font-weight: 700; font-size: 16px;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1); box-shadow: 0 6px 24px rgba(0, 0, 0, 0.18);
        }
        .store-btn:hover { transform: translateY(-3px); box-shadow: 0 10px 35px rgba(0, 0, 0, 0.28); }
        .store-btn svg { width: 26px; height: 26px; stroke-width: 2.5; }
        
        .web-btn {
            background: rgba(255, 255, 255, 0.12); color: var(--kivendi-white);
            backdrop-filter: blur(12px); border: 2px solid rgba(255, 255, 255, 0.2); font-weight: 600;
        }
		.web-btn.primary {
			background: var(--kivendi-white); color: var(--kivendi-green);
		}
        .web-btn svg { stroke: var(--kivendi-white); }
		.web-btn.primary svg { stroke: var(--kivendi-green); }

        .desktop-message {
            display: none; background: rgba(255, 255, 255, 0.12); backdrop-filter: blur(12px);
            padding: 36px 30px; border-radius: 24px; margin-top: 44px;
            border: 2px solid rgba(255, 255, 255, 0.2); box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);
        }
        .desktop-message h2 { font-size: 26px; margin-bottom: 18px; font-weight: 700; }
        .desktop-message > p { margin-bottom: 26px; opacity: 0.95; font-size: 16px; font-weight: 500; }
	</style>
</head>
<body>
    <div class="container">
        <div class="logo-container">
            <i data-lucide="key-round"></i> </div>
        
        <h1 id="mainTitle">Réinitialisation de mot de passe</h1>
        <p id="mainText">Tentative d'ouverture de l'application Kivendi...</p>
        
        <div class="spinner-container" id="spinnerContainer">
            <div class="spinner"></div>
        </div>
        
        <div class="action-buttons" id="mobileOptions">
            <p>L'application n'a pas pu s'ouvrir ?</p>
            
            <a href="%s" class="store-btn primary web-btn">
                <i data-lucide="globe"></i>
                <span>Continuer sur le navigateur</span>
            </a>
            
            <p style="margin-top: 20px; margin-bottom: 10px;">Ou téléchargez l'application :</p>
            
            <a href="%s" class="store-btn" id="androidBtn" style="display:none;">
                <i data-lucide="smartphone"></i>
                <span>Télécharger sur Google Play</span>
            </a>
            <a href="%s" class="store-btn" id="iosBtn" style="display:none;">
                <i data-lucide="apple"></i>
                <span>Télécharger sur App Store</span>
            </a>
        </div>
        
        <div class="desktop-message" id="desktopMessage">
            <h2>Réinitialiser sur le navigateur</h2>
            <p>Vous allez être redirigé vers le site web pour réinitialiser votre mot de passe.</p>
            <a href="%s" class="store-btn primary web-btn">
                <i data-lucide="external-link"></i>
                <span>Continuer sur le site web</span>
            </a>
        </div>
    </div>

    <script>
        const appDeepLink = '%s';
        const androidStoreURL = '%s';
        const iosStoreURL = '%s';
        const webFallbackURL = '%s';
        
        let redirectTimer;

        // Initialiser les icônes Lucide
        lucide.createIcons();

        // Détecter le type d'appareil
        const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);
        const isAndroid = /Android/.test(navigator.userAgent);
        const isMobile = isIOS || isAndroid;
        const isDesktop = !isMobile;

        console.log('Device detection:', { isIOS, isAndroid, isMobile, isDesktop });

        // Fonction pour tenter d'ouvrir l'app (mobile) ou rediriger (desktop)
        function handleRedirect() {
            if (isDesktop) {
                showDesktopMessage();
                return;
            }

            // Si mobile, tenter d'ouvrir l'app
            console.log('Attempting to open app with:', appDeepLink);
            window.location.href = appDeepLink;
            
            // Démarrer le timer de fallback
            redirectTimer = setTimeout(() => {
                if (!document.hidden) {
                    console.log('App did not open, showing mobile options');
                    showMobileOptions();
                }
            }, 2500);
        }

        // Fonction pour afficher les options sur mobile (si app non ouverte)
        function showMobileOptions() {
            document.getElementById('spinnerContainer').style.display = 'none';
            document.getElementById('mainTitle').textContent = 'Ouvrir avec...';
            document.getElementById('mainText').textContent = 'Choisissez une option pour continuer';
            
            const mobileOptions = document.getElementById('mobileOptions');
            mobileOptions.style.display = 'flex';

            if (isAndroid) {
                document.getElementById('androidBtn').style.display = 'inline-flex';
            } else if (isIOS) {
                document.getElementById('iosBtn').style.display = 'inline-flex';
            }
            
            // Réinitialiser les icônes après affichage
            lucide.createIcons();
        }

        // Fonction pour afficher le message desktop
        function showDesktopMessage() {
            document.getElementById('spinnerContainer').style.display = 'none';
            document.getElementById('mainText').textContent = 'Kivendi est une application mobile.';
            document.getElementById('desktopMessage').style.display = 'block';
            
            // Réinitialiser les icônes
            lucide.createIcons();
            
            // Redirection automatique après 5 secondes
            setTimeout(() => {
                console.log('Redirecting to web fallback:', webFallbackURL);
                window.location.href = webFallbackURL;
            }, 5000);
        }

        // --- Détecteurs pour savoir si l'app s'est ouverte ---
        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                clearTimeout(redirectTimer);
            }
        });
        window.addEventListener('blur', () => {
            clearTimeout(redirectTimer);
        });

        // Lancer la logique au chargement
        handleRedirect();
    </script>
</body>
</html>
`,
		// Ordre des variables pour Sprintf
		webFallbackURL,  // %s pour le bouton "Continuer sur le navigateur"
		androidStoreURL, // %s pour androidBtn
		iosStoreURL,     // %s pour iosBtn
		webFallbackURL,  // %s pour le bouton "Continuer sur le site web" (desktop)
		appDeepLink,     // %s pour le script
		androidStoreURL, // %s pour le script
		iosStoreURL,     // %s pour le script
		webFallbackURL,  // %s pour le script
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlContent))
}
