FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o aggregatorIB ./main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/aggregatorIB /app/aggregatorIB
COPY --from=builder /app/static /app/static
EXPOSE 8080
CMD ["/app/aggregatorIB"]
