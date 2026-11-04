"""Layout changes must preserve source relationships and visible exceptions."""

import hashlib
import json
import os
import re
import shutil
from collections import defaultdict
from copy import deepcopy
from itertools import combinations

import pytest
from arc_policy import diagram

DOMAIN = "fixture.domain.async"
CAPABILITIES = [
    ("fixture.cap.alerts", "告警核对"),
    ("fixture.cap.amount", "金额核对"),
    ("fixture.cap.callbacks", "回调核验"),
    ("fixture.cap.delivery", "签名推送"),
    ("fixture.cap.followup", "尾查更正"),
]


def _snapshot(*, extra_nodes=(), extra_edges=(), states=None, history_failures=0):
    """Invent a labelled tool fixture, never evidence about a real product."""
    nodes = [{"id": DOMAIN, "kind": "domain", "name": "异步业务测试域"}]
    nodes.extend(
        {"id": identity, "kind": "capability", "name": name}
        for identity, name in CAPABILITIES
    )
    nodes.extend(deepcopy(extra_nodes))
    edges = [
        {
            "id": f"fixture.contains.{identity.rsplit('.', 1)[-1]}",
            "from": DOMAIN,
            "to": identity,
            "kind": "contains",
        }
        for identity, _ in CAPABILITIES
    ]
    edges.extend(deepcopy(extra_edges))
    graph = {
        "id": "synthetic-layout-fixture-v2",
        "schema_version": 2,
        "scope": (
            "Synthetic layout regression fixture; not a product contract, "
            "an original DB snapshot, or evidence of business execution."
        ),
        "nodes": nodes,
        "edges": edges,
        "rules": [],
        "risks": [],
    }
    digest = hashlib.sha256(
        json.dumps(graph, ensure_ascii=False, sort_keys=True).encode("utf-8")
    ).hexdigest()
    projected = []
    for node in nodes:
        state = dict(node, status="NOT_RUN", history_failures=0, passed=0, required=0)
        state.update((states or {}).get(node["id"], {}))
        projected.append(state)
    return {
        "meta": {
            "graph": graph["id"],
            "graph_sha256": digest,
            "generated": "2026-11-04T00:00:00Z",
            "version": "synthetic-test",
        },
        "model": {"data": graph, "approved": False, "rules": {}, "sha256": digest},
        "views": {
            "capabilities": {
                "graph": graph["id"],
                "view": "capabilities",
                "nodes": projected,
                "edges": deepcopy(edges),
                "verdict": "BLOCKED",
                "reason": "Synthetic unapproved fixture; no business run was performed",
                "context": {},
                "history_failures": history_failures,
                "total": len(nodes),
                "truncated": False,
            }
        },
        "runs": [],
    }


def _presentation(snapshot):
    return {
        "graph": snapshot["meta"]["graph"],
        "graph_sha256": snapshot["meta"]["graph_sha256"],
        "nodes": {},
    }


def _mixed_relation_fixture():
    role = {"id": "fixture.role.reviewer", "kind": "role", "name": "复核专员"}
    edge = {
        "id": "fixture.serves.review",
        "from": role["id"],
        "to": CAPABILITIES[1][0],
        "kind": "serves",
    }
    snapshot = _snapshot(extra_nodes=[role], extra_edges=[edge])
    presentation = _presentation(snapshot)
    presentation["edges"] = {edge["id"]: {"label_zh": "业务专员参与异常金额核对"}}
    return snapshot, presentation


def _verified_packages(tmp_path, presentation, claims):
    """Create real local manifest evidence for explicitly synthetic packages."""
    for index, (identity, packages) in enumerate(claims.items()):
        entries = []
        for package_index, name in enumerate(packages):
            relative = f"evidence/node-{index}/package-{package_index}/package.json"
            path = tmp_path / relative
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps({"name": name}) + "\n", encoding="utf-8")
            entries.append({"name": name, "file": relative, "line": 1})
        presentation["nodes"].setdefault(identity, {})["packages"] = entries
    facts, warnings = diagram._package_facts(presentation, tmp_path)
    assert not warnings
    assert set(facts) == set(claims)
    return facts


