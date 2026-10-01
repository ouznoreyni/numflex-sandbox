-- Ordre de déclaration des numéros d'une flotte. La plateforme rend numeros[]
-- dans l'ordre reçu (capture du 2026-09-18) ; la clé (demande_id, numero)
-- seule ne le conserve pas.
ALTER TABLE demande_numero ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
