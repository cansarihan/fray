FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/fray ./cmd/fray

FROM alpine:3.20
RUN adduser -D -u 10001 fray
COPY --from=build /out/fray /usr/local/bin/fray
COPY --from=build /src/testdata/kv.json /models/kv.json
USER fray
EXPOSE 7000 9100
# The demo target and the panel; fuzz the target the model describes.
ENTRYPOINT ["/usr/local/bin/fray"]
CMD ["serve", "-model", "/models/kv.json", "-target", "127.0.0.1:7000", "-panel", "0.0.0.0:9100"]
