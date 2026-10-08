# Renders a colored yawntest run to docs/demo.svg for the README.
# Usage, from the repo root (script keeps the colors, sed drops the ^D it echoes):
#   go install ./cmd/yawntest && cd _example && script -q /dev/null yawntest run ./... > ../run.ansi
#   rm shop/*_yawntest_test.go && cd .. && python3 docs/render-demo.py run.ansi docs/demo.svg && rm run.ansi
import re, sys, html
raw = open(sys.argv[1], encoding='utf-8', errors='replace').read()
raw = re.sub('\\^D|[\x04\x08]', '', raw).replace('\r', '')
lines = ['\x1b[35m$\x1b[0m yawntest run ./...'] + raw.rstrip('\n').split('\n')
colors = {'31': '#FF7A7A', '32': '#6FD99A', '33': '#F2C66D', '90': '#7E8599', '35': '#C9A7FF'}
fg = '#E6E6EB'
size, step, pad, top = 14, 21, 22, 54
width = int(max(len(re.sub('\x1b\\[[0-9]+m', '', l)) for l in lines) * size * 0.6 + 2 * pad)
height = top + len(lines) * step + pad - 4
out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">',
       f'<rect width="{width}" height="{height}" rx="10" fill="#1B1C24"/>',
       '<circle cx="22" cy="20" r="6" fill="#FF5F57"/><circle cx="42" cy="20" r="6" fill="#FEBC2E"/><circle cx="62" cy="20" r="6" fill="#28C840"/>',
       f'<g font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" font-size="{size}" fill="{fg}" xml:space="preserve" style="white-space: pre">']
for i, line in enumerate(lines):
    y = top + i * step
    spans, color = [], None
    for part in re.split('(\x1b\\[[0-9]+m)', line):
        m = re.fullmatch('\x1b\\[([0-9]+)m', part)
        if m:
            color = colors.get(m.group(1))
        elif part:
            text = html.escape(part).replace(' ', '\u00a0')
            spans.append(f'<tspan fill="{color}">{text}</tspan>' if color else text)
    n = len(re.sub('\x1b\\[[0-9]+m', '', line))
    fit = f' textLength="{n * size * 0.6:.1f}" lengthAdjust="spacingAndGlyphs"' if n else ''
    out.append(f'<text x="{pad}" y="{y}"{fit}>{"".join(spans)}</text>')
out.append('</g></svg>')
open(sys.argv[2], 'w').write('\n'.join(out) + '\n')
