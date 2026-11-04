"""Read-only review must preserve source evidence without creating page code."""

import base64
import json
import unittest
from copy import deepcopy
from html.parser import HTMLParser
from pathlib import Path
from unittest.mock import patch

from arc_policy.diagram import snapshot_to_spec
from arc_policy.diagram_review import render_review


class ReviewParser(HTMLParser):
    """Inspect the wrapper only; srcdoc is an attribute, not wrapper markup."""

    def __init__(self, source):
        super().__init__(convert_charrefs=True)
        self.elements = []
        self.visible = []
        self.spec = []
        self.detail_depth = 0
        self.style_depth = 0
        self.in_spec = False
        self.feed(source)

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        self.elements.append((tag, attrs))
        if tag == "details":
            self.detail_depth += 1
        if tag in {"style", "script"}:
            self.style_depth += 1
        if tag == "pre" and attrs.get("id") == "source-spec":
            self.in_spec = True

    def handle_endtag(self, tag):
        if tag == "details":
            self.detail_depth -= 1
        if tag in {"style", "script"}:
            self.style_depth -= 1
        if tag == "pre":
            self.in_spec = False

    def handle_data(self, data):
        if not self.detail_depth and not self.style_depth:
            self.visible.append(data)
        if self.in_spec:
            self.spec.append(data)

    def attrs(self, tag):
        return [attrs for name, attrs in self.elements if name == tag]


