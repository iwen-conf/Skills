"""Fail-closed ETA vs Arc pixel comparison for matched REAL application screenshots.

Both files must come from independently running applications with the same
fixture, viewport, scale, theme and font. Synthetic source-derived mockups are
NOT valid product baselines.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

from PIL import Image, ImageChops


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def compare(reference: Path, actual: Path, out: Path, tolerance: int = 0) -> dict:
    if not 0 <= tolerance <= 255:
        raise ValueError("RGB channel tolerance must be between 0 and 255")
    if reference.resolve() == actual.resolve():
        raise ValueError("Reference and actual paths must be different files")
    out.mkdir(parents=True, exist_ok=True)
    with Image.open(reference) as source, Image.open(actual) as result:
        source_rgba = source.convert("RGBA")
        result_rgba = result.convert("RGBA")
        same_size = source_rgba.size == result_rgba.size
        report = {
            "scope": "supplied screenshot pixels only; not proof of interaction parity",
            "reference": str(reference),
            "actual": str(actual),
            "reference_sha256": sha256(reference),
            "actual_sha256": sha256(actual),
            "reference_size": list(source_rgba.size),
            "actual_size": list(result_rgba.size),
            "channel_tolerance": tolerance,
            "threshold_mismatched_pixels": 0,
            "identical_dimensions": same_size,
            "mismatched_pixels": None,
            "result": "FAIL",
        }
        if not same_size:
            report["reason"] = "Screenshots have different sizes; refuse crop/stretch"
        else:
            diffs = ImageChops.difference(source_rgba.convert("RGB"), result_rgba.convert("RGB"))
            raw = diffs.tobytes()
            # Detect one-channel, one-value differences without RGB→L rounding.
            mismatches = [
                any(channel > tolerance for channel in raw[i : i + 3])
                for i in range(0, len(raw), 3)
            ]
            count = sum(mismatches)
            report["mismatched_pixels"] = count
            report["mismatch_fraction"] = count / (source_rgba.width * source_rgba.height)
            report["result"] = "PASS" if count == 0 else "FAIL"
            if count:
                mask = Image.new("RGB", source_rgba.size, (255, 255, 255))
                mask.putdata([
                    (210, 31, 46) if mismatch else (255, 255, 255)
                    for mismatch in mismatches
                ])
                mask.save(out / "mismatch-map.png")
                report["mismatch_map"] = str(out / "mismatch-map.png")
    (out / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    return report


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference", required=True, type=Path)
    parser.add_argument("--actual", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--channel-tolerance", type=int, default=0)
    args = parser.parse_args()
    result = compare(args.reference, args.actual, args.out, args.channel_tolerance)
    print(json.dumps(result, ensure_ascii=False))
    return 0 if result["result"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
