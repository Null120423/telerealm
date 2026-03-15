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

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/server ./server
COPY --from=builder /app/static ./static

ENV GIN_MODE=release

EXPOSE 7777

CMD ["./server"]