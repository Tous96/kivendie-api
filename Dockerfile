# Étape 1 : build
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build -o main .

# Étape 2 : exécution
FROM gcr.io/distroless/static
COPY --from=builder /app/main /
ENTRYPOINT ["/main"]
