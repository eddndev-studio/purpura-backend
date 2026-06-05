# Build multi-stage: compila un binario estatico y lo copia a una imagen minima.
# tzdata va embebido en el binario (import _ "time/tzdata"), por eso la imagen
# final 'static' no necesita la base de zonas horarias del sistema.

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/purpura-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/purpura-api /purpura-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/purpura-api"]
