# Etapa 1: compilar. Esta imagen sí tiene el compilador de Go, y no viaja
# a producción — solo lo que copiemos en la etapa 2.
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Primero solo los archivos de dependencias: si no cambian, Docker reusa
# esta capa en el siguiente build y no vuelve a descargar nada.
COPY go.mod go.sum ./
RUN go mod download

# Recién ahora el resto del código.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /servidor .

# Etapa 2: la imagen que de verdad corre. Sin compilador, sin código
# fuente: solo el binario ya compilado. Mucho más chica y con menos
# superficie de ataque.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /servidor /app/servidor

EXPOSE 8080
ENTRYPOINT ["/app/servidor"]
