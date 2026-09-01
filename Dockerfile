# La version suit la directive `go` du go.mod — pgx v5.10 exige 1.25. Les
# désaligner casse le build de l'image sans casser le build local.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0 : l'image golang:alpine n'embarque pas de chaîne C, et aucune
# dépendance n'en a besoin (pgx est en Go pur).
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -o /out/artp ./cmd/artp

FROM alpine:3.20
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/artp /usr/local/bin/artp
COPY migrations /migrations
EXPOSE 8080
CMD ["server"]
