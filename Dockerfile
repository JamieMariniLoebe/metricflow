FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o metricflow ./cmd/metricflow

FROM scratch

COPY --from=builder /app/metricflow /metricflow
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

ENTRYPOINT [ "/metricflow" ]