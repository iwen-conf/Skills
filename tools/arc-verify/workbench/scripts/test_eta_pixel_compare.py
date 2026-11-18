"""A one-pixel deviation must not produce a 1:1 PASS."""
from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from PIL import Image
from eta_pixel_compare import compare


class PixelCompareTest(unittest.TestCase):
    def test_identical_pixels_in_distinct_files_pass(self):
        with tempfile.TemporaryDirectory() as directory:
            d = Path(directory)
            a, b = d / "eta.png", d / "arc.png"
            for path in [a, b]:
                Image.new("RGB", (16, 12), (245, 245, 245)).save(path)
            report = compare(a, b, d / "diff")
            self.assertEqual(report["result"], "PASS")
            self.assertEqual(report["mismatched_pixels"], 0)

    def test_one_pixel_difference_fails_and_keeps_diff(self):
        with tempfile.TemporaryDirectory() as directory:
            d = Path(directory)
            a, b = d / "eta.png", d / "arc.png"
            Image.new("RGB", (16, 12), (245, 245, 245)).save(a)
            modified = Image.new("RGB", (16, 12), (245, 245, 245))
            modified.putpixel((4, 4), (245, 244, 245))
            modified.save(b)
            report = compare(a, b, d / "diff")
            self.assertEqual(report["result"], "FAIL")
            self.assertEqual(report["mismatched_pixels"], 1)
            self.assertTrue((d / "diff" / "mismatch-map.png").exists())

    def test_one_alpha_channel_difference_also_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            d = Path(directory)
            a, b = d / "eta.png", d / "arc.png"
            Image.new("RGBA", (8, 8), (15, 25, 35, 255)).save(a)
            image = Image.new("RGBA", (8, 8), (15, 25, 35, 255))
            image.putpixel((0, 0), (15, 25, 35, 254))
            image.save(b)
            self.assertEqual(compare(a, b, d / "diff")["mismatched_pixels"], 1)

    def test_size_mismatch_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            d = Path(directory)
            a, b = d / "eta.png", d / "arc.png"
            Image.new("RGB", (16, 12)).save(a)
            Image.new("RGB", (16, 13)).save(b)
            self.assertEqual(compare(a, b, d / "diff")["result"], "FAIL")

    def test_self_reference_is_not_validated(self):
        with tempfile.TemporaryDirectory() as directory:
            d = Path(directory)
            a = d / "eta.png"
            Image.new("RGB", (16, 12)).save(a)
            with self.assertRaisesRegex(ValueError, "must be different"):
                compare(a, a, d / "diff")
