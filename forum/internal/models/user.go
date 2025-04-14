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
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("email already exists")
	}

	err = database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = database.DB.Exec(
		"INSERT INTO users (email, username, password, role) VALUES (?, ?, ?, ?)",
		email, username, string(hashedPassword), "user",
	)
	if err != nil {
		return err
	}

	return nil
}

// GetUserByEmail recherche un utilisateur par son adresse email
func GetUserByEmail(email string) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, email, username, password, role, created_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return user, nil
}

// GetUserByID recherche un utilisateur par son numéro d'identification
func GetUserByID(id int) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, email, username, password, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return user, nil
}

// Authenticate vérifie si les identifiants fournis correspondent à un utilisateur
func Authenticate(email, password string) (*User, error) {
	user, err := GetUserByEmail(email)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, errors.New("invalid password")
	}

	return user, nil
}

// CreateSession crée une nouvelle session pour un utilisateur connecté
func CreateSession(userID int) (*Session, error) {
	sessionUUID := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err := database.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}

	_, err = database.DB.Exec(
		"INSERT INTO sessions (uuid, user_id, expires_at) VALUES (?, ?, ?)",
		sessionUUID, userID, expiresAt,
	)
	if err != nil {
		return nil, err
	}

	session := &Session{
		UUID:      sessionUUID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}

	return session, nil
}

// GetSessionByUUID recherche une session par son code unique
func GetSessionByUUID(uuid string) (*Session, error) {
	session := &Session{}
	err := database.DB.QueryRow(
		"SELECT id, uuid, user_id, created_at, expires_at FROM sessions WHERE uuid = ?",
		uuid,
	).Scan(&session.ID, &session.UUID, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	if session.ExpiresAt.Before(time.Now()) {
		_, err = database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", uuid)
		if err != nil {
			log.Printf("Error deleting expired session: %v", err)
		}
		return nil, errors.New("session expired")
	}

	return session, nil
}

// DeleteSession supprime une session par son code unique
func DeleteSession(uuid string) error {
	_, err := database.DB.Exec("DELETE FROM sessions WHERE uuid = ?", uuid)
	return err
}

// GetAllUsers récupère tous les utilisateurs
// C'est comme récupérer toutes les fiches d'inscription du registre des membres
func GetAllUsers() ([]*User, error) {
	users := []*User{}

	rows, err := database.DB.Query(`
		SELECT id, email, username, password, role, created_at 
		FROM users 
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		user := &User{}
		err = rows.Scan(
			&user.ID, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}
