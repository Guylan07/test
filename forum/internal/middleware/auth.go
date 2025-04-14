package middleware
import (
	"context"
	"forum/internal/models"
	"net/http"
)

// Ces variables définissent une clé spéciale pour stocker des informations utilisateur
// C'est comme créer une étiquette unique pour ranger des objets dans un casier
type contextKey string
const UserContextKey contextKey = "user"

// AuthMiddleware vérifie si un utilisateur est authentifié
// C'est comme un portier qui vérifie discrètement votre identité sans vous bloquer le passage
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// On essaie de récupérer le cookie de session du navigateur
		// C'est comme vérifier si le visiteur porte un bracelet d'identification
		cookie, err := r.Cookie("session")
		if err != nil {
			// Si aucun cookie de session n'est trouvé, l'utilisateur continue en tant qu'invité
			// C'est comme laisser passer quelqu'un sans bracelet, mais en le considérant comme un visiteur
			next.ServeHTTP(w, r)
			return
		}
		
		// On vérifie si la session est valide dans notre base de données
		// C'est comme scanner le bracelet pour voir s'il est authentique et toujours valable
		session, err := models.GetSessionByUUID(cookie.Value)
		if err != nil {
			// Si la session est invalide ou expirée, on supprime le cookie du navigateur
			// C'est comme retirer un bracelet périmé ou contrefait
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				Secure:   true,
				MaxAge:   -1,  // Expire immédiatement
			})
			// L'utilisateur continue en tant qu'invité
			next.ServeHTTP(w, r)
			return
		}
		
		// On récupère les informations de l'utilisateur associé à cette session
		// C'est comme vérifier dans notre registre à qui appartient ce bracelet valide
		user, err := models.GetUserByID(session.UserID)
		if err != nil {
			// Si l'utilisateur n'est pas trouvé, on supprime la session et le cookie
			// C'est comme si le bracelet était valide mais que la personne n'existe plus dans notre système
			models.DeleteSession(cookie.Value)
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				Secure:   true,
				MaxAge:   -1,  // Expire immédiatement
			})
			// L'utilisateur continue en tant qu'invité
			next.ServeHTTP(w, r)
			return
		}
		
		// On ajoute l'utilisateur au contexte de la requête pour que les pages puissent le reconnaître
		// C'est comme attacher une étiquette invisible au visiteur pour que tous les services sachent qui il est
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		// On continue vers la page demandée, mais avec cette information supplémentaire
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuthMiddleware redirige vers la page de connexion si l'utilisateur n'est pas authentifié
// C'est comme un gardien qui vérifie si vous avez le droit d'entrer dans une zone réservée
func RequireAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// On vérifie si l'utilisateur est présent dans le contexte de la requête
		// C'est comme vérifier si le visiteur porte l'étiquette d'identification invisible
		user := r.Context().Value(UserContextKey)
		if user == nil {
			// Si l'utilisateur n'est pas authentifié, on le redirige vers la page de connexion
			// C'est comme dire "Désolé, vous devez vous identifier avant d'entrer dans cette zone"
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// Si l'utilisateur est authentifié, on le laisse accéder à la page demandée
		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext récupère l'utilisateur depuis le contexte de la requête
// C'est comme un service qui lit l'étiquette invisible pour savoir qui est le visiteur
func GetUserFromContext(r *http.Request) *models.User {
	// On essaie de récupérer et convertir l'utilisateur depuis le contexte
	// C'est comme essayer de lire et comprendre l'information sur l'étiquette
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		// Si l'information n'est pas lisible ou n'existe pas, on retourne rien
		return nil
	}
	// On retourne l'utilisateur trouvé
	return user
}