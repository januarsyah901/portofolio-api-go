# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/admin ./cmd/admin \
 && mkdir -p /out/public/assets/uploads

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /out/admin /app/
COPY --from=build --chown=nonroot:nonroot /out/public /app/public
ENV PORT=3001 PUBLIC_DIR=/app/public APP_ENV=production LOG_FORMAT=json
EXPOSE 3001
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
