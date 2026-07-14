# Build stage. CGO is off and tzdata is compiled into the binary, so the
# result runs in an empty image.
FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .

ARG VERSION=docker
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/timetools ./cmd/timetools

# Runtime stage: just the binary. The binary is its own health probe
# because there is no shell or curl in here.
FROM scratch

COPY --from=build /out/timetools /timetools

USER 65534:65534
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s \
    CMD ["/timetools", "-health"]

ENTRYPOINT ["/timetools"]
