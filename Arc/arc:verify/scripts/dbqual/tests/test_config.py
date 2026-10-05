from __future__ import annotations

import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
from sqlverify_config import load_config, validate

class ConfigTests(unittest.TestCase):
    def test_example_config_valid(self):
        cfg = load_config(ROOT / "examples" / "sql-verification.toml")
        self.assertEqual(validate(cfg, ROOT), [])

if __name__ == "__main__":
    unittest.main()
