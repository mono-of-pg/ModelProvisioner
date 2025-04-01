# Build stage
FROM golang:1.21 as builder
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o app

# Final stage
FROM gcr.io/distroless/base
COPY --from=builder /app/app /app
USER 1000
CMD ["/app"]
