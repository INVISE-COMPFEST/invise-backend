FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o main ./cmd/api/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o migrate ./cmd/migrate/

FROM alpine:3.23

RUN apk --no-cache add ca-certificates curl tzdata

ENV TZ=Asia/Jakarta

RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app


COPY --from=builder /app/main .
COPY --from=builder /app/migrate .

COPY --from=builder /app/db/migrations ./db/migrations

# COPY --from=builder /app/resources ./resources

RUN mkdir -p logs assets/images && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

CMD ["./main"]