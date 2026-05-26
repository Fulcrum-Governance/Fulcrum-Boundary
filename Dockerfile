FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/boundary ./cmd/boundary

FROM alpine:3.20

RUN adduser -D -H boundary
COPY --from=build /out/boundary /usr/local/bin/boundary
USER boundary

ENTRYPOINT ["boundary", "serve"]
