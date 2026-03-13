FROM --platform=linux/amd64 golang:1.25-bookworm AS builder

WORKDIR /src

RUN curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 -o /usr/local/bin/tailwindcss \
    && chmod +x /usr/local/bin/tailwindcss

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN tailwindcss -i static/css/input.css -o static/css/app.css --minify
RUN CGO_ENABLED=1 go build -o /banki ./cmd/banki/

FROM --platform=linux/amd64 debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /banki /usr/local/bin/banki

WORKDIR /app
EXPOSE 8080
CMD ["banki"]
