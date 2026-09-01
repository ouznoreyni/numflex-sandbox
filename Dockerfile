# syntax=docker/dockerfile:1

# ─────────────────────────────── compilation ────────────────────────────────
# La version suit la directive `go` du go.mod — pgx v5.10 exige 1.25. Les
# désaligner casse le build de l'image sans casser le build local.
ARG GO_VERSION=1.25

# --platform=$BUILDPLATFORM : la compilation reste native, la cible est obtenue
# par GOOS/GOARCH. Construire une image arm64 depuis amd64 ne passe donc pas
# par l'émulation QEMU, qui coûte dix fois le temps de build.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Les dépendances sont copiées seules : tant que go.mod et go.sum ne bougent
# pas, cette couche est réutilisée telle quelle.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
# CGO_ENABLED=0    : aucune dépendance C — pgx est en Go pur — donc un binaire
#                    statique, exécutable depuis une image `scratch`.
# -trimpath        : retire les chemins de la machine de build, le binaire
#                    devient reproductible.
# -ldflags='-s -w' : retire la table des symboles et les infos DWARF, ~25 % de
#                    moins. Le prix : plus de trace de pile symbolisée — le
#                    sandbox n'est pas un service de production.
# -tags timetzdata : embarque la base des fuseaux (~450 Ko). Sans elle, sur une
#                    image sans /usr/share/zoneinfo, un TZ=Africa/Dakar serait
#                    ignoré en silence et les horodatages sortiraient en UTC.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -tags timetzdata -ldflags='-s -w' -o /out/ ./cmd/...

# Un utilisateur non privilégié : `scratch` n'a pas de /etc/passwd, et certains
# chemins de pgx interrogent l'utilisateur courant quand l'URL n'en porte pas.
RUN printf 'numflex:x:10001:10001::/app:/sbin/nologin\n' > /out/passwd && \
    printf 'numflex:x:10001:\n'                          > /out/group

# ──────────────────────────────── exécution ─────────────────────────────────
# `scratch` plutôt qu'alpine : rien à part ce qui est listé ci-dessous n'entre
# dans l'image — pas de busybox, pas de gestionnaire de paquets, donc aucune
# CVE de base à suivre et rien à exécuter pour qui obtiendrait le conteneur.
FROM scratch AS runtime

# Les racines de confiance, sans lesquelles un DATABASE_URL en
# `sslmode=verify-full` échouerait. ~250 Ko, le seul poids non négociable.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/passwd /out/group /etc/

COPY --from=build /out/server /out/artp /usr/local/bin/
# Le serveur est seul propriétaire du schéma : il joue les migrations au
# démarrage et les cherche en remontant depuis le répertoire courant, d'où le
# WORKDIR ci-dessous.
COPY migrations /app/migrations

WORKDIR /app
USER 10001:10001
EXPOSE 8080

# Aucune variable de configuration n'est posée ici, volontairement : un `ENV
# PORT=8080` masquerait la valeur d'un .env monté, puisque l'environnement
# l'emporte sur le fichier.
#
# Aucun HEALTHCHECK non plus : le sandbox n'expose aucune route de santé — la
# plateforme réelle n'en a pas, et la surface doit rester identique — et
# `scratch` n'offre aucun shell pour en éprouver une. À sonder de l'extérieur,
# par POST /api/authenticate.
ENTRYPOINT ["/usr/local/bin/server"]

LABEL org.opencontainers.image.title="numflex-sandbox" \
      org.opencontainers.image.description="Double local de l'API Gateway NumFlex de l'ARTP (guide v2)" \
      org.opencontainers.image.source="https://github.com/yas/numflex-sandbox"
