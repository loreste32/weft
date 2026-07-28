FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /weft ./cmd/weft

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /weft /usr/local/bin/weft
ENTRYPOINT ["weft"]
CMD ["doctor"]
