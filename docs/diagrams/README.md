# Diagrams

UML diagrams in [PlantUML](https://plantuml.com): an activity diagram for one run and a package
diagram for the imports. `style.iuml` holds the shared look. Each diagram is one SVG that
follows the viewer's light or dark mode through a small CSS block, so the docs embed it as a
plain Markdown image.

After editing a `.puml` file, re-render with the [PlantUML jar](https://plantuml.com/download)
(needs Java, no Graphviz):

```bash
PLANTUML_JAR=/path/to/plantuml.jar docs/diagrams/render.sh
```
