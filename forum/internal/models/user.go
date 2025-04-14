package models

import (
	"database/sql"
	"errors"
	"forum/internal/database"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User est comme une fiche d'identité pour chaque personne qui utilise le forum
// Elle contient toutes les informations dont nous avons besoin pour identifier et gérer un utilisateur
type User struct {
	ID        int        // Un numéro unique qui identifie chaque utilisateur
	Email     string     // L'adresse email de l'utilisateur
	Username  string     // Le nom d'utilisateur choisi
	Password  string     // Le mot de passe (stocké de façon sécurisée)
	Role      string     // Le rôle de l'utilisateur (administrateur, modérateur, utilisateur simple...)
	CreatedAt time.Time  // La date et l'heure de création du compte
}

// Session est comme un bracelet d'entrée temporaire pour un utilisateur connecté
// Elle permet de savoir qui est connecté sans lui demander son mot de passe à chaque fois
type Session struct {
	ID        int       // Un numéro unique qui identifie chaque session
	UUID      string    // Un code secret unique pour cette session
	UserID    int       // Le numéro de l'utilisateur à qui appartient cette session
	CreatedAt time.Time // Quand la session a été créée
	ExpiresAt time.Time // Quand la session expire et n'est plus valide
}

// RegisterUser inscrit un nouvel utilisateur dans la base de données
// C'est comme remplir un formulaire d'inscription et l'ajouter au registre des membres
func RegisterUser(email, username, password string) error {
	// On vérifie d'abord si l'email existe déjà dans notre base de données
	// C'est comme vérifier si quelqu'un s'est déjà inscrit avec cette adresse
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		// Si quelque chose ne va pas avec notre recherche, on signale l'erreur
		return err
	}
	if count > 0 {
		// Si l'email existe déjà, on refuse l'inscription
		return errors.New("email already exists")
	}

	// On vérifie aussi si le nom d'utilisateur est déjà pris
	// C'est comme vérifier si quelqu'un utilise déjà ce pseudo
	err = database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		// Si quelque chose ne va pas avec notre recherche, on signale l'erreur
		return err
	}
	if count > 0 {
		// Si le nom d'utilisateur existe déjà, on refuse l'inscription
		return errors.New("username already exists")
	}

	// On sécurise le mot de passe en le transformant en code illisible
	// C'est comme transformer un texte normal en texte codé que seul notre système peut vérifier
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		// Si on n'arrive pas à sécuriser le mot de passe, on signale l'erreur
		return err
	}

	// On ajoute le nouvel utilisateur dans notre base de données
	// C'est comme écrire une nouvelle fiche dans notre registre des membres
	_, err = database.DB.Exec(
		"INSERT INTO users (email, username, password, role) VALUES (?, ?, ?, ?)",
		email, username, string(hashedPassword), "user",
	)
	if err != nil {
		// Si on n'arrive pas à ajouter l'utilisateur, on signale l'erreur
		return err
	}

	// Tout s'est bien passé, on ne signale aucune erreur
	return nil
}

// GetUserByEmail recherche un utilisateur par son adresse email
// C'est comme chercher une fiche dans notre registre en utilisant l'email comme critère
func GetUserByEmail(email string) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, email, username, password, role, created_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Si aucun utilisateur n'a cet email, on signale qu'on ne l'a pas trouvé
			return nil, errors.New("user not found")
		}
		// Si une autre erreur se produit, on la signale
		return nil, err
	}
	// On retourne l'utilisateur trouvé
	return user, nil
}

// GetUserByID recherche un utilisateur par son numéro d'identification
// C'est comme chercher une fiche dans notre registre en utilisant le numéro unique comme critère
func GetUserByID(id int) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, email, username, password, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Si aucun utilisateur n'a ce numéro, on signale qu'on ne l'a pas trouvé
			return nil, errors.New("user not found")
		}
		// Si une autre erreur se produit, on la signale
		return nil, err
	}
	// On retourne l'utilisateur trouvé
	return user, nil
}

// Authenticate vérifie si les identifiants fournis correspondent à un utilisateur
// C'est comme vérifier si quelqu'un qui veut entrer connaît le bon mot de passe
func Authenticate(email, password string) (*User, error) {
	// On recherche d'abord l'utilisateur par son email
	user, err := GetUserByEmail(email)
	if err != nil {
		// Si on ne trouve pas l'utilisateur ou s'il y a une erreur, on la signale
		return nil, err
	}

	// On vérifie si le mot de passe fourni correspond au mot de passe enregistré
	// C'est comme comparer le code secret fourni avec celui que nous avons stocké
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Si le mot de passe ne correspond pas, on refuse l'accès
		return nil, errors.New("invalid password")
	}

	// Les identifiants sont corrects, on retourne l'utilisateur
	return user, nil
}

// CreateSession crée une nouvelle session pour un utilisateur connecté
// C'est comme donner un bracelet temporaire à quelqu'un qui vient de s'identifier
func CreateSession(userID int) (*Session, error) {
	// On génère un code unique pour cette session
	// C'est comme créer un numéro de série unique pour chaque bracelet
	sessionUUID := uuid.New().String()

	// On définit quand ce bracelet ne sera plus valide (dans 24 heures)
	expiresAt := time.Now().Add(24 * time.Hour)

	// On supprime les anciennes sessions de cet utilisateur
	// C'est comme récupérer les anciens bracelets avant d'en donner un nouveau
	_, err := database.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		// Si on n'arrive pas à supprimer les anciennes sessions, on signale l'erreur
		return nil, err
	}

	// On enregistre la nouvelle session dans notre base de données
	// C'est comme ajouter ce nouveau bracelet à notre registre
	_, err = database.DB.Exec(
		"INSERT INTO sessions (uuid, user_id, expires_at) VALUES (?, ?, ?)",
		sessionUUID, userID, expiresAt,
	)
	if err != nil {
		// Si on n'arrive pas à enregistrer la session, on signale l'erreur
		return nil, err
	}

	// On crée un objet Session avec les informations
	session := &Session{
		UUID:      sessionUUID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}

	// On retourne la session créée
	return session, nil
}

// GetSessionByUUID recherche une session par son code unique
// C'est comme vérifier si un bracelet existe et est valide en utilisant son numéro de série
func GetSessionByUUID(uuid string) (*Session, error) {
	session := &Session{}
	err := database.DB.QueryRow(
		"SELECT id, uuid, user_id, created_at, expires_at FROM sessions WHERE uuid = ?",
		uuid,
	).Scan(&session.ID, &session.UUID, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Si aucune session n'a ce code, on signale qu'on ne l'a pas trouvée
			return nil, errors.New("session not found")
		}
		// Si une autre erreur se produit, on la signale
		return nil, err
	}

	// On vérifie si la session n'est pas périmée
	// C'est comme vérifier si le bracelet n'a pas dépassé sa date d'expiration
	if session.ExpiresAt.Before(time.Now()) {
		// Si la session a expiré, on la supprime de notre base de données
		_, err = database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", uuid)
		if err != nil {
			// Si on n'arrive pas à supprimer la session expirée, on le note dans le journal
			log.Printf("Error deleting expired session: %v", err)
		}
		// On signale que la session a expiré
		return nil, errors.New("session expired")
	}

	// La session existe et est valide, on la retourne
	return session, nil
}

// DeleteSession supprime une session par son code unique
// C'est comme récupérer et détruire un bracelet quand quelqu'un quitte (se déconnecte)
func DeleteSession(uuid string) error {
	_, err := database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", uuid)
	return err
}