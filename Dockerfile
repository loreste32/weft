FROM golang:1.27-alpine AS build
WORKDIR /src
# Dependencies are pinned by go.sum (vendor/ is a local-only, gitignored
# convention in this repo — never COPY it; a clean clone has none).
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# -trimpath -buildvcs=false: byte-reproducible binary (see scripts/reproducible-build-check.sh)
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /weft ./cmd/weft
# Smoke inside the build stage: a broken binary must fail the image build.
RUN /weft version | grep -q "weft 0."

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /weft /usr/local/bin/weft
ENTRYPOINT ["weft"]
CMD ["doctor"]
