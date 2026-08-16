FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /kata-exporter ./cmd/kata-exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /kata-exporter /usr/local/bin/kata-exporter
EXPOSE 9788
ENTRYPOINT ["/usr/local/bin/kata-exporter","serve","--config",""]

