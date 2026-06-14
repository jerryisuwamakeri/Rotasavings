# ROTASAVINGS backend — multi-stage build producing a small static image.

# --- build stage ---
FROM golang:1.26 AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

# Build.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/rota ./cmd/server

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/rota /app/rota

# By default the SQLite cache writes to /app/data; mount a volume there, or set
# ROTA_POSTGRES_DSN to use Postgres instead.
ENV ROTA_HTTP_ADDR=":8080" \
    ROTA_DB_PATH="/app/data/rota.db"
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/rota"]
