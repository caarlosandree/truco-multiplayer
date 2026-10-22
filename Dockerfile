# Imagem de produção: o Go serve a API, o WebSocket e o build do frontend (STATIC_DIR) na mesma origem.

FROM node:24-alpine AS web
WORKDIR /src/frontend
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

FROM golang:1.27-alpine AS api
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/truco ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 truco
COPY --from=api /out/truco /app/truco
COPY --from=web /src/frontend/dist /app/web
USER truco
ENV HTTP_ADDR=:8080 STATIC_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/app/truco"]
CMD ["serve"]
