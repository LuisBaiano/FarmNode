# Usa a imagem oficial do Go
FROM golang:1.23

# Define a pasta de trabalho dentro do contêiner
WORKDIR /app

# Copia os arquivos de dependência e baixa
COPY go.mod ./
# Se você tiver criado o go.sum, descomente a linha abaixo:
COPY go.sum ./
RUN go mod download

# Copia todo o código do projeto para dentro do contêiner
COPY . .

# Compila o executável do servidor
RUN go build -o server_exec ./cmd/server

# Expõe a porta UDP 8080
EXPOSE 8080/udp

# Comando que roda quando o contêiner liga
CMD ["./server_exec"]