#!/usr/bin/env python3
"""Regenerate docs/openapi.json and docs/swagger.html from docs/openapi.yaml.

openapi.yaml is the only source. The specification is *inlined* in the page
rather than loaded through fetch: a page served on one port and a spec on
another would trigger a cross-origin request, and the sandbox sends no CORS
header — like the real platform.
"""
import json
import pathlib

import yaml

ROOT = pathlib.Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"

TEMPLATE = """<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>API Gateway NumFlex — sandbox local</title>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui.min.css">
<style>
  body {{ margin: 0; background: #fafafa; }}
  .notice {{ font: 14px/1.5 system-ui, -apple-system, sans-serif; background: #1b1b1f; color: #e8e8ea;
            padding: 14px 20px; }}
  .notice code {{ background: rgba(255,255,255,.12); padding: 1px 5px; border-radius: 3px; }}
</style>
</head>
<body>
<div class="notice">
  Sandbox sur <code>http://localhost:8080</code> — comptes <code>yas/yas2026</code>,
  <code>orange/orange2026</code>, <code>expresso/expresso2026</code>.
  <strong>Cette page vit à la racine, hors de la gateway</strong> : <code>/api/gateway/v1</code>
  garde exactement les 33 routes du contrat, et n'en a gagné aucune. L'image
  <code>standalone</code> la sert sur le port de l'API ; l'image mince <code>:latest</code>
  n'embarque aucune page et répond <code>404</code> ici.
  <strong>« Try it out »</strong> fonctionne sans rien configurer : le CORS est ouvert à
  <code>*</code> par défaut. Poser <code>CORS_ALLOWED_ORIGINS=</code> — vide — rend au sandbox le
  silence de la plateforme réelle, et le navigateur bloquera alors l'appel : utilisez Postman ou
  <code>curl</code>.
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
    raw = json.dumps(spec, ensure_ascii=False, indent=1)
    (DOCS / "openapi.json").write_text(raw, encoding="utf-8")
    (DOCS / "swagger.html").write_text(TEMPLATE.format(spec=raw), encoding="utf-8")

    operations = sum(
        len([m for m in v if m in ("get", "post", "put", "delete", "patch")])
        for v in spec["paths"].values()
    )
    print(f"openapi.json and swagger.html regenerated — "
          f"{len(spec['paths'])} paths, {operations} operations")


if __name__ == "__main__":
    main()
