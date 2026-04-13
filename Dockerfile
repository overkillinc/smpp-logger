FROM golang:1.26 AS build

WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/smpp-logger ./cmd/smpp-logger

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/smpp-logger /smpp-logger

EXPOSE 2775

ENTRYPOINT ["/smpp-logger"]
