# Utilisez une image de Go officielle pour compiler votre application
FROM golang:1.21 as builder

# Répertoire de travail pour la compilation
WORKDIR /app

# Copiez les fichiers go.mod et go.sum
COPY go.mod go.sum ./

# Téléchargez toutes les dépendances
RUN go mod download

# Copiez le reste du code source de l'application
COPY . .

# Compilez l'application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main .

# Utilisez une image de base légère pour l'exécution
FROM alpine:latest

# Installez les certificats CA pour permettre des appels HTTPS
RUN apk --no-cache add ca-certificates

# Variables d'environnement par défaut (peuvent être remplacées lors de l'exécution du conteneur)
ENV INITIAL_PROTO=https
ENV INITIAL_URI=monsite.com
ENV FINAL_URL=monsitecached.com
ENV FINAL_PROTO=http
ENV JPEG_QUALITY=30

# Définir le répertoire de travail
WORKDIR /root/

# Copiez l'exécutable binaire depuis l'étape de construction
COPY --from=builder /app/main .

# Exposez le port sur lequel votre application s'exécutera (modifiez si nécessaire)
EXPOSE 80

# Commande pour exécuter l'application
CMD ["./main"]