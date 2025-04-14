package middleware

import (
	"net/http"
)

// RequireRoleMiddleware vérifie que l'utilisateur possède au moins le rôle requis
func RequireRoleMiddleware(next http.Handler, requiredRole string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Récupérer l'utilisateur actuel depuis le contexte
		currentUser := GetUserFromContext(r)
		
		// Si l'utilisateur n'est pas connecté, rediriger vers la page de connexion
		if currentUser == nil {
			http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
			return
		}
		
		// Vérifier le rôle de l'utilisateur
		if !hasRequiredRole(currentUser.Role, requiredRole) {
			http.Error(w, "Accès refusé. Vous n'avez pas les permissions nécessaires.", http.StatusForbidden)
			return
		}
		
		// Si l'utilisateur a le rôle requis, continuer
		next.ServeHTTP(w, r)
	})
}

// hasRequiredRole vérifie si le rôle de l'utilisateur est suffisant
func hasRequiredRole(userRole, requiredRole string) bool {
	// Définir la hiérarchie des rôles
	roleHierarchy := map[string]int{
		"user":      1,
		"moderator": 2,
		"admin":     3,
	}
	
	// Vérifier que les rôles existent
	userRoleLevel, userExists := roleHierarchy[userRole]
	requiredRoleLevel, requiredExists := roleHierarchy[requiredRole]
	
	// Si un des rôles n'existe pas dans la hiérarchie, refuser l'accès
	if !userExists || !requiredExists {
		return false
	}
	
	// Vérifier si le niveau du rôle de l'utilisateur est suffisant
	return userRoleLevel >= requiredRoleLevel
}