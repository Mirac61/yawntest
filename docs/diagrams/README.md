# Diagrams

Written in [D2](https://d2lang.com). The docs embed the SVGs, which switch between a light
and a dark theme with the viewer's color scheme. After editing a `.d2` file, re-render from
the repo root:

```bash
for f in docs/diagrams/*.d2; do
  go run oss.terrastruct.com/d2@v0.7.1 --layout elk --theme 0 --dark-theme 200 --pad 24 "$f" "${f%.d2}.svg"
done
```
