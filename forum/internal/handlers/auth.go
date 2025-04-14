package handlers
import (
	"forum/internal/models"
	"html/template"
	"log"
	"net/http"
	"time"
)

// RegisterHandler s'occupe de l'inscription des nouveaux utilisateurs
// C'est comme un guichetier qui traite les formulaires d'inscription
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// Si quelqu'un demande simplement à voir le formulaire d'inscription (requête GET)
	// C'est comme quand quelqu'un vous demande un formulaire vierge
	if r.Method == http.MethodGet {
		// On charge le modèle de page HTML pour l'inscription
		// C'est comme sortir le bon formulaire de notre classeur
		tmpl, err := template.ParseFiles("templates/register.html")
		if err != nil {
			// Si on ne trouve pas le bon formulaire, on signale une erreur
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}
		// On envoie le formulaire vide à l'utilisateur
		tmpl.Execute(w, nil)
		return
	}
	
	// Si quelqu'un nous envoie un formulaire rempli (requête POST)
	// C'est comme quand quelqu'un vous rend un formulaire complété
	if r.Method == http.MethodPost {
		// On analyse le contenu du formulaire reçu
		// C'est comme lire les informations inscrites sur le formulaire
		err := r.ParseForm()
		if err != nil {
			// Si le formulaire est illisible, on signale une erreur
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		
		// On récupère les valeurs saisies dans chaque champ du formulaire
		email := r.FormValue("email")
		username := r.FormValue("username")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		
		// On vérifie que tous les champs obligatoires sont remplis
		// C'est comme s'assurer qu'aucune information essentielle ne manque
		if email == "" || username == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}
		
		// On vérifie que les deux mots de passe saisis sont identiques
		// C'est une précaution pour s'assurer que l'utilisateur n'a pas fait de faute de frappe
		if password != confirmPassword {
			http.Error(w, "Passwords do not match", http.StatusBadRequest)
			return
		}
		
		// On enregistre le nouvel utilisateur dans notre base de données
		// C'est comme ajouter une nouvelle fiche dans notre registre des membres
		err = models.RegisterUser(email, username, password)
		if err != nil {
			// Si l'inscription échoue (par exemple si l'email est déjà pris), on signale l'erreur
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		// Si l'inscription réussit, on redirige l'utilisateur vers la page de connexion
		// C'est comme dire "Votre inscription est réussie, maintenant vous pouvez vous connecter"
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	
	// Si quelqu'un essaie d'utiliser une méthode non autorisée (ni GET ni POST)
	// C'est comme si quelqu'un essayait d'accéder au guichet par une entrée interdite
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// LoginHandler s'occupe de la connexion des utilisateurs
// C'est comme un agent de sécurité qui vérifie les identifiants à l'entrée
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Si quelqu'un demande simplement à voir le formulaire de connexion (requête GET)
	// C'est comme quand quelqu'un demande un badge d'accès vierge
	if r.Method == http.MethodGet {
		// On charge le modèle de page HTML pour la connexion
		// C'est comme sortir le bon formulaire de notre classeur
		tmpl, err := template.ParseFiles("templates/login.html")
		if err != nil {
			// Si on ne trouve pas le bon formulaire, on signale une erreur
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}
		// On envoie le formulaire vide à l'utilisateur
		tmpl.Execute(w, nil)
		return
	}
	
	// Si quelqu'un nous envoie ses identifiants de connexion (requête POST)
	// C'est comme quand quelqu'un présente ses papiers d'identité
	if r.Method == http.MethodPost {
		// On analyse le contenu du formulaire reçu
		err := r.ParseForm()
		if err != nil {
			// Si le formulaire est illisible, on signale une erreur
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		
		// On récupère l'email et le mot de passe saisis
		email := r.FormValue("email")
		password := r.FormValue("password")
		
		// On vérifie si les identifiants sont corrects
		// C'est comme vérifier si les papiers d'identité sont authentiques
		user, err := models.Authenticate(email, password)
		if err != nil {
			// Si les identifiants sont incorrects, on refuse l'accès
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		
		// Si les identifiants sont corrects, on crée une session pour l'utilisateur
		// C'est comme donner un bracelet temporaire à un visiteur qui a montré ses papiers
		session, err := models.CreateSession(user.ID)
		if err != nil {
			// Si on n'arrive pas à créer la session, on signale une erreur
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			log.Printf("Error creating session: %v", err)
			return
		}
		
		// On envoie un cookie au navigateur de l'utilisateur pour stocker l'identifiant de session
		// C'est comme coller un autocollant invisible sur le téléphone du visiteur
		http.SetCookie(w, &http.Cookie{
			Name:     "session",  // Le nom du cookie
			Value:    session.UUID,  // La valeur unique de la session
			Path:     "/",  // Valable sur tout le site
			HttpOnly: true,  // Invisible aux scripts JavaScript (sécurité)
			Secure:   true,  // Uniquement transmis en HTTPS (sécurité)
			SameSite: http.SameSiteStrictMode,  // Contrôle d'origine stricte (sécurité)
			Expires:  session.ExpiresAt,  // Date d'expiration de la session
		})
		
		// On redirige l'utilisateur vers la page d'accueil
		// C'est comme ouvrir la porte après avoir vérifié les papiers
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	
	// Si quelqu'un essaie d'utiliser une méthode non autorisée (ni GET ni POST)
	// C'est comme si quelqu'un essayait d'entrer par une fenêtre au lieu de la porte
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// LogoutHandler s'occupe de la déconnexion des utilisateurs
// C'est comme un agent qui récupère le bracelet d'accès quand un visiteur part
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// On récupère le cookie de session dans le navigateur de l'utilisateur
	// C'est comme vérifier si le visiteur a bien un bracelet
	cookie, err := r.Cookie("session")
	if err == nil {
		// Si le cookie existe, on supprime la session correspondante de la base de données
		// C'est comme désactiver le bracelet dans notre système
		models.DeleteSession(cookie.Value)
		
		// On supprime aussi le cookie du navigateur de l'utilisateur
		// C'est comme retirer physiquement le bracelet au visiteur
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "",  // Valeur vide
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,  // Expire immédiatement
			Expires:  time.Now().Add(-1 * time.Hour),  // Date dans le passé
		})
	}
	
	// On redirige l'utilisateur vers la page d'accueil
	// C'est comme raccompagner le visiteur vers la sortie
	http.Redirect(w, r, "/", http.StatusSeeOther)
}