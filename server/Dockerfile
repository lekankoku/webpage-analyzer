FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /bin/web-analyzer ./cmd/app


FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /bin/web-analyzer /bin/web-analyzer

ENV WEB_ANALYZER_HTTP_ADDR=:8080 \
    WEB_ANALYZER_MAX_CONCURRENT_JOBS=10 \
    WEB_ANALYZER_JOB_TTL=1h \
    WEB_ANALYZER_SHUTDOWN_TIMEOUT=30s \
    WEB_ANALYZER_LINK_CHECKER_MAX_WORKERS=100 \
    WEB_ANALYZER_LINK_CHECKER_JOB_BUFFER_SIZE=500 \
    WEB_ANALYZER_LINK_CHECKER_TIMEOUT=10s \
    WEB_ANALYZER_LINK_CHECKER_RETRIES=2 \
    WEB_ANALYZER_PAGE_FETCH_TIMEOUT=10s

ENTRYPOINT ["/bin/web-analyzer"]
