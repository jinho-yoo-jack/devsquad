FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/devsquad ./cmd/devsquad

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git bash && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/devsquad /usr/local/bin/devsquad
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["devsquad"]
