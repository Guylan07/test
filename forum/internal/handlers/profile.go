package handlers

import (
	"forum/internal/middleware"
	"forum/internal/models"
	"forum/internal/utils"
	"log"
	"net/http"
	"strconv"
)

// ProfileHandler gère l'affichage de la page de profil/activité de l'utilisateur
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que l'utilisateur est connecté
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Récupérer l'ID de l'utilisateur à afficher, par défaut l'utilisateur courant
	userIDStr := r.URL.Query().Get("user")
	userID := currentUser.ID // Par défaut, l'utilisateur courant
	profileUser := currentUser

	// Si un ID est spécifié dans l'URL et que ce n'est pas l'utilisateur courant
	if userIDStr != "" {
		requestedID, err := strconv.Atoi(userIDStr)
		if err == nil && requestedID > 0 && requestedID != currentUser.ID {
			userID = requestedID
			// Récupérer les informations de l'utilisateur demandé
			profileUser, err = models.GetUserByID(userID)
			if err != nil {
				http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
				return
			}
		}
	}

	// Récupérer l'onglet actif
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "activity" // Onglet par défaut
	}

	// Préparer les données en fonction de l'onglet sélectionné
	var data = map[string]interface{}{
		"CurrentUser":  currentUser,
		"ProfileUser":  profileUser,
		"IsOwnProfile": currentUser.ID == profileUser.ID,
		"ActiveTab":    tab,
		"PageTitle":    "Profil de " + profileUser.Username,
	}

	// Récupérer les données spécifiques à l'onglet
	switch tab {
	case "activity":
		activities, err := models.GetUserActivity(userID, 50) // Limité aux 50 dernières activités
		if err != nil {
			log.Printf("Error fetching user activities: %v", err)
			http.Error(w, "Erreur lors de la récupération des activités", http.StatusInternalServerError)
			return
		}
		data["Activities"] = activities

	case "posts":
		posts, err := models.GetUserPosts(userID)
		if err != nil {
			log.Printf("Error fetching user posts: %v", err)
			http.Error(w, "Erreur lors de la récupération des posts", http.StatusInternalServerError)
			return
		}
		data["Posts"] = posts

	case "comments":
		comments, err := models.GetUserComments(userID)
		if err != nil {
			log.Printf("Error fetching user comments: %v", err)
			http.Error(w, "Erreur lors de la récupération des commentaires", http.StatusInternalServerError)
			return
		}
		data["Comments"] = comments

	case "likes":
		likedPosts, err := models.GetUserLikedPosts(userID)
		if err != nil {
			log.Printf("Error fetching liked posts: %v", err)
			http.Error(w, "Erreur lors de la récupération des posts aimés", http.StatusInternalServerError)
			return
		}
		data["LikedPosts"] = likedPosts

	case "dislikes":
		dislikedPosts, err := models.GetUserDislikedPosts(userID)
		if err != nil {
			log.Printf("Error fetching disliked posts: %v", err)
			http.Error(w, "Erreur lors de la récupération des posts non aimés", http.StatusInternalServerError)
			return
		}
		data["DislikedPosts"] = dislikedPosts
	}

	// Charger et exécuter le template
	tmpl, err := utils.ParseTemplate("templates/base.html", "templates/profile.html")
	if err != nil {
		http.Error(w, "Erreur lors du chargement des templates", http.StatusInternalServerError)
		log.Printf("Error parsing template: %v", err)
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		http.Error(w, "Erreur lors du rendu de la page", http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
	}
}

// N'oubliez pas d'ajouter cette route dans le fichier cmd/main.go:
// mux.HandleFunc("/profile", handlers.ProfileHandler)