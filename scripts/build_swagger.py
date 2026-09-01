#!/usr/bin/env python3
"""Régénère docs/openapi.json et docs/swagger.html depuis docs/openapi.yaml.

openapi.yaml est la seule source. La spécification est *inlinée* dans la page
plutôt que chargée par fetch : une page servie sur un port et une spec sur un
autre déclencheraient une requête cross-origin, et le sandbox n'envoie aucun
en-tête CORS — comme la plateforme réelle.
"""
import json
import pathlib

import yaml

RACINE = pathlib.Path(__file__).resolve().parent.parent
DOCS = RACINE / "docs"

GABARIT = """<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>API Gateway NumFlex — sandbox local</title>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui.min.css">
<style>
  body {{ margin: 0; background: #fafafa; }}
  .rappel {{ font: 14px/1.5 system-ui, -apple-system, sans-serif; background: #1b1b1f; color: #e8e8ea;
            padding: 14px 20px; }}
  .rappel code {{ background: rgba(255,255,255,.12); padding: 1px 5px; border-radius: 3px; }}
</style>
</head>
<body>
<div class="rappel">
  Sandbox sur <code>http://localhost:8080</code> — comptes <code>yas/yas2026</code>,
  <code>orange/orange2026</code>, <code>expresso/expresso2026</code>. Cette page est servie hors
  de la gateway : le sandbox n'expose que les 33 routes du contrat.
  <strong>« Try it out » depuis cette page échouera</strong> — le sandbox n'envoie pas d'en-têtes CORS,
  comme la plateforme réelle. Utilisez la collection Postman ou <code>curl</code> pour appeler.
</div>
<div id="swagger-ui"></div>
<script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui-bundle.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui-standalone-preset.min.js"></script>
<script>
const spec = {spec};
window.ui = SwaggerUIBundle({{
  spec: spec,
  dom_id: '#swagger-ui',
  deepLinking: true,
  docExpansion: 'list',
  defaultModelsExpandDepth: 0,
  presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
  layout: 'BaseLayout'
}});
</script>
</body>
</html>
"""


def main() -> None:
    spec = yaml.safe_load((DOCS / "openapi.yaml").read_text(encoding="utf-8"))
    brut = json.dumps(spec, ensure_ascii=False, indent=1)
    (DOCS / "openapi.json").write_text(brut, encoding="utf-8")
    (DOCS / "swagger.html").write_text(GABARIT.format(spec=brut), encoding="utf-8")

    operations = sum(
        len([m for m in v if m in ("get", "post", "put", "delete", "patch")])
        for v in spec["paths"].values()
    )
    print(f"openapi.json et swagger.html régénérés — "
          f"{len(spec['paths'])} chemins, {operations} opérations")


if __name__ == "__main__":
    main()
