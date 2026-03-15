FROM --platform=$BUILDPLATFORM golang:1.23.7-alpine3.21 AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -trimpath -ldflags="-s -w" -o server ./main.go

RUN mkdir -p /app/storage/multipart/sessions /app/storage/multipart/files

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/static ./static
COPY --from=builder /app/storage ./storage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENV GIN_MODE=release
ENV MULTIPART_STORAGE_DIR=/app/storage/multipart

EXPOSE 7777

CMD ["./server"]