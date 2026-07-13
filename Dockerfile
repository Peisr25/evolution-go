FROM golang:1.25.0-alpine AS build

RUN apk update && apk add --no-cache git build-base libjpeg-turbo-dev libwebp-dev

WORKDIR /build

# Copiar apenas arquivos de dependências primeiro para cachear o download
COPY go.mod go.sum ./

# Clonar whatsmeow-lib diretamente (submodule não é inicializado pelo Railway)
# Fork Peisr25 = EvolutionAPI/whatsmeow + merge do tulir/main (tctoken exigido
# pelo servidor no group create desde ~jun/2026; pin antigo 0923702 ficou mudo)
RUN git clone https://github.com/Peisr25/whatsmeow.git whatsmeow-lib && \
    cd whatsmeow-lib && \
    git checkout d9d7265ca740289bdd055f5d81bd6c0912819c66

# Download best-effort (novas deps transitivas do whatsmeow entram via go mod tidy)
RUN go mod download || true

# Copiar o restante do código
COPY . .

ARG VERSION=dev
RUN go mod tidy && CGO_ENABLED=1 go build -ldflags "-X main.version=${VERSION}" -o server ./cmd/evolution-go

FROM alpine:3.19.1 AS final

RUN apk update && apk add --no-cache tzdata ffmpeg libjpeg-turbo libwebp

WORKDIR /app

COPY --from=build /build/server .
COPY --from=build /build/manager/dist ./manager/dist
COPY --from=build /build/manager/dashboard ./manager/dashboard
COPY --from=build /build/VERSION ./VERSION

ENV TZ=America/Sao_Paulo

ENTRYPOINT ["/app/server"]
