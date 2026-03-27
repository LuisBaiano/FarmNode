# Dockerfile para cmd/client (sensores + atuadores)
FROM golang:1.23

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

# Compila o executável do client (sensores e atuadores)
RUN go build -o client_exec ./cmd/client

CMD ["./client_exec"]
