FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /hookrelay ./cmd/hookrelay
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S app -G app
COPY --from=build /hookrelay /usr/local/bin/hookrelay
USER app
EXPOSE 8090
ENTRYPOINT ["hookrelay"]
