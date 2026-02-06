FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/dashboard ./cmd/dashboard

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/dashboard /app/dashboard
COPY web /app/web

USER app
EXPOSE 8080
ENTRYPOINT ["/app/dashboard"]
