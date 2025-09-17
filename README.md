# 🌙 Alucinari - Jogo de Cartas Multiplayer Online

> *"Em Alucinari, seu único objetivo é se manter são num delirante mundo de psicodelia. Com cartas estranhas, que lhe conferem poderes oníricos cada vez mais alucinantes, você precisa derrotar inimigos um por um, fazendo-os acordar... ou se perder mais ainda nesse ilógico labirinto. Cada novo minuto é um novo sonho a se desdobrar. Cada passo dado lhe faz perceber a crua realidade: em Alucinari, não há saída, apenas o próximo duelo."*

## 📖 Sobre o Jogo

Alucinari é um jogo de cartas multiplayer baseado em turnos onde dois jogadores batalham em um mundo onírico surreal. O objetivo é reduzir a sanidade do oponente a zero enquanto mantém a sua própria sanidade acima de zero.

### 🎮 Mecânicas de Jogo

- **Sanidade**: Cada jogador começa com 40 pontos de sanidade
- **Estados de Sonho**: Os jogadores podem estar em diferentes estados mentais que afetam o gameplay:
  - 😴 **Adormecido**: Estado padrão, perde 3 pontos de sanidade por turno
  - 😎 **Consciente**: Recupera 1 ponto de sanidade por turno (dura 2 turnos)
  - 🚫 **Paralisado**: Perde o turno (dura 1 turno)
  - 😱 **Assustado**: Perde 4 pontos de sanidade por turno (dura 2 turnos)

### 🃏 Tipos de Cartas

- **REM**: Cartas que causam dano ao oponente
- **NREM**: Cartas que causam dano ao oponente
- **Pill**: Cartas que curam o próprio jogador

### ⭐ Raridades das Cartas

- **Comum**: 50% das cartas nos boosters
- **Incomum**: 40% das cartas nos boosters
- **Rara**: 10% das cartas nos boosters

## 📋 Pré-requisitos

Antes de começar, você precisa instalar:

### Go
- **Versão**: 1.19 ou superior
- **Download**: [https://golang.org/](https://golang.org/)
- **Documentação**: [https://golang.org/doc/](https://golang.org/doc/)

### Docker
- **Download**: [https://www.docker.com/](https://www.docker.com/)
- **Documentação**: [https://docs.docker.com/](https://docs.docker.com/)

## 🏗️ Estrutura do Projeto

```
alucinari/
├── server/
│   ├── server.go
│   ├── types.go
│   ├── cardVault.go
│   ├── handlers.go
│   ├── matchManager.go
│   ├── playerManager.go
│   └── data/
│       └── cardVault.json
├── client/
│   └── client.go
├── bots.go
└── docker-compose.yml
```

## 🚀 Como Executar

### 🐳 Opção 1: Usando Docker (Recomendado)

1. **Clone o repositório**:
   ```bash
   git clone <url-do-repositorio>
   cd alucinari
   ```

2. **Execute com Docker Compose**:
   ```bash
   docker-compose up --build
   ```

3. **Para conectar mais clientes**, em terminais separados:
   ```bash
   docker-compose run client
   ```

### 🖥️ Opção 2: Executando no Terminal

1. **Execute o servidor**:
   ```bash
   cd server
   go run *.go
   ```

2. **Em outro terminal, execute o cliente**:
   ```bash
   cd client
   go run client.go
   ```

3. **Para mais clientes**, repita o passo 2 em novos terminais.

## 🎯 Como Jogar

### 📝 Menu Principal

1. **Registrar**: Crie uma nova conta
2. **Login**: Entre com uma conta existente
3. **Comprar booster**: Adquira novos pacotes de cartas
4. **Ver inventário**: Visualize suas cartas
5. **Batalhar**: Entre na fila de matchmaking
6. **Ping**: Teste a latência com o servidor

### ⚔️ Durante a Batalha

- Você recebe 10 cartas aleatórias do seu inventário
- No seu turno, escolha uma carta pelo número (1-10)
- Digite `gv` para desistir da partida
- Monitore sua sanidade e estado de sonho
- Vença reduzindo a sanidade do oponente a zero!

## 🤖 Testando com Bots

Para testar o servidor com múltiplos jogadores automatizados:

1. **Execute o servidor** (Docker ou terminal)

2. **Execute os bots**:
   ```bash
   go run bots.go
   ```

3. **Personalize a quantidade de bots**:
   - Edite a constante `NUMBOTS` no arquivo `bots.go`
   - Ou use variável de ambiente:
     ```bash
     NUM_BOTS=100 go run bots.go
     ```

### 🔧 Configurações dos Bots

- **Padrão**: 500 bots simultâneos
- **Comportamento**: Registram automaticamente, compram boosters e batalham
- **Estratégia**: Jogam sempre a primeira carta da mão
- **Conexão**: Aguardam 200ms * ID antes de conectar (evita sobrecarga)

## 🌐 Configurações de Rede

### Portas Utilizadas

- **8080/TCP**: Comunicação principal do jogo
- **8081/UDP**: Teste de latência (ping)

### Variáveis de Ambiente

- `SERVER_ADDR`: Endereço do servidor (padrão: `:8080`)
- `PORT`: Porta do servidor (padrão: `8080`)
- `NUM_BOTS`: Quantidade de bots para teste

## 🏆 Estratégias de Vitória

1. **Gerencie sua sanidade**: Use cartas Pill quando necessário
2. **Controle os estados**: Cartas com efeitos podem mudar o rumo da partida
3. **Timing é crucial**: Alguns estados duram múltiplos turnos
4. **Diversifique seu deck**: Tenha diferentes tipos de cartas disponíveis

## 🔧 Desenvolvimento

### Adicionar Novas Cartas

Edite o arquivo `server/data/cardVault.json` seguindo a estrutura:

```json
{
  "cards": {
    "CARD_ID": {
      "name": "Nome da Carta",
      "CID": "CARD_ID",
      "desc": "Descrição da carta",
      "cardtype": "rem|nrem|pill",
      "cardrarity": "comum|incomum|rara",
      "cardeffect": "adormecido|consciente|paralisado|assustado|nenhum",
      "points": 0
    }
  }
}
```

### Logs do Servidor

O servidor exibe estatísticas a cada 2 segundos:
- Jogadores inscritos
- Jogadores online
- Estoque de boosters
- Partidas ativas

## 🐛 Troubleshooting

### Problemas Comuns

1. **"Erro ao criar estoque"**: Verifique se `cardVault.json` existe e está válido
2. **"Usuário já logado"**: Um player só pode ter uma sessão ativa
3. **"Timeout"**: Verifique a conectividade de rede
4. **Docker não inicia**: Certifique-se que as portas 8080 e 8081 estão livres

### Logs Úteis

- Servidor: Mostra conexões, matchmaking e estatísticas
- Cliente: Exibe estado do jogo e notificações
- Bots: Prefixo `[Bot ID - Nome]` para cada ação

## 📚 Arquitetura Técnica

- **Linguagem**: Go 1.19+
- **Comunicação**: TCP com JSON
- **Concorrência**: Goroutines para cada cliente e partida
- **Sincronização**: Mutexes para thread-safety
- **Containerização**: Docker com multi-stage builds

---

*Mergulhe no mundo surreal de Alucinari e teste sua sanidade contra outros jogadores! 🎭✨*
