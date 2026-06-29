# --- Stage 1: build the web SPA ---
FROM node:24-alpine AS web
RUN corepack enable
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# --- Stage 2: build the Go binary with the SPA embedded ---
FROM golang:1.26-alpine AS go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Embed the freshly built SPA.
COPY --from=web /app/web/dist/client/ internal/server/dist/
ARG VERSION=0.0.0-docker
RUN CGO_ENABLED=0 go build \
      -ldflags "-X github.com/nees/envvar/internal/version.Version=${VERSION}" \
      -o /out/envvar ./cmd/envvar

# --- Stage 3: minimal runtime ---
FROM gcr.io/distroless/static-debian12
COPY --from=go /out/envvar /usr/local/bin/envvar
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/envvar"]
CMD ["server", "--addr", ":8080"]
