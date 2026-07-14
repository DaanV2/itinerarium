# Single-image build: the SvelteKit site is compiled to a static SPA and
# embedded into the Go binary, so the runtime stage ships one executable.

# Web build — adapter-static writes into the Go module (api/infrastructure/webapp/dist)
FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# API build — pure-Go SQLite driver, so CGO stays off and the image is static
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
COPY --from=web /src/api/infrastructure/webapp/dist ./infrastructure/webapp/dist
RUN CGO_ENABLED=0 go build -tags embedweb -o /itinerarium .

# Runtime stage
FROM alpine:3.22
RUN adduser -D -H itinerarium
WORKDIR /app
COPY --from=build /itinerarium /app/itinerarium
# WORKDIR/COPY default to root ownership; the app user needs write access to
# persist the SQLite DB and JWT signing keys under the mounted volume.
RUN mkdir -p /app/data && chown -R itinerarium:itinerarium /app && chmod 0750 /app/data
USER itinerarium
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/itinerarium"]
CMD ["serve"]
