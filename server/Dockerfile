# Usa imagem do Go (ver 1.25)
FROM golang:1.25

WORKDIR /app 
#diretorio de trabalho

# copia o código
COPY . .

# compila o servidor
RUN go build -o server

# garante execução do binário
RUN chmod +x ./server

# portas usadas
# - 8080 pro tcp (conexão do jogo)
# - 8081 q é pro udp (teste de latência)
EXPOSE 8080
EXPOSE 8081

# comando para iniciar o servidor
CMD ["./server"]