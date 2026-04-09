# Dockerfile para cmd/simulador (sensores + atuadores)
FROM golang:1.23

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

# Compila o executável do simulador (sensores e atuadores)
RUN go build -o simulador_exec ./cmd/simulador

CMD ["./simulador_exec"]
