# build stage
FROM golang:1.25-alpine AS build

WORKDIR /build

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o corsair -ldflags "-s -w -X main.version=${VERSION}" ./cmd/

# final image
FROM gcr.io/distroless/static

USER nonroot:nonroot

COPY --from=build --chown=nonroot:nonroot /build/corsair /corsair

ENTRYPOINT ["/corsair"]