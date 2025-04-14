package handlers

import (
	"forum/internal/middleware"
	"forum/internal/models"
	"forum/internal/utils"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// ListPendingPostsHandler affiche les posts en attente de modération
func ListPendingPostsHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que l'utilisateur a le rôle de modérateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || (currentUser.Role != "moderator" && currentUser.Role != "admin") {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Récupérer tous les posts en attente
	pendingPosts, err := models.GetPendingPosts()
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des posts en attente", http.StatusInternalServerError)
		log.Printf("Error fetching pending posts: %v", err)
		return
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"PendingPosts": pendingPosts,
		"CurrentUser":  currentUser,
		"PageTitle":    "Modération",
	}

	// Charger et exécuter le template
	tmpl, err := utils.ParseTemplate("templates/base.html", "templates/moderation.html")
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

// ApprovePostHandler approuve un post en attente
func ApprovePostHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur a le rôle de modérateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || (currentUser.Role != "moderator" && currentUser.Role != "admin") {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Extraire l'ID du post de l'URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}

	postID, err := strconv.Atoi(parts[3])
	if err != nil || postID <= 0 {
		http.NotFound(w, r)
		return
	}

	// Approuver le post
	newPostID, err := models.ApprovePendingPost(postID, currentUser.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error approving post: %v", err)
		return
	}

	// Rediriger vers le post approuvé
	http.Redirect(w, r, "/post/"+strconv.Itoa(newPostID), http.StatusSeeOther)
}

// RejectPostHandler rejette un post en attente
func RejectPostHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur a le rôle de modérateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || (currentUser.Role != "moderator" && currentUser.Role != "admin") {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Extraire l'ID du post de l'URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}

	postID, err := strconv.Atoi(parts[3])
	if err != nil || postID <= 0 {
		http.NotFound(w, r)
		return
	}

	// Analyser le formulaire pour récupérer la raison du rejet
	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Requête incorrecte", http.StatusBadRequest)
		return
	}

	reason := r.FormValue("reason")
	if reason == "" {
		http.Error(w, "La raison du rejet est requise", http.StatusBadRequest)
		return
	}

	// Rejeter le post
	err = models.RejectPendingPost(postID, currentUser.ID, reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error rejecting post: %v", err)
		return
	}

	// Rediriger vers la liste des posts en attente
	http.Redirect(w, r, "/mod/pending", http.StatusSeeOther)
}

// ListReportsHandler affiche les signalements en attente
func ListReportsHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que l'utilisateur a le rôle d'administrateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || currentUser.Role != "admin" {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Récupérer tous les signalements en attente
	reports, err := models.GetReports()
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des signalements", http.StatusInternalServerError)
		log.Printf("Error fetching reports: %v", err)
		return
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Reports":      reports,
		"CurrentUser":  currentUser,
		"PageTitle":    "Gestion des signalements",
	}

	// Charger et exécuter le template
	tmpl, err := utils.ParseTemplate("templates/base.html", "templates/reports.html")
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

// HandleReportHandler traite un signalement (approbation ou rejet)
func HandleReportHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur a le rôle d'administrateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || currentUser.Role != "admin" {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Extraire l'ID du signalement de l'URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}

	reportID, err := strconv.Atoi(parts[3])
	if err != nil || reportID <= 0 {
		http.NotFound(w, r)
		return
	}

	// Analyser le formulaire
	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Requête incorrecte", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	response := r.FormValue("response")

	if action != "approve" && action != "reject" {
		http.Error(w, "Action invalide", http.StatusBadRequest)
		return
	}

	// Traiter le signalement
	status := "rejected"
	if action == "approve" {
		status = "approved"
	}

	err = models.HandleReport(reportID, currentUser.ID, status, response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error handling report: %v", err)
		return
	}

	// Rediriger vers la liste des signalements
	http.Redirect(w, r, "/admin/reports", http.StatusSeeOther)
}

// ListUsersHandler affiche la liste des utilisateurs pour l'administration
func ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que l'utilisateur a le rôle d'administrateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || currentUser.Role != "admin" {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Récupérer tous les utilisateurs
	users, err := models.GetAllUsers()
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des utilisateurs", http.StatusInternalServerError)
		log.Printf("Error fetching users: %v", err)
		return
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Users":        users,
		"CurrentUser":  currentUser,
		"PageTitle":    "Gestion des utilisateurs",
	}

	// Charger et exécuter le template
	tmpl, err := utils.ParseTemplate("templates/base.html", "templates/admin_users.html")
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

// UpdateUserRoleHandler met à jour le rôle d'un utilisateur
func UpdateUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur a le rôle d'administrateur
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil || currentUser.Role != "admin" {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Extraire l'ID de l'utilisateur de l'URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}

	userID, err := strconv.Atoi(parts[3])
	if err != nil || userID <= 0 {
		http.NotFound(w, r)
		return
	}

	// Analyser le formulaire
	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Requête incorrecte", http.StatusBadRequest)
		return
	}

	role := r.FormValue("role")
	if role != "user" && role != "moderator" && role != "admin" {
		http.Error(w, "Rôle invalide", http.StatusBadRequest)
		return
	}

	// Ne pas permettre à un administrateur de rétrograder son propre compte
	if userID == currentUser.ID && role != "admin" {
		http.Error(w, "Vous ne pouvez pas rétrograder votre propre compte administrateur", http.StatusBadRequest)
		return
	}

	// Mettre à jour le rôle de l'utilisateur
	err = models.UpdateUserRole(userID, role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error updating user role: %v", err)
		return
	}

	// Rediriger vers la liste des utilisateurs
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}