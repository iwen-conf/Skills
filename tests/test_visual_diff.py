"""Unit tests for visual_diff.py core comparison algorithm."""

from __future__ import annotations

import json
import importlib.util
import sys
from pathlib import Path

import pytest

_SCRIPTS = Path(__file__).resolve().parent.parent / "Arc" / "arc-e2e" / "scripts"
_VISUAL_DIFF_SPEC = importlib.util.spec_from_file_location(
    "visual_diff_under_test",
    _SCRIPTS / "visual_diff.py",
)
if _VISUAL_DIFF_SPEC is None or _VISUAL_DIFF_SPEC.loader is None:
    raise RuntimeError("Unable to load visual_diff.py")

_visual_diff = importlib.util.module_from_spec(_VISUAL_DIFF_SPEC)
_VISUAL_DIFF_SPEC.loader.exec_module(_visual_diff)

_parse_mask_regions = _visual_diff._parse_mask_regions
compute_diff = _visual_diff.compute_diff
generate_diff_image = _visual_diff.generate_diff_image
sha256_file = _visual_diff.sha256_file

PIL = pytest.importorskip("PIL", reason="Pillow required for visual_diff tests")
from PIL import Image, ImageDraw  # noqa: E402


@pytest.fixture()
def tmp(tmp_path: Path) -> Path:
    return tmp_path


def _make_solid(path: Path, color: tuple[int, int, int], size: tuple[int, int] = (100, 100)) -> Path:
    img = Image.new("RGB", size, color)
    img.save(str(path))
    return path


def _make_with_rect(
    path: Path,
    bg: tuple[int, int, int],
    rect_color: tuple[int, int, int],
    rect: tuple[int, int, int, int],
    size: tuple[int, int] = (100, 100),
) -> Path:
    img = Image.new("RGB", size, bg)
    draw = ImageDraw.Draw(img)
    draw.rectangle(list(rect), fill=rect_color)
    img.save(str(path))
    return path


class TestIdenticalImages:
    def test_identical_gives_perfect_score(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 100, 200))
        b = _make_solid(tmp / "b.png", (0, 100, 200))
        result = compute_diff(a, b)
        assert result["pixel_diff_ratio"] == 0.0
        assert result["similarity"] >= 0.99

    def test_identical_pass_at_threshold(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (255, 255, 255))
        b = _make_solid(tmp / "b.png", (255, 255, 255))
        result = compute_diff(a, b)
        assert result["similarity"] >= 0.95


class TestDifferentImages:
    def test_completely_different(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 0, 0))
        b = _make_solid(tmp / "b.png", (255, 255, 255))
        result = compute_diff(a, b)
        assert result["pixel_diff_ratio"] == 1.0
        assert result["similarity"] < 0.5

    def test_small_change_detectable(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 100, 200))
        b = _make_with_rect(tmp / "b.png", (0, 100, 200), (255, 0, 0), (45, 45, 55, 55))
        result = compute_diff(a, b)
        assert result["pixel_diff_ratio"] > 0.0
        assert result["changed_pixels"] > 0
        # Small change should still have high similarity
        assert result["similarity"] > 0.8


class TestMaskRegions:
    def test_parse_empty(self) -> None:
        assert _parse_mask_regions("") == []
        assert _parse_mask_regions("   ") == []

    def test_parse_single(self) -> None:
        result = _parse_mask_regions("10,20,30,40")
        assert result == [(10, 20, 30, 40)]

    def test_parse_multiple(self) -> None:
        result = _parse_mask_regions("0,0,100,50;200,300,50,50")
        assert result == [(0, 0, 100, 50), (200, 300, 50, 50)]

    def test_mask_excludes_region(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 100, 200))
        b = _make_with_rect(tmp / "b.png", (0, 100, 200), (255, 0, 0), (0, 0, 20, 20))
        # Without mask, should detect difference
        no_mask = compute_diff(a, b)
        assert no_mask["pixel_diff_ratio"] > 0.0
        # With mask covering the changed area, should be identical
        masked = compute_diff(a, b, mask_regions=[(0, 0, 21, 21)])
        assert masked["pixel_diff_ratio"] == 0.0


class TestDiffImage:
    def test_generates_output_file(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 0, 0))
        b = _make_solid(tmp / "b.png", (255, 255, 255))
        metrics = compute_diff(a, b)
        out = tmp / "diff.png"
        generate_diff_image(metrics, out)
        assert out.exists()
        img = Image.open(out)
        assert img.size == (100, 100)

    def test_creates_parent_dirs(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 0, 0))
        b = _make_solid(tmp / "b.png", (0, 0, 0))
        metrics = compute_diff(a, b)
        out = tmp / "sub" / "dir" / "diff.png"
        generate_diff_image(metrics, out)
        assert out.exists()


class TestSizeNormalization:
    def test_different_sizes(self, tmp: Path) -> None:
        a = _make_solid(tmp / "a.png", (0, 100, 200), size=(100, 100))
        b = _make_solid(tmp / "b.png", (0, 100, 200), size=(120, 110))
        result = compute_diff(a, b)
        # Should normalize to baseline size
        assert result["width"] == 100
        assert result["height"] == 100


class TestSha256:
    def test_consistent_hash(self, tmp: Path) -> None:
        p = _make_solid(tmp / "img.png", (128, 128, 128))
        h1 = sha256_file(p)
        h2 = sha256_file(p)
        assert h1 == h2
        assert len(h1) == 64


class TestCLI:
    def test_json_output(self, tmp: Path) -> None:
        import subprocess
        a = _make_solid(tmp / "a.png", (0, 100, 200))
        b = _make_solid(tmp / "b.png", (0, 100, 200))
        result = subprocess.run(
            [sys.executable, str(_SCRIPTS / "visual_diff.py"),
             "--baseline", str(a), "--current", str(b), "--json"],
            capture_output=True, text=True,
        )
        assert result.returncode == 0
        data = json.loads(result.stdout)
        assert data["passed"] is True
        assert data["similarity"] >= 0.95
