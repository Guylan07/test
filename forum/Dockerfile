FROM golang:1.21-alpine

WORKDIR /app

# Installer les dépendances nécessaires
RUN apk add --no-cache gcc musl-dev

# Copier les fichiers de dépendances et télécharger les modules
COPY go.mod go.sum ./
RUN go mod download

# Copier le code source
COPY . .

# Construire l'application
RUN go build -o forum ./cmd/main.go

# Exposer le port 8085 (HTTP) et 8443 (HTTPS)
EXPOSE 8085 8443

# Commande de démarrage
CMD ["./forum"]