FROM golang:1.22-alpine

# Install system dependencies
RUN apk add --no-cache \
    git \
    curl \
    bash \
    nodejs \
    npm

# Install pnpm
RUN npm install -g pnpm

# Set working directory
WORKDIR /jua

# Copy go.mod and go.sum first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the jua CLI
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/jua ./cmd/jua

# Expose ports used by generated Jua projects
# 8080 — Go API
# 3000 — Next.js web app
# 3001 — Next.js admin panel
EXPOSE 8080 3000 3001

CMD ["jua", "dev"]
