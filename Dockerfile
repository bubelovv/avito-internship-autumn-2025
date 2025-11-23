FROM golang:1.25.0 AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pr-reviewer ./cmd/app

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /src/pr-reviewer .
EXPOSE 8080
ENTRYPOINT ["./pr-reviewer"]