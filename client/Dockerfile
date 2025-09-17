# usa imagem do golang 1.25!! tem que ser 1.25
FROM golang:1.25

# diretorio de trabalho
WORKDIR /app

# copia o código
COPY . .

# compila
RUN go build -o client

# permissao de exec binário
RUN chmod +x ./client

# comando p iniciar client
CMD ["./client"]