def _rendered_by_source(spec):
    elements = [*spec["groups"], *spec["nodes"]]
    result = {element["sourceId"]: element for element in elements}
    assert len(result) == len(elements), "a source node was duplicated in the diagram"
    assert spec["arc"]["node_ids"] == {
        element["id"]: element["sourceId"] for element in elements
    }
    return result


def _card_text(card):
    # Only renderer-visible fields: sourceId remains legitimate hidden metadata.
    parts = [
        str(card.get(key, ""))
        for key in ("title", "sub", "text", "footer", "statusText")
    ]
    parts.extend(card.get("lines", []))
    parts.extend(card.get("pipeline", []))
    for row in card.get("rows", []):
        parts.extend(str(row.get(key, "")) for key in ("name", "ref", "desc", "tag"))
    return "\n".join(parts)


def _overview_text(spec):
    return "\n".join(
        [spec["title"], spec["subtitle"], *(note["text"] for note in spec["notes"])]
    ).replace("\n", "")


def _source_relations(spec):
    mapping = spec["arc"]["edge_rendering"]
    assert set(mapping) == set(spec["arc"]["edge_ids"])
    assert all(
        item["source_id"] == spec["arc"]["edge_ids"][renderer]
        for renderer, item in mapping.items()
    )
    result = {item["source_id"]: item for item in mapping.values()}
    assert len(result) == len(mapping), "a canonical relation was duplicated"
    return result


def _assert_inside(child, parent):
    assert parent["x"] < child["x"]
    assert parent["y"] < child["y"]
    assert child["x"] + child["w"] < parent["x"] + parent["w"]
    assert child["y"] + child["h"] < parent["y"] + parent["h"]


def _assert_disjoint(first, second):
    assert (
        first["x"] + first["w"] <= second["x"]
        or second["x"] + second["w"] <= first["x"]
        or first["y"] + first["h"] <= second["y"]
        or second["y"] + second["h"] <= first["y"]
    ), f"overlapping siblings: {first['sourceId']} and {second['sourceId']}"


def test_five_siblings_use_balanced_containment_without_losing_source_relations():
    snapshot = _snapshot()
    original = deepcopy(snapshot)
    spec = diagram.snapshot_to_spec(snapshot, view="capabilities")
    rendered = _rendered_by_source(spec)
    graph = snapshot["model"]["data"]

    assert snapshot == original
    assert set(rendered) == {node["id"] for node in graph["nodes"]}
    assert {node["id"]: node for node in spec["arc"]["source_nodes"]} == {
        node["id"]: node for node in graph["nodes"]
    }
    assert spec["arc"]["fixture"] is True
    assert "工具示例" in _overview_text(spec)
    assert spec["arc"]["group_ids"] == {rendered[DOMAIN]["id"]: DOMAIN}
    assert spec["edges"] == [], "contains must not masquerade as process arrows"

    relations = _source_relations(spec)
    assert set(relations) == {edge["id"] for edge in graph["edges"]}
    for edge in graph["edges"]:
        represented = relations[edge["id"]]
        assert represented["representation"] == "containment"
        assert spec["arc"]["node_ids"][represented["from"]] == edge["from"]
        assert spec["arc"]["node_ids"][represented["to"]] == edge["to"]

    group = rendered[DOMAIN]
    siblings = [rendered[identity] for identity, _ in CAPABILITIES]
    for child in siblings:
        _assert_inside(child, group)
    for first, second in combinations(siblings, 2):
        _assert_disjoint(first, second)
    rows = defaultdict(list)
    for child in siblings:
        rows[child["y"]].append(child)
    assert 1 < len(rows) <= 2, "five siblings should not occupy three sparse rows"
    final_row = rows[max(rows)]
    assert len(final_row) < len(rows[min(rows)])
    row_center = (
        min(child["x"] for child in final_row)
        + max(child["x"] + child["w"] for child in final_row)
    ) / 2
    assert row_center == pytest.approx(group["x"] + group["w"] / 2, abs=1)


