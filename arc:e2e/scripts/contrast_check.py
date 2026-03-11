#!/usr/bin/env python3
"""
WCAG contrast ratio batch scanning on screenshots.

Extracts dominant color pairs via median-cut quantization (Pillow-only)
and evaluates against WCAG 2.x thresholds:
  - AA normal text: >= 4.5:1
  - AA large text:  >= 3.0:1
  - AAA normal text: >= 7.0:1

Requires: Pillow >= 10.4.0
"""

from __future__ import annotations

import argparse
import json
import math
import sys
from pathlib import Path
from typing import Any


def _ensure_pillow() -> None:
    try:
        from PIL import Image as _  # noqa: F401
    except ImportError:
        print(
            "Pillow is required but not installed.\n"
            "Install with: uv sync --group visual  (or: pip install Pillow>=10.4.0)",
            file=sys.stderr,
        )
        raise SystemExit(2)


def _srgb_linear(c: int) -> float:
    s = c / 255.0
    return s / 12.92 if s <= 0.04045 else ((s + 0.055) / 1.055) ** 2.4


def _relative_luminance(r: int, g: int, b: int) -> float:
    return 0.2126 * _srgb_linear(r) + 0.7152 * _srgb_linear(g) + 0.0722 * _srgb_linear(b)


def _contrast_ratio(c1: tuple[int, int, int], c2: tuple[int, int, int]) -> float:
    l1 = _relative_luminance(*c1)
    l2 = _relative_luminance(*c2)
    lighter = max(l1, l2)
    darker = min(l1, l2)
    return (lighter + 0.05) / (darker + 0.05)


def _color_distance(c1: tuple[int, ...], c2: tuple[int, ...]) -> float:
    return math.sqrt(sum((a - b) ** 2 for a, b in zip(c1, c2)))


def extract_dominant_colors(
    image_path: str | Path,
    n_colors: int = 8,
) -> list[tuple[int, int, int]]:
    """Extract dominant colors via Pillow quantize (median cut)."""
    from PIL import Image

    img = Image.open(image_path).convert("RGB")
    # Resize for performance
    img.thumbnail((200, 200))
    quantized = img.quantize(colors=n_colors, method=0)  # 0 = median cut
    palette = quantized.getpalette()
    if not palette:
        return []
    colors: list[tuple[int, int, int]] = []
    for i in range(n_colors):
        idx = i * 3
        if idx + 2 < len(palette):
            colors.append((palette[idx], palette[idx + 1], palette[idx + 2]))
    return colors


def analyze_contrast(
    image_path: str | Path,
    *,
    n_colors: int = 8,
) -> dict[str, Any]:
    """Analyze contrast between dominant color pairs in a screenshot."""
    colors = extract_dominant_colors(image_path, n_colors=n_colors)
    if len(colors) < 2:
        return {
            "image": str(image_path),
            "colors_found": len(colors),
            "pairs": [],
            "worst_ratio": None,
            "aa_normal_pass": True,
            "aa_large_pass": True,
            "aaa_normal_pass": True,
        }

    pairs: list[dict[str, Any]] = []
    worst_ratio = float("inf")

    for i in range(len(colors)):
        for j in range(i + 1, len(colors)):
            # Skip very similar colors (likely same region)
            if _color_distance(colors[i], colors[j]) < 30:
                continue
            ratio = _contrast_ratio(colors[i], colors[j])
            worst_ratio = min(worst_ratio, ratio)
            pairs.append({
                "color_1": f"rgb({colors[i][0]}, {colors[i][1]}, {colors[i][2]})",
                "color_2": f"rgb({colors[j][0]}, {colors[j][1]}, {colors[j][2]})",
                "contrast_ratio": round(ratio, 2),
                "aa_normal": ratio >= 4.5,
                "aa_large": ratio >= 3.0,
                "aaa_normal": ratio >= 7.0,
            })

    if worst_ratio == float("inf"):
        worst_ratio = None

    return {
        "image": str(image_path),
        "colors_found": len(colors),
        "pairs_checked": len(pairs),
        "worst_ratio": round(worst_ratio, 2) if worst_ratio is not None else None,
        "aa_normal_pass": all(p["aa_normal"] for p in pairs) if pairs else True,
        "aa_large_pass": all(p["aa_large"] for p in pairs) if pairs else True,
        "aaa_normal_pass": all(p["aaa_normal"] for p in pairs) if pairs else True,
        "pairs": pairs,
    }


def generate_annotated_image(
    image_path: str | Path,
    analysis: dict[str, Any],
    output_path: str | Path,
) -> None:
    """Generate annotated image marking low-contrast regions (placeholder)."""
    from PIL import Image, ImageDraw

    img = Image.open(image_path).convert("RGB")
    draw = ImageDraw.Draw(img)

    # Draw a red border if any AA normal check fails
    if not analysis.get("aa_normal_pass", True):
        w, h = img.size
        for offset in range(3):
            draw.rectangle(
                [offset, offset, w - 1 - offset, h - 1 - offset],
                outline=(255, 0, 0),
            )

    Path(output_path).parent.mkdir(parents=True, exist_ok=True)
    img.save(str(output_path))


def main() -> None:
    _ensure_pillow()

    parser = argparse.ArgumentParser(description="WCAG contrast ratio scanning on screenshots")
    parser.add_argument("--input", required=True, help="Screenshot file or directory of PNGs")
    parser.add_argument("--output", help="Output directory for annotated images")
    parser.add_argument("--n-colors", type=int, default=8, help="Number of dominant colors to extract (default: 8)")
    parser.add_argument("--json", action="store_true", dest="json_output", help="Output as JSON")
    parser.add_argument("--fail-on-aa", action="store_true", help="Exit 1 if any AA normal check fails")
    args = parser.parse_args()

    input_path = Path(args.input)
    if input_path.is_dir():
        files = sorted(input_path.glob("*.png"))
    elif input_path.is_file():
        files = [input_path]
    else:
        print(f"[FAIL] Input not found: {input_path}", file=sys.stderr)
        raise SystemExit(2)

    results: list[dict[str, Any]] = []
    for f in files:
        analysis = analyze_contrast(f, n_colors=args.n_colors)
        results.append(analysis)
        if args.output:
            out_path = Path(args.output) / f"{f.stem}.contrast.png"
            generate_annotated_image(f, analysis, out_path)

    all_aa_pass = all(r.get("aa_normal_pass", True) for r in results)
    summary = {
        "files_checked": len(results),
        "all_aa_normal_pass": all_aa_pass,
        "results": results,
    }

    if args.json_output:
        print(json.dumps(summary, indent=2))
    else:
        for r in results:
            status = "PASS" if r.get("aa_normal_pass", True) else "FAIL"
            worst = r.get("worst_ratio", "-")
            print(f"[{status}] {r['image']} worst_ratio={worst}")

    if args.fail_on_aa and not all_aa_pass:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
