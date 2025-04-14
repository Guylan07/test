package handlers

import (
	"forum/internal/middleware"
	"forum/internal/models"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// CreateCommentHandler gère la création d'un nouveau commentaire
func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur est connecté
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Analyser le formulaire
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Récupérer les données du formulaire
	content := r.FormValue("content")
	postIDStr := r.FormValue("post_id")

	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	// Validation de base
	if content == "" {
		http.Error(w, "Comment content is required", http.StatusBadRequest)
		return
	}

	// Créer le commentaire
	_, err = models.CreateComment(content, currentUser.ID, postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating comment: %v", err)
		return
	}

	// Rediriger vers la page du post
	http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
}

// EditCommentHandler gère la modification d'un commentaire existant
func EditCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur est connecté
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Analyser le formulaire
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Récupérer les données du formulaire
	content := r.FormValue("content")
	commentIDStr := r.FormValue("comment_id")
	postIDStr := r.FormValue("post_id")

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil || commentID <= 0 {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	// Validation de base
	if content == "" {
		http.Error(w, "Comment content is required", http.StatusBadRequest)
		return
	}

	// Mettre à jour le commentaire
	err = models.UpdateComment(commentID, currentUser.ID, content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error updating comment: %v", err)
		return
	}

	// Rediriger vers la page du post
	http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
}

// DeleteCommentHandler gère la suppression d'un commentaire
func DeleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Extraire l'ID du commentaire de l'URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 || parts[2] != "delete" {
		http.NotFound(w, r)
		return
	}

	commentID, err := strconv.Atoi(parts[3])
	if err != nil || commentID <= 0 {
		http.NotFound(w, r)
		return
	}

	// Vérifier que l'utilisateur est connecté
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Récupérer l'ID du post pour la redirection
	postIDStr := r.URL.Query().Get("post_id")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	// Supprimer le commentaire
	isAdmin := currentUser.Role == "admin"
	err = models.DeleteComment(commentID, currentUser.ID, isAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error deleting comment: %v", err)
		return
	}

	// Rediriger vers la page du post
	http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
}

// ReactToCommentHandler gère les réactions (like/dislike) à un commentaire
func ReactToCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur est connecté
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Error(w, "You must be logged in to react to comments", http.StatusUnauthorized)
		return
	}

	// Analyser le formulaire
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Récupérer les données du formulaire
	commentIDStr := r.FormValue("comment_id")
	postIDStr := r.FormValue("post_id")
	reactionType := r.FormValue("reaction_type") // "like", "dislike" ou "" (pour supprimer)

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil || commentID <= 0 {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	// Enregistrer la réaction
	err = models.ReactToComment(commentID, currentUser.ID, reactionType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error reacting to comment: %v", err)
		return
	}

	// Rediriger vers la page du post
	http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
}