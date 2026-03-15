# Stage 1: Build Go app
FROM golang:1.23.7 AS builder

WORKDIR /app

# copy go mod để cache
COPY go.mod go.sum ./
RUN go mod download

# copy source code
COPY . .

# build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app

# Stage 2: Ubuntu 22.04 runtime
FROM ubuntu:22.04

WORKDIR /app

# cài cert để https hoạt động
RUN apt update && apt install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# copy binary
COPY --from=builder /app/app .

# expose port
EXPOSE 7777

# run app
CMD ["./app"]