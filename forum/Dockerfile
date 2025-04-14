FROM golang:1.21

WORKDIR /app

# Installer les dépendances nécessaires
RUN apt-get update && apt-get install -y gcc libsqlite3-dev

# Copier les fichiers de dépendances et télécharger les modules
COPY go.mod go.sum ./
RUN go mod download

# Copier le code source
COPY . .

# Créer les dossiers nécessaires
RUN mkdir -p certs static/uploads

# Construire l'application
RUN go build -o forum ./cmd/main.go

# Exposer le port 8085 (HTTP) et 8443 (HTTPS)
EXPOSE 8085 8443

# Commande de démarrage
CMD ["./forum"]