def test_parent_failure_stays_visible_while_card_audit_metadata_moves_to_evidence(
    tmp_path,
):
    snapshot = _snapshot(
        states={DOMAIN: {"status": "FAIL", "history_failures": 2}}, history_failures=2
    )
    presentation = _presentation(snapshot)
    facts = _verified_packages(
        tmp_path,
        presentation,
        {
            identity: [f"@layout-fixture/{identity.rsplit('.', 1)[-1]}"]
            for identity, _ in CAPABILITIES
        },
    )
    spec = diagram.snapshot_to_spec(
        snapshot, presentation, "capabilities", package_facts=facts
    )
    group = _rendered_by_source(spec)[DOMAIN]

    assert "FAIL" in group["label"]
    assert re.search(r"历史\s*FAIL[：:\s]*2", group["label"].replace("\n", ""))
    assert "历史 FAIL：2" in _overview_text(spec)
    assert "FAIL 1 个" in _overview_text(spec)
    assert spec["arc"]["history_failures"] == 2
    assert spec["arc"]["package_evidence"] == facts
    assert spec["arc"]["node_status"] == {
        node["id"]: node for node in snapshot["views"]["capabilities"]["nodes"]
    }
    for card in spec["nodes"]:
        text = _card_text(card).replace("\n", "")
        assert "候选" in text
        assert "历史 FAIL" not in text
        assert "源 ID" not in text
        assert "包名来源" not in text
        assert all(
            node["id"] not in text for node in snapshot["model"]["data"]["nodes"]
        )
        assert all(
            item["file"] not in text for entries in facts.values() for item in entries
        )


@pytest.mark.parametrize("shape", ["multiple_parents", "cycle", "self_cycle"])
def test_non_tree_containment_falls_back_without_dropping_source_edges(shape):
    extra_nodes = []
    if shape == "multiple_parents":
        other = "fixture.domain.shared"
        extra_nodes = [{"id": other, "kind": "domain", "name": "共享业务测试域"}]
        start, end = other, CAPABILITIES[0][0]
    elif shape == "cycle":
        start, end = CAPABILITIES[0][0], DOMAIN
    else:
        start = end = DOMAIN
    snapshot = _snapshot(
        extra_nodes=extra_nodes,
        extra_edges=[
            {
                "id": "fixture.contains.non_tree",
                "from": start,
                "to": end,
                "kind": "contains",
            }
        ],
    )
    spec = diagram.snapshot_to_spec(snapshot, view="capabilities")
    graph = snapshot["model"]["data"]

    assert not spec["groups"]
    assert set(_rendered_by_source(spec)) == {node["id"] for node in graph["nodes"]}
    relations = _source_relations(spec)
    assert set(relations) == {edge["id"] for edge in graph["edges"]}
    assert all(item["representation"] == "wire" for item in relations.values())
    assert {edge["sourceId"] for edge in spec["edges"]} == set(relations)
    assert {edge["id"]: edge for edge in spec["arc"]["source_edges"]} == {
        edge["id"]: edge for edge in graph["edges"]
    }
    assert not spec["arc"]["omitted"]["edges"]
    assert "显式关系" in _overview_text(spec)


def test_parallel_contains_edges_share_geometry_but_keep_each_canonical_identity():
    parallel = {
        "id": "fixture.contains.alerts.second_source",
        "from": DOMAIN,
        "to": CAPABILITIES[0][0],
        "kind": "contains",
    }
    snapshot = _snapshot(extra_edges=[parallel])
    spec = diagram.snapshot_to_spec(snapshot, view="capabilities")

    assert len(_rendered_by_source(spec)) == len(snapshot["model"]["data"]["nodes"])
    assert len(spec["nodes"]) == len(CAPABILITIES)
    assert not spec["edges"]
    relations = _source_relations(spec)
    assert set(relations) == {edge["id"] for edge in snapshot["model"]["data"]["edges"]}
    first = relations["fixture.contains.alerts"]
    second = relations[parallel["id"]]
    assert first["representation"] == second["representation"] == "containment"
    assert (first["from"], first["to"]) == (second["from"], second["to"])
    assert "6/6 条关系" in _overview_text(spec)


def test_real_serves_relation_keeps_its_source_endpoints_and_chinese_wire_label():
    snapshot, presentation = _mixed_relation_fixture()
    edge = next(
        item for item in snapshot["model"]["data"]["edges"] if item["kind"] == "serves"
    )
    label = presentation["edges"][edge["id"]]["label_zh"]
    spec = diagram.snapshot_to_spec(snapshot, presentation, "capabilities")

    assert spec["groups"]
    _rendered_by_source(spec)
    assert len(spec["edges"]) == 1
    wire = spec["edges"][0]
    assert wire["sourceId"] == edge["id"]
    assert wire["flow"] == "serves"
    assert wire["packet"] is False
    assert wire["label"].replace("\n", "") == label
    assert spec["arc"]["node_ids"][wire["from"].split(".", 1)[0]] == edge["from"]
    assert spec["arc"]["node_ids"][wire["to"].split(".", 1)[0]] == edge["to"]
    assert _source_relations(spec)[edge["id"]]["representation"] == "wire"
    assert "6/6 条关系" in _overview_text(spec)


