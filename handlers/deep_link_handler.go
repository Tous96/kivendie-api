package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// DeepLinkHandler gère les redirections pour les deep links
func DeepLinkHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	adID := vars["adID"]

	// URLs des stores
	androidStoreURL := "https://play.google.com/store/apps/details?id=bj.kivendie.app"
	iosStoreURL := "https://apps.apple.com/app/idVOTRE_APP_ID"

	// Deep link de votre application
	appDeepLink := fmt.Sprintf("kivendi://ad/%s", adID)

	// URL de fallback pour desktop
	webFallbackURL := fmt.Sprintf("https://kivendi.com/annonces/%s", adID)

	// Page HTML avec redirection intelligente
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redirection vers Kivendi</title>
    <meta name="description" content="Découvrez cette annonce sur Kivendi">
    
    <!-- Open Graph pour le partage sur les réseaux sociaux -->
    <meta property="og:title" content="Annonce sur Kivendi">
    <meta property="og:description" content="Cliquez pour voir cette annonce">
    <meta property="og:type" content="website">
    
    
    <!-- Lucide Icons -->
    <script src="https://unpkg.com/lucide@latest"></script>
    
    <!-- Google Fonts - Raleway -->
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Raleway:wght@400;500;600;700;800&display=swap" rel="stylesheet">
    
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        :root {
            --kivendi-green: #007782;
            --kivendi-green-dark: #00BFA5;
            --kivendi-white: #FFFFFF;
            --kivendi-black: #222222;
            --kivendi-surface: #1E1E1E;
        }
        
        body {
            font-family: 'Raleway', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, var(--kivendi-green) 0%%, #005F6B 100%%);
            color: var(--kivendi-white);
            text-align: center;
            padding: 20px;
            position: relative;
            overflow: hidden;
        }
        
        /* Mode sombre adaptatif */
        @media (prefers-color-scheme: dark) {
            body {
                background: linear-gradient(135deg, var(--kivendi-green-dark) 0%%, #008C7A 100%%);
            }
        }
        
        /* Effet de fond animé */
        body::before {
            content: '';
            position: absolute;
            width: 200%%;
            height: 200%%;
            background: radial-gradient(circle, rgba(255,255,255,0.08) 1px, transparent 1px);
            background-size: 40px 40px;
            animation: moveBackground 30s linear infinite;
            z-index: 0;
        }
        
        @keyframes moveBackground {
            0%% { transform: translate(0, 0); }
            100%% { transform: translate(40px, 40px); }
        }
        
        .container {
            max-width: 480px;
            width: 100%%;
            position: relative;
            z-index: 1;
        }
        
        .logo-container {
            width: 110px;
            height: 110px;
            margin: 0 auto 32px;
            background: rgba(255, 255, 255, 0.12);
            backdrop-filter: blur(12px);
            border-radius: 28px;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);
            animation: float 3s ease-in-out infinite;
            border: 1px solid rgba(255, 255, 255, 0.18);
        }
        
        @keyframes float {
            0%%, 100%% { transform: translateY(0px); }
            50%% { transform: translateY(-12px); }
        }
        
        .logo-container svg {
            width: 56px;
            height: 56px;
            stroke: var(--kivendi-white);
            stroke-width: 2;
        }
        
        h1 {
            font-size: 32px;
            margin-bottom: 14px;
            font-weight: 700;
            letter-spacing: -0.5px;
            line-height: 1.2;
        }
        
        p {
            opacity: 0.95;
            margin-bottom: 32px;
            font-size: 17px;
            line-height: 1.6;
            font-weight: 500;
        }
        
        .spinner-container {
            margin: 44px auto;
        }
        
        .spinner {
            width: 64px;
            height: 64px;
            margin: 0 auto;
            border: 4px solid rgba(255, 255, 255, 0.15);
            border-radius: 50%%;
            border-top-color: var(--kivendi-white);
            animation: spin 0.9s ease-in-out infinite;
        }
        
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        
        .store-buttons {
            display: none;
            flex-direction: column;
            gap: 14px;
            margin-top: 44px;
        }
        
        .store-buttons > p {
            font-size: 15px;
            margin-bottom: 4px;
            opacity: 0.9;
            font-weight: 600;
        }
        
        .store-btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 14px;
            padding: 20px 32px;
            background: var(--kivendi-white);
            color: var(--kivendi-green);
            text-decoration: none;
            border-radius: 16px;
            font-weight: 700;
            font-size: 16px;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            box-shadow: 0 6px 24px rgba(0, 0, 0, 0.18);
            position: relative;
            overflow: hidden;
            border: 2px solid transparent;
        }
        
        .store-btn::before {
            content: '';
            position: absolute;
            top: 0;
            left: -100%%;
            width: 100%%;
            height: 100%%;
            background: linear-gradient(90deg, transparent, rgba(255,255,255,0.4), transparent);
            transition: left 0.5s;
        }
        
        .store-btn:hover::before {
            left: 100%%;
        }
        
        .store-btn:hover {
            transform: translateY(-3px);
            box-shadow: 0 10px 35px rgba(0, 0, 0, 0.28);
        }
        
        .store-btn:active {
            transform: translateY(-1px);
        }
        
        .store-btn svg {
            width: 26px;
            height: 26px;
            stroke-width: 2.5;
        }
        
        .web-btn {
            background: rgba(255, 255, 255, 0.12);
            color: var(--kivendi-white);
            backdrop-filter: blur(12px);
            border: 2px solid rgba(255, 255, 255, 0.2);
            font-weight: 600;
        }
        
        .web-btn svg {
            stroke: var(--kivendi-white);
        }
        
        .desktop-message {
            display: none;
            background: rgba(255, 255, 255, 0.12);
            backdrop-filter: blur(12px);
            padding: 36px 30px;
            border-radius: 24px;
            margin-top: 44px;
            border: 2px solid rgba(255, 255, 255, 0.2);
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);
        }
        
        .desktop-message h2 {
            font-size: 26px;
            margin-bottom: 18px;
            font-weight: 700;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 14px;
            line-height: 1.3;
        }
        
        .desktop-message h2 svg {
            width: 32px;
            height: 32px;
        }
        
        .desktop-message > p {
            margin-bottom: 26px;
            opacity: 0.95;
            font-size: 16px;
            font-weight: 500;
        }
        
        .feature-list {
            text-align: left;
            margin: 26px 0;
            padding: 0;
        }
        
        .feature-item {
            display: flex;
            align-items: center;
            gap: 14px;
            margin-bottom: 14px;
            font-size: 15px;
            opacity: 0.92;
            font-weight: 500;
        }
        
        .feature-item svg {
            width: 22px;
            height: 22px;
            flex-shrink: 0;
            stroke-width: 2.5;
        }
        
        /* RESPONSIVE DESIGN */
        
        /* Tablets */
        @media (max-width: 768px) {
            body {
                padding: 16px;
            }
            
            .logo-container {
                width: 100px;
                height: 100px;
                margin-bottom: 28px;
            }
            
            .logo-container svg {
                width: 50px;
                height: 50px;
            }
            
            h1 {
                font-size: 28px;
                margin-bottom: 12px;
            }
            
            p {
                font-size: 16px;
                margin-bottom: 28px;
            }
            
            .spinner {
                width: 56px;
                height: 56px;
            }
            
            .store-btn {
                padding: 18px 28px;
                font-size: 15px;
            }
            
            .desktop-message {
                padding: 30px 24px;
            }
            
            .desktop-message h2 {
                font-size: 23px;
            }
        }
        
        /* Mobile devices */
        @media (max-width: 480px) {
            body {
                padding: 12px;
            }
            
            .container {
                max-width: 100%%;
            }
            
            .logo-container {
                width: 85px;
                height: 85px;
                margin-bottom: 24px;
                border-radius: 22px;
            }
            
            .logo-container svg {
                width: 42px;
                height: 42px;
            }
            
            h1 {
                font-size: 24px;
                margin-bottom: 10px;
            }
            
            p {
                font-size: 15px;
                margin-bottom: 24px;
            }
            
            .spinner-container {
                margin: 36px auto;
            }
            
            .spinner {
                width: 50px;
                height: 50px;
                border-width: 3px;
            }
            
            .store-buttons {
                gap: 12px;
                margin-top: 36px;
            }
            
            .store-buttons > p {
                font-size: 14px;
            }
            
            .store-btn {
                padding: 16px 24px;
                font-size: 14px;
                gap: 12px;
                border-radius: 14px;
            }
            
            .store-btn svg {
                width: 22px;
                height: 22px;
            }
            
            .desktop-message {
                padding: 24px 20px;
                border-radius: 20px;
                margin-top: 36px;
            }
            
            .desktop-message h2 {
                font-size: 20px;
                margin-bottom: 14px;
                gap: 10px;
            }
            
            .desktop-message h2 svg {
                width: 26px;
                height: 26px;
            }
            
            .desktop-message > p {
                font-size: 14px;
                margin-bottom: 20px;
            }
            
            .feature-list {
                margin: 20px 0;
            }
            
            .feature-item {
                font-size: 14px;
                gap: 12px;
                margin-bottom: 12px;
            }
            
            .feature-item svg {
                width: 20px;
                height: 20px;
            }
        }
        
        /* Small mobile devices */
        @media (max-width: 360px) {
            .logo-container {
                width: 75px;
                height: 75px;
                margin-bottom: 20px;
            }
            
            .logo-container svg {
                width: 38px;
                height: 38px;
            }
            
            h1 {
                font-size: 22px;
            }
            
            p {
                font-size: 14px;
            }
            
            .store-btn {
                padding: 14px 20px;
                font-size: 13px;
            }
            
            .desktop-message {
                padding: 20px 16px;
            }
            
            .desktop-message h2 {
                font-size: 18px;
                flex-direction: column;
                gap: 8px;
            }
            
            .feature-item {
                font-size: 13px;
            }
        }
        
        /* Landscape orientation pour mobile */
        @media (max-height: 500px) and (orientation: landscape) {
            body {
                padding: 10px;
            }
            
            .logo-container {
                width: 60px;
                height: 60px;
                margin-bottom: 16px;
            }
            
            .logo-container svg {
                width: 30px;
                height: 30px;
            }
            
            h1 {
                font-size: 20px;
                margin-bottom: 8px;
            }
            
            p {
                font-size: 14px;
                margin-bottom: 16px;
            }
            
            .spinner-container {
                margin: 20px auto;
            }
            
            .spinner {
                width: 40px;
                height: 40px;
            }
            
            .store-buttons {
                margin-top: 20px;
                gap: 10px;
            }
            
            .store-btn {
                padding: 12px 20px;
                font-size: 13px;
            }
            
            .desktop-message {
                padding: 20px;
                margin-top: 20px;
            }
            
            .feature-list {
                margin: 16px 0;
            }
            
            .feature-item {
                margin-bottom: 8px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo-container">
            <i data-lucide="shopping-bag"></i>
        </div>
        
        <h1 id="mainTitle">Ouverture de Kivendi...</h1>
        <p id="mainText">Vous allez être redirigé vers l'application</p>
        
        <div class="spinner-container" id="spinnerContainer">
            <div class="spinner"></div>
        </div>
        
        <div class="store-buttons" id="storeButtons">
            <p>L'application n'est pas installée ?</p>
            <a href="%s" class="store-btn" id="androidBtn" style="display:none;">
                <i data-lucide="smartphone"></i>
                <span>Télécharger sur Google Play</span>
            </a>
            <a href="%s" class="store-btn" id="iosBtn" style="display:none;">
                <i data-lucide="apple"></i>
                <span>Télécharger sur App Store</span>
            </a>
            <a href="%s" class="store-btn web-btn" id="webBtn" style="display:none;">
                <i data-lucide="globe"></i>
                <span>Voir sur le site web</span>
            </a>
        </div>
        
        <div class="desktop-message" id="desktopMessage">
            <h2>
                <i data-lucide="monitor"></i>
                <span>Accès depuis un ordinateur</span>
            </h2>
            <p>Kivendi est optimisé pour mobile. Scannez le QR code avec votre téléphone ou visitez le site web.</p>
            <div class="feature-list">
                <div class="feature-item">
                    <i data-lucide="check-circle"></i>
                    <span>Interface mobile intuitive</span>
                </div>
                <div class="feature-item">
                    <i data-lucide="check-circle"></i>
                    <span>Notifications en temps réel</span>
                </div>
                <div class="feature-item">
                    <i data-lucide="check-circle"></i>
                    <span>Géolocalisation des annonces</span>
                </div>
            </div>
            <a href="%s" class="store-btn web-btn">
                <i data-lucide="external-link"></i>
                <span>Voir l'annonce sur le site web</span>
            </a>
        </div>
    </div>

    <script>
        const appDeepLink = '%s';
        const androidStoreURL = '%s';
        const iosStoreURL = '%s';
        const webFallbackURL = '%s';
        
        let appOpened = false;
        let redirectTimer;

        // Initialiser les icônes Lucide
        lucide.createIcons();

        // Détecter le type d'appareil
        const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);
        const isAndroid = /Android/.test(navigator.userAgent);
        const isMobile = isIOS || isAndroid;
        const isDesktop = !isMobile;

        console.log('Device detection:', { isIOS, isAndroid, isMobile, isDesktop });

        // Fonction pour tenter d'ouvrir l'app
        function openApp() {
            if (isDesktop) {
                showDesktopMessage();
                return;
            }

            console.log('Attempting to open app with:', appDeepLink);
            window.location.href = appDeepLink;
            
            redirectTimer = setTimeout(() => {
                if (!document.hidden) {
                    console.log('App did not open, showing store buttons');
                    showStoreButtons();
                }
            }, 2500);
        }

        // Fonction pour afficher les boutons du store (mobile)
        function showStoreButtons() {
            document.getElementById('spinnerContainer').style.display = 'none';
            document.getElementById('mainTitle').textContent = 'Application non installée';
            document.getElementById('mainText').textContent = 'Téléchargez Kivendi pour continuer';
            
            const storeButtons = document.getElementById('storeButtons');
            storeButtons.style.display = 'flex';

            if (isAndroid) {
                document.getElementById('androidBtn').style.display = 'inline-flex';
            } else if (isIOS) {
                document.getElementById('iosBtn').style.display = 'inline-flex';
            }
            
            document.getElementById('webBtn').style.display = 'inline-flex';
            
            // Réinitialiser les icônes après affichage
            lucide.createIcons();
        }

        // Fonction pour afficher le message desktop
        function showDesktopMessage() {
            document.getElementById('spinnerContainer').style.display = 'none';
            document.getElementById('mainTitle').textContent = 'Application Mobile Kivendi';
            document.getElementById('mainText').textContent = '';
            document.getElementById('desktopMessage').style.display = 'block';
            
            // Réinitialiser les icônes
            lucide.createIcons();
            
            // Redirection automatique après 5 secondes
            setTimeout(() => {
                console.log('Redirecting to web fallback:', webFallbackURL);
                window.location.href = webFallbackURL;
            }, 5000);
        }

        // Détecter si l'utilisateur revient sur la page
        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                console.log('Page hidden, app probably opened');
                clearTimeout(redirectTimer);
                appOpened = true;
            }
        });

        // Détecter si l'utilisateur quitte la page
        window.addEventListener('pagehide', () => {
            console.log('Page unloading');
            clearTimeout(redirectTimer);
            appOpened = true;
        });

        // Détecter le blur
        window.addEventListener('blur', () => {
            console.log('Window blur detected');
            clearTimeout(redirectTimer);
            appOpened = true;
        });

        // Tenter d'ouvrir l'app au chargement
        openApp();
    </script>
</body>
</html>
`, androidStoreURL, iosStoreURL, webFallbackURL, webFallbackURL, appDeepLink, androidStoreURL, iosStoreURL, webFallbackURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlContent))
}
