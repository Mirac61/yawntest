# Renders colored yawntest runs to an SVG for the README, standard library only.
#
# One screen:   python3 docs/render-demo.py docs/demo.svg 'yawntest run ./...' run.ansi
# Animated:     python3 docs/render-demo.py docs/demo.svg 'cmd 1' one.ansi 'cmd 2' two.ansi ...
#
# With several screens the SVG plays them one after another and loops: each screen types its
# command, its output appears line by line, it holds, and the next one replaces it. CSS
# animations play in <img> tags, so GitHub shows it in the README.
#
# Record a screen with script, so the colors stay (stderr off, or the progress line ends up in it):
#   cd _example && script -q /dev/null -c 'yawntest run ./... 2>/dev/null' > ../run.ansi
import html
import re
import sys

colors = {'31': '#FF7A7A', '32': '#6FD99A', '33': '#F2C66D', '90': '#7E8599', '35': '#C9A7FF'}
fg, bg = '#E6E6EB', '#1B1C24'
size, step, pad, top = 14, 21, 22, 54
escape = re.compile('\x1b\\[[0-9]+m')

hold = 4.0       # seconds a finished screen stays
per_line = 0.12  # seconds between output lines
typing = 0.9     # seconds to "type" a command


def read(path):
    raw = open(path, encoding='utf-8', errors='replace').read()
    raw = re.sub('\\^D|[\x04\x08]', '', raw).replace('\r', '')
    return raw.rstrip('\n').split('\n')


def text(line, y):
    spans, color = [], None
    for part in re.split('(\x1b\\[[0-9]+m)', line):
        m = re.fullmatch('\x1b\\[([0-9]+)m', part)
        if m:
            color = colors.get(m.group(1))
        elif part:
            content = html.escape(part).replace(' ', '\u00a0')
            spans.append(f'<tspan fill="{color}">{content}</tspan>' if color else content)
    n = len(escape.sub('', line))
    fit = f' textLength="{n * size * 0.6:.1f}" lengthAdjust="spacingAndGlyphs"' if n else ''
    return f'<text x="{pad}" y="{y}"{fit}>{"".join(spans)}</text>', n


def main(out, pairs):
    screens = [(['\x1b[35m$\x1b[0m ' + cmd] + read(path)) for cmd, path in pairs]
    longest = max(len(escape.sub('', l)) for s in screens for l in s)
    width = int(longest * size * 0.6 + 2 * pad)
    height = top + max(len(s) for s in screens) * step + pad - 4

    svg = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">',
           f'<rect width="{width}" height="{height}" rx="10" fill="{bg}"/>',
           '<circle cx="22" cy="20" r="6" fill="#FF5F57"/><circle cx="42" cy="20" r="6" fill="#FEBC2E"/>'
           '<circle cx="62" cy="20" r="6" fill="#28C840"/>',
           f'<g font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" font-size="{size}" '
           f'fill="{fg}" xml:space="preserve" style="white-space: pre">']

    if len(screens) == 1:
        for i, line in enumerate(screens[0]):
            svg.append(text(line, top + i * step)[0])
        svg.append('</g></svg>')
        return '\n'.join(svg) + '\n'

    # Timeline: when each screen starts and ends, and when each of its lines shows up.
    starts, t = [], 0.0
    for screen in screens:
        starts.append(t)
        t += typing + per_line * (len(screen) - 1) + hold
    total = t
    pct = lambda s: f'{100 * s / total:.3f}%'

    style, body = [], []
    for k, screen in enumerate(screens):
        start, end = starts[k], starts[k + 1] if k + 1 < len(screens) else total
        for i, line in enumerate(screen):
            appear = start + (0.05 if i == 0 else typing + per_line * (i - 1))
            name = f's{k}l{i}'
            element, n = text(line, top + i * step)
            if i == 0 and n:
                # The command line is typed: it grows from the left while it appears.
                style.append(f'@keyframes {name}{{0%,{pct(appear)}{{opacity:0;clip-path:inset(0 100% 0 0)}}'
                             f'{pct(appear + 0.01)}{{opacity:1;clip-path:inset(0 100% 0 0)}}'
                             f'{pct(appear + typing * 0.8)},{pct(end - 0.01)}{{opacity:1;clip-path:inset(0 0 0 0)}}'
                             f'{pct(end)},100%{{opacity:0}}}}')
            else:
                style.append(f'@keyframes {name}{{0%,{pct(appear)}{{opacity:0}}'
                             f'{pct(appear + 0.01)},{pct(end - 0.01)}{{opacity:1}}{pct(end)},100%{{opacity:0}}}}')
            style.append(f'.{name}{{opacity:0;animation:{name} {total:.2f}s linear infinite}}')
            body.append(element.replace('<text ', f'<text class="{name}" ', 1))

    svg.insert(3, '<style>' + ''.join(style) + '</style>')
    svg.extend(body)
    svg.append('</g></svg>')
    return '\n'.join(svg) + '\n'


if __name__ == '__main__':
    args = sys.argv[1:]
    if len(args) < 3 or len(args) % 2 == 0:
        sys.exit("usage: render-demo.py out.svg 'command' output.ansi ['command' output.ansi ...]")
    open(args[0], 'w').write(main(args[0], list(zip(args[1::2], args[2::2]))))
