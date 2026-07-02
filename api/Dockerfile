# Build stage — pure-Go SQLite driver, so CGO stays off and the image is static
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /itinerarium .

# Runtime stage
FROM alpine:3.22
RUN adduser -D -H itinerarium
USER itinerarium
WORKDIR /app
COPY --from=build /itinerarium /app/itinerarium
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/itinerarium"]
CMD ["serve"]
