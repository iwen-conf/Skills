from __future__ import annotations

import json
import os
import shlex
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))

from sql_static_audit import evaluate_sqlfluff, evaluate_squawk
from sqlverify_config import validate

# Report shapes captured from sqlfluff 4.4.0 (`lint --format json`) and squawk 2.68.0 (`--reporter json`).
FAKE_TOOL = textwrap.dedent("""
    import json, os, sys
    if "--version" in sys.argv:
        print("fake 0"); sys.exit(0)
    print(open(os.environ["FAKE_REPORT_" + os.environ["FAKE_NAME"]]).read())
    sys.exit(int(os.environ.get("FAKE_EXIT_" + os.environ["FAKE_NAME"], "0")))
""")


def violation(code: str, name: str = "", warning: bool = False) -> dict:
    return {
        "code": code,
        "name": name,
        "description": f"{code} found",
        "start_line_no": 1,
        "start_line_pos": 1,
        "warning": warning,
    }


class EvaluateTests(unittest.TestCase):
    def test_parse_errors_always_block_and_style_is_advisory_by_default(self):
        report = [
            {"filepath": "a.sql", "violations": [violation("PRS")]},
            {"filepath": "b.sql", "violations": [violation("AM04", "ambiguous.column_count"), violation("LT14")]},
        ]
        out = evaluate_sqlfluff(report, ["a.sql", "b.sql"], [])
        self.assertEqual([x["code"] for x in out["syntax_errors"]], ["PRS"])
        self.assertEqual((out["blocking"], out["advisory"]), ([], {"AM04": 1, "LT14": 1}))

    def test_fail_on_codes_block_unless_the_project_marked_them_warnings(self):
        report = [{"filepath": "b.sql", "violations": [violation("AM04"), violation("CV05", warning=True)]}]
        out = evaluate_sqlfluff(report, ["b.sql"], ["AM04", "CV05"])
        self.assertEqual([x["code"] for x in out["blocking"]], ["AM04"])
        self.assertEqual(out["advisory"], {"CV05": 1})

    def test_a_file_missing_from_the_report_is_detected(self):
        out = evaluate_sqlfluff([{"filepath": "a.sql", "violations": []}], ["a.sql", "b.sql"], [])
        self.assertEqual(out["unreported_files"], ["b.sql"])

    def test_squawk_syntax_errors_are_separated_and_lines_are_one_based(self):
        out = evaluate_squawk(
            [
                {"file": "m.sql", "line": 0, "rule_name": "syntax-error", "message": "expected command"},
                {"file": "m.sql", "line": 1, "rule_name": "require-concurrent-index-creation", "message": "lock"},
            ]
        )
        self.assertEqual(out["syntax_errors"][0]["line"], 1)
        self.assertEqual([x["rule"] for x in out["blocking"]], ["require-concurrent-index-creation"])

    def test_config_requires_dialect_paths_and_exclusion_rationale(self):
        cfg = {
            "project": {"name": "t"},
            "database": {"dsn_env": "X", "safe_database_pattern": "_test$"},
            "dataset": {},
            "sql_static": {"paths": [], "squawk_excluded_rules": {"require-statement-timeout": " "}},
        }
        errors = [e for e in validate(cfg, ROOT) if e.startswith("sql_static")]
        self.assertEqual(len(errors), 3, errors)


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.td = tempfile.TemporaryDirectory()
        self.root = Path(self.td.name)
        (self.root / "db" / "migrations").mkdir(parents=True)
        (self.root / "db" / "q.sql").write_text("SELECT o.id FROM orders AS o;\n", encoding="utf-8")
        (self.root / "db" / "migrations" / "001.up.sql").write_text(
            "CREATE INDEX i ON orders (id);\n", encoding="utf-8"
        )
        self.fake = self.root / "fake_tool.py"
        self.fake.write_text(FAKE_TOOL, encoding="utf-8")
        self.env = dict(os.environ)

    def tearDown(self):
        self.td.cleanup()

    def tool(self, name: str, report: list, exit_code: int = 0) -> list[str]:
        path = self.root / f"{name}.json"
        path.write_text(json.dumps(report), encoding="utf-8")
        self.env[f"FAKE_REPORT_{name}"] = str(path)
        self.env[f"FAKE_EXIT_{name}"] = str(exit_code)
        return ["env", f"FAKE_NAME={name}", sys.executable, str(self.fake)]

    def run_audit(self, section: str) -> tuple[int, dict]:
        cfg = self.root / "c.toml"
        cfg.write_text(section, encoding="utf-8")
        out = self.root / "sql.static.json"
        cp = subprocess.run(
            [
                sys.executable,
                str(SCRIPTS / "sql_static_audit.py"),
                "--config",
                str(cfg),
                "--root",
                str(self.root),
                "--out",
                str(out),
            ],
            capture_output=True,
            text=True,
            env=self.env,
        )
        return cp.returncode, json.loads(out.read_text(encoding="utf-8"))

    def config(self, sqlfluff: list[str], extra: str = "", paths: str = '["db/*.sql"]') -> str:
        return (
            f'[sql_static]\ndialect = "postgres"\npaths = {paths}\nsqlfluff_command = {json.dumps(sqlfluff)}\n{extra}'
        )

    def test_clean_files_pass_and_style_findings_stay_warnings(self):
        cmd = self.tool("sqlfluff", [{"filepath": "db/q.sql", "violations": [violation("LT14")]}], exit_code=1)
        rc, ev = self.run_audit(self.config(cmd))
        self.assertEqual((rc, ev["status"]), (0, "PASS"), ev["violations"])
        self.assertEqual(ev["warnings"], ["advisory LT14: 1"])
        self.assertEqual(ev["observed"]["files_checked"], 1)

    def test_parse_error_fails(self):
        cmd = self.tool("sqlfluff", [{"filepath": "db/q.sql", "violations": [violation("PRS")]}], exit_code=1)
        rc, ev = self.run_audit(self.config(cmd))
        self.assertEqual((rc, ev["status"]), (2, "FAIL"))

    def test_nothing_matched_is_blocked(self):
        cmd = self.tool("sqlfluff", [])
        rc, ev = self.run_audit(self.config(cmd, paths='["nowhere/*.sql"]'))
        self.assertEqual((rc, ev["status"]), (2, "BLOCKED"))
        self.assertIn("nothing was checked", ev["violations"][0])

    def test_missing_tool_and_usage_error_are_blocked(self):
        rc, ev = self.run_audit(self.config(["no-such-sqlfluff-binary"]))
        self.assertEqual(ev["status"], "BLOCKED")
        cmd = self.tool("sqlfluff", [], exit_code=2)
        rc, ev = self.run_audit(self.config(cmd))
        self.assertEqual((rc, ev["status"]), (2, "BLOCKED"))

    def test_file_silently_skipped_by_the_tool_is_blocked(self):
        cmd = self.tool("sqlfluff", [])
        rc, ev = self.run_audit(self.config(cmd))
        self.assertEqual(ev["status"], "BLOCKED")
        self.assertTrue(any("reported nothing" in v for v in ev["violations"]))

    def test_squawk_findings_fail_unless_excluded_with_rationale(self):
        clean = self.tool("sqlfluff", [{"filepath": "db/q.sql", "violations": []}])
        finding = [
            {
                "file": "db/migrations/001.up.sql",
                "line": 0,
                "level": "Warning",
                "rule_name": "require-concurrent-index-creation",
                "message": "blocks writes",
            }
        ]
        squawk = self.tool("squawk", finding, exit_code=1)
        rc, ev = self.run_audit(
            self.config(clean, f'migration_paths = ["db/migrations/*.sql"]\nsquawk_command = {json.dumps(squawk)}\n')
        )
        self.assertEqual((rc, ev["status"]), (2, "FAIL"))
        squawk = self.tool("squawk", [], exit_code=0)
        extra = (
            f'migration_paths = ["db/migrations/*.sql"]\nsquawk_command = {json.dumps(squawk)}\n'
            '[sql_static.squawk_excluded_rules]\nrequire-concurrent-index-creation = "table is created in this migration"\n'
        )
        rc, ev = self.run_audit(self.config(clean, extra))
        self.assertEqual((rc, ev["status"]), (0, "PASS"), ev["violations"])
        self.assertIn("--exclude", ev["reproduction"]["squawk_command"])


