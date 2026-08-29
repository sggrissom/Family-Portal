#!/usr/bin/env python3
"""Generate the Family Record icon set from a single geometry definition.

The mark is three white strokes on the brand green: a vertical rule crossed by
two shorter ticks that rise like pencil marks on a doorframe. It also reads as
an "F". Run `python3 scripts/gen-icons.py` after changing any constant here.

The iOS app icon uses the same geometry — keep the constants here in sync with
Family-Portal-Ios/scripts/gen-app-icon.py.
"""

import os
import struct

from PIL import Image, ImageDraw

OUT = os.path.join(os.path.dirname(__file__), "..", "frontend", "images")

GREEN = (16, 185, 129, 255)  # manifest theme_color
WHITE = (255, 255, 255, 255)

S = 512  # design canvas
CORNER = 112
STROKE = 44
SS = 4  # supersample factor for the master render

# Stroke centerlines, all axis-aligned: (x1, y1, x2, y2)
STROKES = [
    (156, 106, 156, 406),  # vertical rule
    (156, 166, 356, 166),  # upper tick (tallest mark)
    (156, 286, 286, 286),  # lower tick
]

# Maskable icons are cropped to a circle 80% of the canvas, so the glyph is
# scaled down to sit inside that safe zone.
MASKABLE_SCALE = 0.8


def _capsule(x1, y1, x2, y2):
    """An axis-aligned round-capped stroke is a rounded rectangle."""
    r = STROKE / 2
    return (x1 - r, y1 - r, x2 + r, y2 + r), r


def render(rounded=True, scale=1.0):
    """Draw the master icon at SS times the design canvas."""
    n = S * SS
    img = Image.new("RGBA", (n, n), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)

    if rounded:
        d.rounded_rectangle((0, 0, n - 1, n - 1), radius=CORNER * SS, fill=GREEN)
    else:
        d.rectangle((0, 0, n - 1, n - 1), fill=GREEN)

    c = S / 2
    for stroke in STROKES:
        (x1, y1, x2, y2), r = _capsule(*stroke)
        box = [(v - c) * scale + c for v in (x1, y1, x2, y2)]
        d.rounded_rectangle(
            [v * SS for v in box], radius=r * scale * SS, fill=WHITE
        )
    return img


def save(img, name, size):
    out = os.path.join(OUT, name)
    img.resize((size, size), Image.LANCZOS).save(out)
    print(f"  {name} ({size}x{size})")


def save_ico(img, name, sizes=(16, 32, 48)):
    """Write an ICO containing PNG-encoded frames at each size."""
    import io

    frames = []
    for s in sizes:
        buf = io.BytesIO()
        img.resize((s, s), Image.LANCZOS).save(buf, format="PNG")
        frames.append((s, buf.getvalue()))

    offset = 6 + 16 * len(frames)
    header = struct.pack("<HHH", 0, 1, len(frames))
    entries, blobs = b"", b""
    for s, data in frames:
        entries += struct.pack(
            "<BBBBHHII", s if s < 256 else 0, s if s < 256 else 0,
            0, 0, 1, 32, len(data), offset
        )
        blobs += data
        offset += len(data)

    with open(os.path.join(OUT, name), "wb") as f:
        f.write(header + entries + blobs)
    print(f"  {name} ({', '.join(str(s) for s in sizes)})")


def svg_source():
    paths = "\n".join(
        f'    <path d="M{x1} {y1} {"V" + str(y2) if x1 == x2 else "H" + str(x2)}"/>'
        for x1, y1, x2, y2 in STROKES
    )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {S} {S}">\n'
        f'  <rect width="{S}" height="{S}" rx="{CORNER}" fill="#10b981"/>\n'
        f'  <g stroke="#fff" stroke-width="{STROKE}" stroke-linecap="round" fill="none">\n'
        f"{paths}\n"
        f"  </g>\n"
        f"</svg>\n"
    )


def main():
    print("Generating icons in frontend/images/")

    # Rounded plate: browsers render these as-is, so the corners are ours.
    rounded = render(rounded=True)
    save(rounded, "favicon-16x16.png", 16)
    save(rounded, "favicon-32x32.png", 32)
    save(rounded, "android-chrome-192x192.png", 192)
    save(rounded, "android-chrome-512x512.png", 512)
    save_ico(rounded, "favicon.ico")

    # Square plate: iOS applies its own squircle mask and ignores transparency.
    save(render(rounded=False), "apple-touch-icon.png", 180)

    # Square plate, inset glyph: Android may crop this to any shape.
    save(
        render(rounded=False, scale=MASKABLE_SCALE),
        "icon-maskable-512x512.png",
        512,
    )

    with open(os.path.join(OUT, "icon.svg"), "w") as f:
        f.write(svg_source())
    print("  icon.svg (vector source)")


if __name__ == "__main__":
    main()
