# syntax=docker/dockerfile:1.7

# ----- Go: ctf-evals binary + bundled toy engine ------------------------------
FROM golang:1.25-alpine AS go-build
WORKDIR /src
RUN apk add --no-cache make
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /out/ctf-evals ./cmd/ctf-evals
RUN make -C challenges/regex-count all

# ----- Web: Vite SPA bundle ---------------------------------------------------
FROM node:22-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ----- Runtime ---------------------------------------------------------------
FROM alpine:3.21
RUN apk add --no-cache bash ca-certificates tzdata
WORKDIR /app
COPY --from=go-build /out/ctf-evals /app/ctf-evals
COPY --from=go-build /src/challenges /app/challenges
COPY --from=web-build /web/dist /app/web/dist
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh
ENV CTF_EVALS_DB="sqlite:///app/data/ctf-evals.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" \
    CTF_EVALS_CACHE_DIR=/app/data/cache \
    CTF_EVALS_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["serve"]
