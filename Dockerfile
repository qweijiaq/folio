# ---------- 前端构建 ----------
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---------- 后端构建 ----------
FROM golang:1.24-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w" -o /out/folio .

# ---------- 运行镜像 ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=server /out/folio /app/folio
COPY --from=web /app/web/dist /app/web/dist
RUN mkdir -p /app/data/uploads
ENV FOLIO_ADDR=:8080 \
    FOLIO_WEB=/app/web/dist \
    FOLIO_DB=/app/data/folio.db \
    FOLIO_UPLOAD_DIR=/app/data/uploads
EXPOSE 8080
CMD ["/app/folio"]
