CREATE TABLE operateur (
    id              TEXT PRIMARY KEY,
    nom             TEXT NOT NULL UNIQUE,
    prefixe_routage TEXT NOT NULL
);

CREATE TABLE utilisateur (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    operateur_id  TEXT NOT NULL REFERENCES operateur(id),
    roles         TEXT[] NOT NULL
);

CREATE TABLE motif_rejet (
    id    TEXT PRIMARY KEY,
    motif TEXT NOT NULL
);

CREATE TABLE type_demande (
    id   TEXT PRIMARY KEY,
    type TEXT NOT NULL UNIQUE
);

CREATE TABLE processus (
    id   TEXT PRIMARY KEY,
    type TEXT NOT NULL UNIQUE
);

CREATE TABLE type_incident (
    id           TEXT PRIMARY KEY,
    libelle      TEXT NOT NULL,
    fige_systeme BOOLEAN NOT NULL
);

-- Registre national des numéros. C'est lui qui rend calculables
-- DELAI_PORTAGE_NON_RESPECTE, NUMERO_NON_PORTE, OPERATEUR_SOURCE_INCORRECT
-- et NUMERO_DEJA_CHEZ_DESTINATAIRE.
CREATE TABLE numero (
    msisdn               TEXT PRIMARY KEY,
    operateur_actuel_id  TEXT NOT NULL REFERENCES operateur(id),
    operateur_origine_id TEXT NOT NULL REFERENCES operateur(id),
    date_dernier_portage TIMESTAMPTZ,
    deja_restitue        BOOLEAN NOT NULL DEFAULT FALSE,
    actif                BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE demande (
    id                        TEXT PRIMARY KEY,
    numero                    TEXT NOT NULL,
    type_abonne               TEXT NOT NULL,
    type_demande              TEXT NOT NULL,
    statut_demande            TEXT NOT NULL,
    etape_actuelle            TEXT NOT NULL,
    statut_etape_actuel       TEXT NOT NULL,
    operateur_source_id       TEXT NOT NULL REFERENCES operateur(id),
    operateur_destinataire_id TEXT NOT NULL REFERENCES operateur(id),
    createur_operateur_id     TEXT NOT NULL REFERENCES operateur(id),
    processus                 TEXT,
    routage_info              TEXT,
    date_demande              TIMESTAMPTZ NOT NULL,
    date_debut_etape          TIMESTAMPTZ NOT NULL,
    transition_prevue_a       TIMESTAMPTZ,
    date_finalisation         TIMESTAMPTZ,
    motif_rejet_id            TEXT REFERENCES motif_rejet(id),
    commentaire               TEXT
);

CREATE INDEX demande_etape_idx ON demande (statut_demande, etape_actuelle);
CREATE INDEX demande_numero_idx ON demande (numero);

CREATE TABLE demande_numero (
    demande_id            TEXT NOT NULL REFERENCES demande(id) ON DELETE CASCADE,
    numero                TEXT NOT NULL,
    statut                TEXT NOT NULL,
    motif_rejet_id        TEXT REFERENCES motif_rejet(id),
    exclu                 BOOLEAN NOT NULL DEFAULT FALSE,
    raison_exclusion      TEXT,
    code_erreur_exclusion TEXT,
    routage_info          TEXT,
    PRIMARY KEY (demande_id, numero)
);

CREATE TABLE demande_client (
    demande_id     TEXT PRIMARY KEY REFERENCES demande(id) ON DELETE CASCADE,
    nom            TEXT,
    prenom         TEXT,
    date_naissance DATE,
    lieu_naissance TEXT,
    type_piece     TEXT,
    numero_piece   TEXT,
    raison_sociale TEXT,
    num_rc         TEXT
);

CREATE TABLE etape_historique (
    id           BIGSERIAL PRIMARY KEY,
    demande_id   TEXT NOT NULL REFERENCES demande(id) ON DELETE CASCADE,
    etape        TEXT NOT NULL,
    statut       TEXT NOT NULL,
    operateur_id TEXT REFERENCES operateur(id),
    origine      TEXT NOT NULL,
    commentaire  TEXT,
    date_debut   TIMESTAMPTZ NOT NULL,
    date_fin     TIMESTAMPTZ
);

CREATE TABLE confirmation (
    demande_id   TEXT NOT NULL REFERENCES demande(id) ON DELETE CASCADE,
    operateur_id TEXT NOT NULL REFERENCES operateur(id),
    commentaire  TEXT,
    date_conf    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (demande_id, operateur_id)
);

CREATE TABLE otp (
    numero     TEXT PRIMARY KEY,
    code       TEXT NOT NULL,
    expire_a   TIMESTAMPTZ NOT NULL,
    tentatives INT NOT NULL DEFAULT 0,
    consomme   BOOLEAN NOT NULL DEFAULT FALSE,
    cree_le    TIMESTAMPTZ NOT NULL
);

CREATE TABLE reverse_request (
    id            TEXT PRIMARY KEY,
    numero        TEXT NOT NULL,
    operateur_id  TEXT NOT NULL REFERENCES operateur(id),
    statut        TEXT NOT NULL,
    date_demande  TIMESTAMPTZ NOT NULL,
    date_decision TIMESTAMPTZ,
    demande_id    TEXT REFERENCES demande(id)
);

CREATE TABLE incident (
    id                    TEXT PRIMARY KEY,
    operateur_id          TEXT NOT NULL REFERENCES operateur(id),
    type_incident_id      TEXT NOT NULL REFERENCES type_incident(id),
    -- Dénormalisé depuis type_incident : un index partiel ne peut pas suivre
    -- une jointure, et la contrainte du §7.12 ne vise que les incidents internes.
    fige_systeme          BOOLEAN NOT NULL,
    description           TEXT NOT NULL,
    statut                TEXT NOT NULL,
    date_ouverture        TIMESTAMPTZ NOT NULL,
    date_resolution       TIMESTAMPTZ,
    commentaire_resolution TEXT
);

-- Un seul incident INTERNE ouvert à la fois par opérateur — §7.12. Les incidents
-- gateway ne sont pas limités.
CREATE UNIQUE INDEX incident_interne_unique_ouvert
    ON incident (operateur_id)
    WHERE statut = 'EN_COURS' AND fige_systeme;