@unittest.skipUnless(
    os.environ.get("SQLVERIFY_SQLFLUFF_CMD"),
    "set SQLVERIFY_SQLFLUFF_CMD (e.g. 'uvx sqlfluff==4.4.0') to run the real tool",
)
class RealToolTests(unittest.TestCase):
    def test_real_sqlfluff_and_squawk(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "db" / "migrations").mkdir(parents=True)
            (root / "db" / "ok.sql").write_text("SELECT o.id\nFROM orders AS o\nWHERE o.id = 1;\n", encoding="utf-8")
            (root / "db" / "broken.sql").write_text("SELEC id FROM orders;\n", encoding="utf-8")
            (root / "db" / "migrations" / "001.up.sql").write_text(
                "CREATE INDEX orders_id_idx ON orders (id);\n", encoding="utf-8"
            )
            squawk = os.environ.get("SQLVERIFY_SQUAWK_CMD")
            cfg = root / "c.toml"
            cfg.write_text(
                '[sql_static]\ndialect = "postgres"\npaths = ["db/*.sql"]\n'
                f"sqlfluff_command = {json.dumps(shlex.split(os.environ['SQLVERIFY_SQLFLUFF_CMD']))}\n"
                + (
                    f'migration_paths = ["db/migrations/*.sql"]\nsquawk_command = {json.dumps(shlex.split(squawk))}\n'
                    if squawk
                    else ""
                ),
                encoding="utf-8",
            )
            out = root / "e.json"
            subprocess.run(
                [
                    sys.executable,
                    str(SCRIPTS / "sql_static_audit.py"),
                    "--config",
                    str(cfg),
                    "--root",
                    str(root),
                    "--out",
                    str(out),
                ],
                capture_output=True,
                text=True,
                timeout=600,
            )
            ev = json.loads(out.read_text(encoding="utf-8"))
            self.assertEqual(ev["status"], "FAIL")
            self.assertEqual(
                [x["file"] for x in ev["observed"]["sqlfluff"]["syntax_errors"]],
                ["db/broken.sql"] * len(ev["observed"]["sqlfluff"]["syntax_errors"]),
            )
            self.assertTrue(ev["observed"]["sqlfluff"]["syntax_errors"])
            if squawk:
                self.assertIn(
                    "require-concurrent-index-creation", [x["rule"] for x in ev["observed"]["squawk"]["blocking"]]
                )


if __name__ == "__main__":
    unittest.main()
