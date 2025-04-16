package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"forum/internal/database"
	"forum/internal/models"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthURL      string
	TokenURL     string
	ProfileURL   string
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

var (
	googleConfig = OAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURI:  "https://localhost:8443/auth/google/callback",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		ProfileURL:   "https://www.googleapis.com/oauth2/v3/userinfo",
	}

	// Configuration pour GitHub OAuth
	githubConfig = OAuthConfig{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		RedirectURI:  "https://localhost:8443/auth/github/callback",
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		ProfileURL:   "https://api.github.com/user",
	}
)

// Structure pour stocker l'état OAuth
type OAuthState struct {
	State    string
	Provider string
	ExpireAt time.Time
}

// Map pour stocker les états OAuth temporairement
var oauthStates = make(map[string]OAuthState)

// Générer un état aléatoire pour sécuriser le flux OAuth
func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// GoogleLoginHandler initie l'authentification avec Google
func GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	
	// Stocker l'état pour la vérification ultérieure
	oauthStates[state] = OAuthState{
		State:    state,
		Provider: "google",
		ExpireAt: time.Now().Add(15 * time.Minute),
	}
	
	// Construire l'URL d'authentification
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=email%%20profile&state=%s",
		googleConfig.AuthURL,
		googleConfig.ClientID,
		url.QueryEscape(googleConfig.RedirectURI),
		state,
	)
	
	// Rediriger vers Google
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// GoogleCallbackHandler traite la réponse de Google après authentification
func GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer le code et l'état depuis les paramètres
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	
	// Vérifier l'état pour prévenir les attaques CSRF
	storedState, exists := oauthStates[state]
	if !exists || storedState.Provider != "google" || storedState.ExpireAt.Before(time.Now()) {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	
	// Supprimer l'état utilisé
	delete(oauthStates, state)
	
	// Échanger le code contre un token
	formData := url.Values{}
	formData.Set("code", code)
	formData.Set("client_id", googleConfig.ClientID)
	formData.Set("client_secret", googleConfig.ClientSecret)
	formData.Set("redirect_uri", googleConfig.RedirectURI)
	formData.Set("grant_type", "authorization_code")
	
	tokenResp, err := http.PostForm(googleConfig.TokenURL, formData)
	if err != nil {
		http.Error(w, "Failed to get token", http.StatusInternalServerError)
		log.Printf("Token exchange error: %v", err)
		return
	}
	defer tokenResp.Body.Close()
	
	var tokenData TokenResponse
	
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		http.Error(w, "Failed to parse token response", http.StatusInternalServerError)
		log.Printf("Token parsing error: %v", err)
		return
	}
	
	// Utiliser le token pour obtenir les informations de l'utilisateur
	profileReq, _ := http.NewRequest("GET", googleConfig.ProfileURL, nil)
	profileReq.Header.Add("Authorization", "Bearer "+tokenData.AccessToken)
	
	client := &http.Client{}
	profileResp, err := client.Do(profileReq)
	if err != nil {
		http.Error(w, "Failed to get user profile", http.StatusInternalServerError)
		log.Printf("Profile request error: %v", err)
		return
	}
	defer profileResp.Body.Close()
	
	var profileData struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Sub      string `json:"sub"` // ID unique de l'utilisateur Google
	}
	
	if err := json.NewDecoder(profileResp.Body).Decode(&profileData); err != nil {
		http.Error(w, "Failed to parse profile data", http.StatusInternalServerError)
		log.Printf("Profile parsing error: %v", err)
		return
	}
	
	// Traiter l'authentification de l'utilisateur
	user, err := processOAuthUser(profileData.Email, profileData.Name, "google_"+profileData.Sub)
	if err != nil {
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		log.Printf("OAuth authentication error: %v", err)
		return
	}
	
	// Créer une session pour l'utilisateur
	session, err := models.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		log.Printf("Error creating session: %v", err)
		return
	}
	
	// Définir le cookie de session
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.UUID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
	})
	
	// Rediriger vers la page d'accueil
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GitHubLoginHandler initie l'authentification avec GitHub
func GitHubLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	
	// Stocker l'état pour la vérification ultérieure
	oauthStates[state] = OAuthState{
		State:    state,
		Provider: "github",
		ExpireAt: time.Now().Add(15 * time.Minute),
	}
	
	// Construire l'URL d'authentification
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
		githubConfig.AuthURL,
		githubConfig.ClientID,
		url.QueryEscape(githubConfig.RedirectURI),
		state,
	)
	
	// Rediriger vers GitHub
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// GitHubCallbackHandler traite la réponse de GitHub après authentification
func GitHubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer le code et l'état depuis les paramètres
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	
	// Vérifier l'état pour prévenir les attaques CSRF
	storedState, exists := oauthStates[state]
	if !exists || storedState.Provider != "github" || storedState.ExpireAt.Before(time.Now()) {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	
	// Supprimer l'état utilisé
	delete(oauthStates, state)
	
	// Échanger le code contre un token
	formData := url.Values{}
	formData.Set("code", code)
	formData.Set("client_id", githubConfig.ClientID)
	formData.Set("client_secret", githubConfig.ClientSecret)
	formData.Set("redirect_uri", githubConfig.RedirectURI)
	
	tokenReq, _ := http.NewRequest("POST", githubConfig.TokenURL, strings.NewReader(formData.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("Accept", "application/json")
	
	client := &http.Client{}
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		http.Error(w, "Failed to get token", http.StatusInternalServerError)
		log.Printf("Token exchange error: %v", err)
		return
	}
	defer tokenResp.Body.Close()
	
	var githubToken TokenResponse
	
	if err := json.NewDecoder(tokenResp.Body).Decode(&githubToken); err != nil {
		http.Error(w, "Failed to parse token response", http.StatusInternalServerError)
		log.Printf("Token parsing error: %v", err)
		return
	}
	
	// Utiliser le token pour obtenir les informations de l'utilisateur
	profileReq, _ := http.NewRequest("GET", githubConfig.ProfileURL, nil)
	profileReq.Header.Add("Authorization", "token "+githubToken.AccessToken)
	profileReq.Header.Add("Accept", "application/json")
	
	profileResp, err := client.Do(profileReq)
	if err != nil {
		http.Error(w, "Failed to get user profile", http.StatusInternalServerError)
		log.Printf("Profile request error: %v", err)
		return
	}
	defer profileResp.Body.Close()
	
	var profileData struct {
		Login string `json:"login"`
		ID    int    `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	
	if err := json.NewDecoder(profileResp.Body).Decode(&profileData); err != nil {
		http.Error(w, "Failed to parse profile data", http.StatusInternalServerError)
		log.Printf("Profile parsing error: %v", err)
		return
	}
	
	// Si l'email n'est pas disponible publiquement, on le récupère via l'API emails
	if profileData.Email == "" {
		emailReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		emailReq.Header.Add("Authorization", "token "+githubToken.AccessToken)
		emailReq.Header.Add("Accept", "application/json")
		
		emailResp, err := client.Do(emailReq)
		if err == nil {
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			
			if json.NewDecoder(emailResp.Body).Decode(&emails) == nil {
				for _, email := range emails {
					if email.Primary && email.Verified {
						profileData.Email = email.Email
						break
					}
				}
			}
			emailResp.Body.Close()
		}
	}
	
	// Fallback si l'email est toujours vide
	if profileData.Email == "" {
		profileData.Email = fmt.Sprintf("%s@github.com", profileData.Login)
	}
	
	// Si le nom est vide, on utilise le login
	if profileData.Name == "" {
		profileData.Name = profileData.Login
	}
	
	// Traiter l'authentification de l'utilisateur
	user, err := processOAuthUser(profileData.Email, profileData.Name, fmt.Sprintf("github_%d", profileData.ID))
	if err != nil {
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		log.Printf("OAuth authentication error: %v", err)
		return
	}
	
	// Créer une session pour l'utilisateur
	session, err := models.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		log.Printf("Error creating session: %v", err)
		return
	}
	
	// Définir le cookie de session
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.UUID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
	})
	
	// Rediriger vers la page d'accueil
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Traiter l'authentification OAuth
func processOAuthUser(email, name, oauthID string) (*models.User, error) {
	// Vérifier si l'utilisateur existe déjà avec cet ID OAuth
	var userID int
	
	err := database.DB.QueryRow(
		"SELECT id FROM users WHERE oauth_id = ?", 
		oauthID,
	).Scan(&userID)
	
	if err == nil {
		// L'utilisateur existe déjà, on le récupère
		return models.GetUserByID(userID)
	} else if err != sql.ErrNoRows {
		// Erreur inattendue
		return nil, err
	}
	
	// Vérifier si l'utilisateur existe avec cet email
	err = database.DB.QueryRow(
		"SELECT id FROM users WHERE email = ?",
		email,
	).Scan(&userID)
	
	if err == nil {
		// L'utilisateur existe avec cet email, on met à jour son OAuth ID
		_, err = database.DB.Exec(
			"UPDATE users SET oauth_id = ? WHERE id = ?",
			oauthID, userID,
		)
		if err != nil {
			return nil, err
		}
		return models.GetUserByID(userID)
	} else if err != sql.ErrNoRows {
		// Erreur inattendue
		return nil, err
	}
	
	// L'utilisateur n'existe pas, on le crée
	username := generateUsername(name)
	
	// Insérer le nouvel utilisateur dans la base de données
	result, err := database.DB.Exec(
		"INSERT INTO users (email, username, password, oauth_id, role) VALUES (?, ?, ?, ?, ?)",
		email, username, "", oauthID, "user",
	)
	if err != nil {
		return nil, err
	}
	
	// Récupérer l'ID du nouvel utilisateur
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	
	// Récupérer l'utilisateur créé
	return models.GetUserByID(int(id))
}

// Générer un nom d'utilisateur unique à partir du nom complet
func generateUsername(name string) string {
	// Nettoyer le nom (enlever les espaces, caractères spéciaux, etc.)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	
	// Vérifier si ce nom d'utilisateur existe déjà
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", name).Scan(&count)
	if err != nil || count == 0 {
		// Si le nom n'existe pas ou s'il y a une erreur, on retourne le nom tel quel
		return name
	}
	
	// Ajouter un nombre aléatoire au nom
	return fmt.Sprintf("%s%d", name, time.Now().UnixNano()%10000)
}