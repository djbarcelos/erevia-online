# syntax=docker/dockerfile:1

# 1) Cliente: build estático do Vite (Babylon.js + TS)
FROM node:22-slim AS client
WORKDIR /app/client
COPY client/package.json client/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY client/ ./
RUN npm run build

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
