FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /gslb-switcher

FROM alpine:latest
WORKDIR /
COPY --from=builder /gslb-switcher .
ENTRYPOINT ["/gslb-switcher"]