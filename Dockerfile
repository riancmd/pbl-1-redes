# usa imagem do go
FROM golang:1.25

WORKDIR /app

# copia o código
COPY . .

# compila o arquivo de bots
RUN go build -o bots

# garente a permissão de exec binario
RUN chmod +x ./client

# comando p iniciar
CMD ["./botsgo"]