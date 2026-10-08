# Diagrams

UML diagrams in [PlantUML](https://plantuml.com): an activity diagram for one run and a package
diagram for the imports. `style.iuml` holds the shared look; every diagram is rendered twice,
light and dark, and the docs pick one with `prefers-color-scheme`.

After editing a `.puml` file, re-render from this directory with the
[PlantUML jar](https://plantuml.com/download) (needs Java, no Graphviz):

```bash
for f in flow packages; do
  java -jar plantuml.jar -tsvg -pipe < $f.puml > $f.svg
  java -jar plantuml.jar -DDARK=1 -tsvg -pipe < $f.puml > $f-dark.svg
done
```
