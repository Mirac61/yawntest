#!/bin/sh
# Renders every .puml here to one SVG that follows the viewer's light/dark mode:
# PlantUML draws the light colors from style.iuml, then a CSS block swaps them in dark mode.
# Usage: PLANTUML_JAR=/path/to/plantuml.jar ./render.sh
set -eu
cd "$(dirname "$0")"

# light color -> dark color, keep in sync with style.iuml
pairs="FFFFFF:1B1C24 1F2330:E6E6EB F4F5FA:2B2D3A 5B6275:9AA0B5 6B4FBB:C9A7FF"

css="@media (prefers-color-scheme: dark){"
for pair in $pairs; do
  light=${pair%%:*}
  dark=${pair##*:}
  css="$css[fill=\"#$light\"]{fill:#$dark}[style*=\"stroke:#$light\"]{stroke:#$dark !important}"
done
css="$css svg{background:#1B1C24 !important}}"

for source in *.puml; do
  target=${source%.puml}.svg
  java -jar "${PLANTUML_JAR:-plantuml.jar}" -tsvg -pipe < "$source" > "$target"
  # ELK draws package frames black whatever the style says, so they get the accent here.
  sed -i.bak -e "s|<defs/>|<defs><style>$css</style></defs>|" -e "s|stroke:#000000|stroke:#6B4FBB|g" "$target"
  rm "$target.bak"
done
