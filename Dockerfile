FROM --platform=$BUILDPLATFORM golang:1.23.7-alpine3.21 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
	go build -trimpath -ldflags="-s -w" -o /app/server ./main.go

RUN mkdir -p /app/storage/multipart/sessions /app/storage/multipart/files

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder --chown=nonroot:nonroot /app/server ./server
COPY --from=builder --chown=nonroot:nonroot /app/static ./static
COPY --from=builder --chown=nonroot:nonroot /app/storage ./storage

ENV GIN_MODE=release
ENV MULTIPART_STORAGE_DIR=/app/storage/multipart

EXPOSE 7777

CMD ["./server"]