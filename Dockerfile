FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/server ./cmd/server && go build -o /out/artp ./cmd/artp

FROM alpine:3.20
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/artp /usr/local/bin/artp
COPY migrations /migrations
EXPOSE 8080
CMD ["server"]
