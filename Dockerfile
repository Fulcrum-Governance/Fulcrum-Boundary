# Boundary links the Postgres SQL classifier (pganalyze/pg_query_go) via cgo,
# so the binary MUST be built with CGO_ENABLED=1 and a C toolchain present.
# A CGO_ENABLED=0 build fails with "undefined: pg_query.Parse".
FROM golang:1.25-alpine AS build

# C toolchain for cgo (gcc/musl headers) required by the SQL classifier.
RUN apk add --no-cache build-base

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /out/boundary ./cmd/boundary

FROM alpine:3.20

# musl runtime libs for the cgo binary.
RUN apk add --no-cache libc6-compat \
  && adduser -D -H boundary \
  && mkdir -p /var/lib/boundary \
  && chown boundary /var/lib/boundary
COPY --from=build /out/boundary /usr/local/bin/boundary
USER boundary

# serve writes decision records by default; / is not writable by the boundary
# user, so point them at a state directory it owns. Mount a volume there to
# keep them, or pass --no-records.
ENTRYPOINT ["boundary", "serve", "--record-dir", "/var/lib/boundary/serve"]