@pytest.mark.skipif(
    not os.environ.get("CHROME") or not (shutil.which("node") or shutil.which("bun")),
    reason="set CHROME and provide Node/Bun to run the official browser geometry check",
)
def test_mixed_containment_and_serves_label_pass_official_browser_check(tmp_path):
    """A role below a group must not put its label across the lower siblings."""
    snapshot, presentation = _mixed_relation_fixture()
    source_path = tmp_path / "snapshot.json"
    source_path.write_text(json.dumps(snapshot, ensure_ascii=False), encoding="utf-8")
    labels_path = tmp_path / "presentation.json"
    labels_path.write_text(
        json.dumps(presentation, ensure_ascii=False), encoding="utf-8"
    )
    result = diagram.build_diagram(
        source_path,
        tmp_path / "rendered",
        presentation=labels_path,
        view="capabilities",
        root=tmp_path,
        check=True,
    )

    assert result["readiness"]["layout_check"] == "PASS", (
        result.get("reason"),
        result.get("check"),
    )


@pytest.mark.parametrize("identity", [DOMAIN, CAPABILITIES[0][0]])
def test_long_chinese_and_multiple_actual_package_identities_survive_wrapping(
    tmp_path, identity
):
    snapshot = _snapshot()
    presentation = _presentation(snapshot)
    name = "多来源回调校验与异常金额差异核对及后续补偿处理完整闭环"
    packages = [
        "@layout-fixture/reconciliation-long-name",
        "example.test/callbacks/receipt_delivery",
    ]
    presentation["nodes"][identity] = {"name_zh": name}
    facts = _verified_packages(tmp_path, presentation, {identity: packages})
    spec = diagram.snapshot_to_spec(
        snapshot, presentation, "capabilities", package_facts=facts
    )
    element = _rendered_by_source(spec)[identity]
    visible_name = element["label"] if "label" in element else element["title"]

    assert "\n" in visible_name
    assert f"{name}【{'、'.join(packages)}】" in visible_name.replace("\n", "")
    assert spec["arc"]["package_evidence"][identity] == facts[identity]
    assert all(
        item["file_sha256"] for item in spec["arc"]["package_evidence"][identity]
    )


@pytest.mark.parametrize("selected_children", [None, 2])
def test_displayed_and_omitted_counts_follow_source_relations_not_wire_count(
    selected_children,
):
    snapshot = _snapshot()
    presentation = _presentation(snapshot)
    if selected_children is not None:
        presentation["node_ids"] = [
            DOMAIN,
            *(identity for identity, _ in CAPABILITIES[:selected_children]),
        ]
    spec = diagram.snapshot_to_spec(snapshot, presentation, "capabilities")
    graph = snapshot["model"]["data"]
    selected = set(
        presentation.get("node_ids", [node["id"] for node in graph["nodes"]])
    )
    shown = {
        edge["id"]
        for edge in graph["edges"]
        if edge["from"] in selected and edge["to"] in selected
    }
    omitted = {edge["id"] for edge in graph["edges"]} - shown

    assert spec["edges"] == []
    assert set(_source_relations(spec)) == shown
    assert set(spec["arc"]["omitted"]["edges"]) == omitted
    assert (
        set(spec["arc"]["omitted"]["nodes"])
        == {node["id"] for node in graph["nodes"]} - selected
    )
    assert (
        f"{len(selected)}/{len(graph['nodes'])} 个节点，{len(shown)}/{len(graph['edges'])} 条关系"
        in _overview_text(spec)
    )
    assert f"关系 {len(shown)}/{len(graph['edges'])}" in "\n".join(
        spec["arc"]["presentation"]["provenance"]
    )
    if omitted:
        assert (
            f"省略 {len(graph['nodes']) - len(selected)} 个节点、{len(omitted)} 条关系"
            in _overview_text(spec)
        )