class ReviewTests(unittest.TestCase):
    def setUp(self):
        self.spec = {
            "title": "报名业务图",
            "status": "READY",  # visual readiness must never supply the business verdict
            "nodes": [
                {"id": "n1", "title": "提交报名【enrollment】", "status": "done"}
            ],
            "arc": {
                "read_only": True,
                "view": "flows",
                "meta": {
                    "graph": "course-v2",
                    "generated": "2026-11-04T08:00:00Z",
                    "graph_sha256": "a" * 64,
                },
                "business_verdict": "BLOCKED",
                "business_reason": "独立业务证据未齐备",
                "scope": "报名、拒绝与恢复路径",
                "model_approved": False,
                "candidate_nodes": ["enrollment.submit"],
                "history_failures": 2,
                "node_ids": {"n1": "enrollment.submit"},
                "edge_ids": {"e1": "requires.login"},
                "source_nodes": [
                    {"id": "enrollment.submit", "name": "提交报名", "kind": "action"}
                ],
                "node_status": {
                    "enrollment.submit": {
                        "status": "FLAKY",
                        "history_failures": 2,
                        "passed": 1,
                        "required": 3,
                    }
                },
                "source_edges": [
                    {
                        "id": "requires.login",
                        "from": "enrollment.submit",
                        "to": "condition.login",
                        "kind": "requires",
                    }
                ],
                "rules": [
                    {
                        "id": "RULE.enroll",
                        "action": "enrollment.submit",
                        "conditions": ["condition.login"],
                        "expected": "enrolled",
                        "rejected": "UNCHANGED",
                    }
                ],
                "rule_approvals": {"RULE.enroll": False},
                "business_logic": {
                    "guards": [
                        {
                            "rule": "RULE.enroll",
                            "any_of": [
                                [
                                    {"condition": "condition.login", "value": True},
                                    {"condition": "suspended", "value": False},
                                ],
                                [{"condition": "override", "value": True}],
                            ],
                        }
                    ],
                    "constraints": [
                        {
                            "id": "C1",
                            "literals": [
                                {"condition": "full", "value": False},
                                {"condition": "waitlist", "value": True},
                            ],
                        }
                    ],
                    "history": [{"previous": "course-v1", "failure": "R-old"}],
                },
                "package_evidence": {
                    "enrollment.submit": [
                        {
                            "name": "enrollment",
                            "file": "src/enrollment/service.go",
                            "line": 12,
                            "kind": "directory",
                            "file_sha256": "b" * 64,
                        }
                    ]
                },
                "runs": [
                    {
                        "run": "R-failed",
                        "status": "FAIL",
                        "first_divergence": "step-1",
                        "evidence_status": "PASS",
                        "steps": [
                            {
                                "step": "step-1",
                                "node": "enrollment.submit",
                                "verdict": "FAIL",
                                "expected": "ready",
                                "observed": "pending",
                                "raw": "/project/.arc/raw/fail.json",
                                "sha256": "c" * 64,
                                "evidence_status": "PASS",
                            }
                        ],
                        "evidence": [
                            {
                                "phase": "after",
                                "step": "step-1",
                                "sha256": "c" * 64,
                                "error": "RAW digest mismatch",
                            }
                        ],
                    },
                    {
                        "run": "R-retry",
                        "status": "PASS",
                        "evidence_status": "PASS",
                        "steps": [],
                        "evidence": [],
                    },
                ],
                "omitted": {
                    "nodes": ["omitted.role"],
                    "edges": ["omitted.edge"],
                    "logic": ["long.guard"],
                    "projection_truncated": True,
                },
                "snapshot": {
                    "path": "/project/review.snapshot.json",
                    "sha256": "d" * 64,
                },
                "renderer": {
                    "repository": "https://github.com/bassnova42/arch-diagram-skill",
                    "version": "1.2.3",
                    "commit": "pinned",
                    "license_text": "MIT notice",
                },
                "presentation": {
                    "compact_notes": True,
                    "details": {"enrollment.submit": ["evidence retained"]},
                    "containment": {"group1": ["enrollment.submit"]},
                },
            },
        }
        self.original_html = '<!doctype html>\r\n<html><head><style>body { color: #c9d1d9 }</style></head><body><div id="stage">业务图 &amp; evidence</div><script>window.result = "</iframe> & \\" preserved";</script></body></html>'

    def test_renderer_and_full_spec_are_preserved_without_mutation(self):
        before = deepcopy(self.spec)
        page = render_review(self.spec, self.original_html, diagram_height=845)
        parsed = ReviewParser(page)
        self.assertEqual(parsed.attrs("iframe")[0]["srcdoc"], self.original_html)
        self.assertEqual(parsed.attrs("iframe")[0]["height"], "845")
        self.assertEqual(json.loads("".join(parsed.spec)), before)
        download = parsed.attrs("a")[0]
        self.assertTrue(download["href"].startswith("data:application/json;base64,"))
        self.assertEqual(
            json.loads(base64.b64decode(download["href"].split(",", 1)[1])), before
        )
        self.assertEqual(self.spec, before)
        for retained in (
            "omitted.role",
            "omitted.edge",
            "requires.login",
            "RULE.enroll",
            "/project/.arc/raw/fail.json",
            "src/enrollment/service.go:12",
            "MIT notice",
        ):
            self.assertIn(retained, page)

    def test_business_failures_and_scope_are_visible_without_opening_details(self):
        parsed = ReviewParser(render_review(self.spec, self.original_html))
        visible = " ".join(parsed.visible)
        for value in (
            "BLOCKED",
            "独立业务证据未齐备",
            "2026-11-04T08:00:00Z",
            "候选目录",
            "历史 FAIL",
            "FLAKY 1",
            "R-failed",
            "step-1",
            "RAW digest mismatch",
            "省略节点 1",
            "来源投影已截断",
        ):
            self.assertIn(value, visible)
        self.assertNotIn("READY", visible)
        self.assertIn("图形检查和 RAW 完整性均不替代业务判定", visible)
        self.assertTrue(all("open" not in attrs for attrs in parsed.attrs("details")))

    def test_untrusted_text_and_attribute_closures_do_not_create_wrapper_markup(self):
        attack = '"><script>window.parent.pwned=true</script></iframe><img src="https://attacker.invalid/leak" onerror="alert(1)">&\'\n</script>'
        arc = self.spec["arc"]
        self.spec["title"] = attack
        arc["meta"]["generated"] = attack
        arc["scope"] = attack
        arc["warnings"] = [attack]
        arc["diagnostics"] = [{"message": attack}]
        arc["runs"][0]["first_divergence"] = attack
        arc["package_evidence"]["enrollment.submit"][0]["file"] = attack
        arc["source_nodes"][0]["name"] = attack
        arc["source_edges"][0]["id"] = attack
        arc["presentation"]["containment"][attack] = [attack]
        parsed = ReviewParser(render_review(self.spec, self.original_html))
        self.assertEqual(len(parsed.attrs("iframe")), 1)
        self.assertEqual(parsed.attrs("iframe")[0]["srcdoc"], self.original_html)
        self.assertFalse(parsed.attrs("script"))
        self.assertFalse(parsed.attrs("img"))
        self.assertFalse(
            any(
                key.lower().startswith("on")
                for _, attrs in parsed.elements
                for key in attrs
            )
        )
        self.assertEqual(json.loads("".join(parsed.spec)), self.spec)
        self.assertIn(attack, "".join(parsed.visible))

    def test_offline_sandbox_and_no_mutation_controls_or_file_access(self):
        with (
            patch(
                "builtins.open",
                side_effect=AssertionError("must not open source files"),
            ),
            patch(
                "subprocess.run",
                side_effect=AssertionError("must not execute commands"),
            ),
        ):
            parsed = ReviewParser(render_review(self.spec, self.original_html))
        iframe = parsed.attrs("iframe")[0]
        self.assertEqual(iframe["sandbox"], "allow-scripts")
        self.assertEqual(iframe["referrerpolicy"], "no-referrer")
        csp = next(
            attrs["content"]
            for attrs in parsed.attrs("meta")
            if attrs.get("http-equiv") == "Content-Security-Policy"
        )
        for directive in (
            "default-src 'none'",
            "connect-src 'none'",
            "form-action 'none'",
            "base-uri 'none'",
            "font-src 'none'",
            "object-src 'none'",
        ):
            self.assertIn(directive, csp)
        for tag in (
            "form",
            "button",
            "input",
            "textarea",
            "select",
            "script",
            "link",
            "object",
            "embed",
        ):
            self.assertFalse(parsed.attrs(tag), tag)
        self.assertFalse(any("src" in attrs for _, attrs in parsed.elements))
        self.assertEqual(len(parsed.attrs("a")), 1)  # the local data download only

    def test_logic_keeps_and_or_and_false_literals(self):
        page = render_review(self.spec, self.original_html)
        self.assertIn(
            "（condition.login=满足 且 suspended=不满足） 或 （override=满足）", page
        )
        self.assertIn("full=不满足 或 waitlist=满足", page)
        self.assertIn("各约束须同时成立", page)
        self.assertIn("展示映射与包含关系", page)

    def test_visual_check_readiness_is_supplied_and_kept_separate(self):
        readiness = {
            "spec": "READY",
            "build": "READY",
            "layout_check": "FAIL",
            "screenshot": "READY",
            "visual_review": "NOT_RUN",
        }
        before = deepcopy(readiness)
        parsed = ReviewParser(
            render_review(self.spec, self.original_html, readiness=readiness)
        )
        visible = " ".join(parsed.visible)
        self.assertIn("阻塞 · BLOCKED", visible)
        self.assertIn("布局检查： FAIL", visible)
        self.assertIn("目视核对： NOT_RUN", visible)
        self.assertIn("业务结论见上方快照记录", visible)
        self.assertEqual(readiness, before)
        self.assertEqual(json.loads("".join(parsed.spec)), self.spec)
        parsed = ReviewParser(render_review(self.spec, self.original_html))
        self.assertIn("图形检查状态未提供", " ".join(parsed.visible))

    def test_containment_uses_canonical_sources_and_native_mobile_width(self):
        self.spec["width"] = 1240
        arc = self.spec["arc"]
        arc["presentation"]["names"] = {
            "enrollment.submit": "核对报名申请",
            "condition.login": "已登录",
        }
        arc["presentation"]["relationships"] = [
            "requires.login：核对报名申请需要已登录"
        ]
        arc["group_ids"] = {"g1": "enrollment.submit"}
        arc["node_ids"] = {"g1": "enrollment.submit"}
        arc["edge_rendering"] = {
            "e1": {
                "source_id": "requires.login",
                "representation": "containment",
                "from": "g1",
                "to": "n2",
            }
        }
        parsed = ReviewParser(render_review(self.spec, self.original_html))
        self.assertEqual(parsed.attrs("iframe")[0]["style"], "min-width:1272px")
        page = render_review(self.spec, self.original_html)
        self.assertIn("核对报名申请 → 已登录", page)
        self.assertIn("包含区域", page)
        self.assertIn("<code>g1</code>", page)
        self.assertEqual(
            json.loads("".join(parsed.spec))["arc"]["edge_rendering"],
            arc["edge_rendering"],
        )

    def test_missing_optional_fields_do_not_invent_pass_or_approval(self):
        for spec in (
            {},
            {"arc": None},
            {"arc": {"runs": None, "omitted": None, "source_nodes": None}},
        ):
            parsed = ReviewParser(render_review(spec, "<html></html>"))
            visible = " ".join(parsed.visible)
            self.assertIn("未提供状态 · UNKNOWN", visible)
            self.assertIn("确认状态未提供", visible)
            self.assertIn("省略范围未提供", visible)
            self.assertIn("本快照未提供运行记录", visible)
            self.assertEqual(json.loads("".join(parsed.spec)), spec)

    def test_height_rejects_code_and_boolean_but_allows_known_measurements(self):
        for height in ('1" onload="alert(1)', True, 0, -1, 1.5):
            with self.assertRaises(ValueError):
                render_review(self.spec, self.original_html, height)
        with self.assertRaises(TypeError):
            render_review([], self.original_html)

    def test_current_arc_snapshot_contract_has_failed_attempt_and_retry(self):
        path = (
            Path(__file__).resolve().parents[1]
            / "tools/arc-verify/workbench/dev/sample-snapshot.json"
        )
        spec = snapshot_to_spec(json.loads(path.read_text(encoding="utf-8")))
        parsed = ReviewParser(render_review(spec, self.original_html))
        visible = " ".join(parsed.visible)
        self.assertIn("BLOCKED", visible)
        self.assertIn("工具示例／合成夹具", visible)
        self.assertIn(spec["arc"]["runs"][0]["run"], visible)
        stored = json.loads("".join(parsed.spec))
        self.assertEqual(
            [run["status"] for run in stored["arc"]["runs"]], ["FAIL", "PASS"]
        )
        self.assertEqual(stored["arc"]["runs"][0]["evidence_status"], "PASS")
        self.assertEqual(stored["arc"]["history_failures"], 1)


if __name__ == "__main__":
    unittest.main()
