# syntax=docker/dockerfile:1

# 1) Cliente: build estático do Vite (Babylon.js + TS)
FROM node:22-slim AS client
WORKDIR /app/client
COPY client/package.json client/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY client/ ./
# O build gera public/novidades.json a partir do changelog da raiz.
COPY CHANGELOG.md /app/CHANGELOG.md
# Commit do deploy, gravado no cliente para os relatórios de erro (a Railway passa as
# variáveis do serviço como build args; o contexto não leva o .git).
ARG RAILWAY_GIT_COMMIT_SHA
# Os sourcemaps saem da pasta servida e ficam só neste estágio, em /app/sourcemaps (para
# traduzir stacks: ver README, "Erros do cliente"). A imagem final não leva nenhum .map.
RUN npm run build \
 && find dist -name '*.map' -exec sh -c 'for f; do d="/app/sourcemaps/${f#dist/}"; mkdir -p "${d%/*}"; mv "$f" "$d"; done' sh {} + \
 && test -z "$(find dist -name '*.map')"

# 2) Servidor Go: binário estático
FROM golang:1.26 AS server
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/zone ./cmd/zone

# 3) Imagem final mínima: só o binário e os arquivos do cliente
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=server /out/zone /app/zone
COPY --from=client /app/client/dist /app/public
ENV STATIC_DIR=/app/public
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/app/zone"]
