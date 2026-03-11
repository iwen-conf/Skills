#!/usr/bin/env python3
"""
Pixel-level screenshot comparison with diff visualization.

Compares a baseline image against a current screenshot, producing:
  - A similarity score (NCC + pixel diff ratio)
  - A visual diff image (changed pixels highlighted in red)
  - JSON output for machine consumption

Requires: Pillow >= 10.4.0
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from PIL import Image


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


def _parse_mask_regions(spec: str) -> list[tuple[int, int, int, int]]:
    """Parse mask regions from 'x,y,w,h;x,y,w,h' format."""
    if not spec or not spec.strip():
        return []
    regions: list[tuple[int, int, int, int]] = []
    for part in spec.split(";"):
        part = part.strip()
        if not part:
            continue
        nums = [int(x.strip()) for x in part.split(",")]
        if len(nums) != 4:
            raise ValueError(f"Mask region must have 4 values (x,y,w,h), got: {part}")
        regions.append((nums[0], nums[1], nums[2], nums[3]))
    return regions


def _apply_masks(
    img: "Image.Image",
    regions: list[tuple[int, int, int, int]],
    fill: tuple[int, int, int] = (128, 128, 128),
) -> "Image.Image":
    """Fill mask regions with a uniform color to exclude from comparison."""
    from PIL import ImageDraw

    draw = ImageDraw.Draw(img)
    for x, y, w, h in regions:
        draw.rectangle([x, y, x + w, y + h], fill=fill)
    return img


def _normalize_size(
    baseline: "Image.Image",
    current: "Image.Image",
) -> tuple["Image.Image", "Image.Image"]:
    """Resize current to match baseline dimensions if they differ."""
    from PIL import Image

    if baseline.size == current.size:
        return baseline, current
    bw, bh = baseline.size
    cw, ch = current.size
    # Crop or pad current to match baseline
    result = Image.new("RGB", (bw, bh), (0, 0, 0))
    result.paste(current, (0, 0))
    # Crop if current is larger
    result = result.crop((0, 0, bw, bh))
    return baseline, result


def compute_diff(
    baseline_path: str | Path,
    current_path: str | Path,
    *,
    mask_regions: list[tuple[int, int, int, int]] | None = None,
) -> dict[str, Any]:
    """Core comparison function. Returns metrics dict (no side effects)."""
    from PIL import Image, ImageChops

    baseline = Image.open(baseline_path).convert("RGB")
    current = Image.open(current_path).convert("RGB")
    baseline, current = _normalize_size(baseline, current)

    if mask_regions:
        baseline = _apply_masks(baseline.copy(), mask_regions)
        current = _apply_masks(current.copy(), mask_regions)

    w, h = baseline.size
    total_pixels = w * h

    # Pixel-level difference
    diff_img = ImageChops.difference(baseline, current)
    _getdata = getattr(diff_img, "get_flattened_data", diff_img.getdata)
    diff_data = _getdata()
    changed = sum(1 for r, g, b in diff_data if r > 0 or g > 0 or b > 0)
    pixel_diff_ratio = changed / total_pixels if total_pixels > 0 else 0.0

    # NCC: normalized cross-correlation on grayscale histograms
    b_gray = baseline.convert("L")
    c_gray = current.convert("L")
    b_hist = b_gray.histogram()
    c_hist = c_gray.histogram()

    mean_b = sum(i * v for i, v in enumerate(b_hist)) / max(sum(b_hist), 1)
    mean_c = sum(i * v for i, v in enumerate(c_hist)) / max(sum(c_hist), 1)

    num = sum((b_hist[i] - mean_b) * (c_hist[i] - mean_c) for i in range(256))
    den_b = math.sqrt(sum((b_hist[i] - mean_b) ** 2 for i in range(256)))
    den_c = math.sqrt(sum((c_hist[i] - mean_c) ** 2 for i in range(256)))
    denom = den_b * den_c
    ncc = num / denom if denom > 0 else 0.0

    # Combined similarity: weighted average (NCC is global, pixel ratio is local)
    similarity = round(max(0.0, min(1.0, 0.6 * ncc + 0.4 * (1.0 - pixel_diff_ratio))), 6)

    return {
        "baseline": str(baseline_path),
        "current": str(current_path),
        "width": w,
        "height": h,
        "total_pixels": total_pixels,
        "changed_pixels": changed,
        "pixel_diff_ratio": round(pixel_diff_ratio, 6),
        "ncc": round(ncc, 6),
        "similarity": similarity,
        "_diff_img": diff_img,
        "_current_img": current,
    }


def generate_diff_image(
    metrics: dict[str, Any],
    output_path: str | Path,
) -> None:
    """Generate a visual diff: changed pixels highlighted in red over the current image."""
    diff_img = metrics["_diff_img"]
    current = metrics["_current_img"]
    output = current.copy()
    _get_diff = getattr(diff_img, "get_flattened_data", diff_img.getdata)
    _get_out = getattr(output, "get_flattened_data", output.getdata)
    diff_data = list(_get_diff())
    out_data = list(_get_out())

    highlighted: list[tuple[int, int, int]] = []
    for (dr, dg, db), (cr, cg, cb) in zip(diff_data, out_data):
        if dr > 0 or dg > 0 or db > 0:
            # Blend: 60% red + 40% original
            highlighted.append((
                min(255, int(0.6 * 255 + 0.4 * cr)),
                int(0.4 * cg),
                int(0.4 * cb),
            ))
        else:
            highlighted.append((cr, cg, cb))

    output.putdata(highlighted)
    Path(output_path).parent.mkdir(parents=True, exist_ok=True)
    output.save(str(output_path))


def sha256_file(path: str | Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


def main() -> None:
    _ensure_pillow()

    parser = argparse.ArgumentParser(
        description="Compare two screenshots and produce a visual diff",
    )
    parser.add_argument("--baseline", required=True, help="Path to baseline screenshot")
    parser.add_argument("--current", required=True, help="Path to current screenshot")
    parser.add_argument("--output", help="Path for diff output image (default: <current>.diff.png)")
    parser.add_argument(
        "--threshold",
        type=float,
        default=0.95,
        help="Similarity threshold for PASS (default: 0.95)",
    )
    parser.add_argument(
        "--mask-regions",
        default="",
        help='Mask regions to exclude: "x,y,w,h;x,y,w,h" (e.g. "0,0,200,50;1720,0,200,50")',
    )
    parser.add_argument("--json", action="store_true", dest="json_output", help="Output results as JSON")
    parser.add_argument("--fail-on-diff", action="store_true", help="Exit 1 if similarity < threshold")
    args = parser.parse_args()

    baseline = Path(args.baseline)
    current = Path(args.current)

    if not baseline.exists():
        print(f"[FAIL] Baseline not found: {baseline}", file=sys.stderr)
        raise SystemExit(2)
    if not current.exists():
        print(f"[FAIL] Current screenshot not found: {current}", file=sys.stderr)
        raise SystemExit(2)

    masks = _parse_mask_regions(args.mask_regions)
    metrics = compute_diff(baseline, current, mask_regions=masks)

    output_path = args.output or str(current.with_suffix(".diff.png"))
    generate_diff_image(metrics, output_path)

    passed = metrics["similarity"] >= args.threshold
    result = {
        "baseline": str(baseline),
        "current": str(current),
        "diff_image": output_path,
        "similarity": metrics["similarity"],
        "pixel_diff_ratio": metrics["pixel_diff_ratio"],
        "ncc": metrics["ncc"],
        "threshold": args.threshold,
        "passed": passed,
        "dimensions": f"{metrics['width']}x{metrics['height']}",
        "changed_pixels": metrics["changed_pixels"],
        "total_pixels": metrics["total_pixels"],
    }

    if args.json_output:
        print(json.dumps(result, indent=2))
    else:
        status = "PASS" if passed else "FAIL"
        print(f"[{status}] similarity={metrics['similarity']:.4f} threshold={args.threshold} "
              f"pixel_diff={metrics['pixel_diff_ratio']:.4f} diff={output_path}")

    if args.fail_on_diff and not passed:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
