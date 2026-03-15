# Stage 1: Build Go app
FROM golang:1.22-alpine AS builder

WORKDIR /app

# copy go mod trước để cache
COPY go.mod go.sum ./
RUN go mod download

# copy source
COPY . .

# build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app

# Stage 2: chạy app
FROM alpine:latest

WORKDIR /root/

# copy binary từ stage builder
COPY --from=builder /app/app .

# expose port
EXPOSE 7777

# run app
CMD ["./app"]