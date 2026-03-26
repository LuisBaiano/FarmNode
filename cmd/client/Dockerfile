FROM golang:1.23

WORKDIR /app

COPY go.mod ./
# COPY go.sum ./
RUN go mod download

COPY . .

# Compila o executável do cliente
RUN go build -o client_exec ./cmd/client

# Expõe a porta TCP 9000
EXPOSE 9000/tcp

CMD ["./client_exec"]