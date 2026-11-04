#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""检查器自测：先确认示例通过，再往示例里故意埋漏洞（变异测试），确认检查器逐个抓到。

用法：python selftest.py        # 全部通过则退出码 0
"""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

import yaml

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import check_closure as cc  # noqa: E402

EXAMPLE = os.path.join(HERE, "assets", "examples", "example-closed.yaml")
base = yaml.safe_load(open(EXAMPLE, encoding="utf-8"))
fails = 0


def tr(s, ent, tid):
    return [t for t in s["entities"][ent]["transitions"] if t["id"] == tid][0]


def drop(s, ent, tid):
    s["entities"][ent]["transitions"] = [t for t in s["entities"][ent]["transitions"] if t["id"] != tid]
    for sc in s["scenarios"].values():
        sc["steps"] = [x for x in sc["steps"] if x != tid]


def case(name, mutate, expect):
    global fails
    s = copy.deepcopy(base)
    mutate(s)
    _, f, _ = cc.run_checks(s, 300000)
    got = sorted({x.rule for x in f if not x.waived and x.level != "INFO"})
    ok = all(r in got for r in expect)
    fails += (not ok)
    print(("通过" if ok else "失败"), f"| {name} | 期望 {expect} | 实际 {got}")


# 0. 基线必须零问题
_, f0, st0 = cc.run_checks(copy.deepcopy(base), 300000)
live0 = [x for x in f0 if not x.waived and x.level != "INFO"]
print(("通过" if not live0 else "失败"), f"| 示例基线应零问题 | 实际 {len(live0)} 个 | 状态空间 {st0 and st0['states']}")
fails += bool(live0)


def late_pay(s):
    drop(s, "支付单", "P-09")
    s["entities"]["支付单"]["fields"]["渠道流水号"]["writers"] = ["P-01"]
    tr(s, "支付单", "P-10")["kind"] = "exception"


def no_stock_back(s):
    drop(s, "库存锁定", "K-04")
    e = s["entities"]["库存锁定"]
    e["states"].remove("已回库")
    e["terminal"].remove("已回库")
    s["invariants"][0]["then"]["库存锁定"] = ["已释放"]


def dead_end(s):
    s["entities"]["订单"]["states"].append("待审核")
    s["entities"]["订单"]["transitions"].append(
        {"id": "O-99", "from": "待支付", "event": "用户取消", "actor": "用户", "to": "待审核", "kind": "exception", "feedback": "x"})


def loop(s):
    s["events"]["A"] = {"source": "internal"}
    s["events"]["B"] = {"source": "internal"}
    s["entities"]["订单"]["transitions"][0]["emits"] = ["A"]
    s["entities"]["库存锁定"]["transitions"] += [
        {"id": "K-90", "from": "已锁定", "event": "A", "actor": "系统", "to": "已锁定", "emits": ["B"]},
        {"id": "K-91", "from": "已锁定", "event": "B", "actor": "系统", "to": "已锁定", "emits": ["A"]}]


def no_exc(s):
    drop(s, "订单", "O-04")
    drop(s, "订单", "O-09")


def ignores_gone(s):
    for e in s["entities"].values():
        e.pop("ignores", None)


case("迟到支付无人处理", late_pay, ["R17"])
case("发货后取消，库存无人回补", no_stock_back, ["R17", "R15", "R14"])
case("取消不释放库存", lambda s: tr(s, "库存锁定", "K-02").pop("releases"), ["R16"])
case("已付款取消不退款", lambda s: drop(s, "支付单", "P-04"), ["R17", "R14"])
case("待发货无异常出口", no_exc, ["R8"])
case("商家不发货无超时兜底", lambda s: drop(s, "订单", "O-09"), ["R19"])
case("死胡同 + 同事件无guard", dead_end, ["R3", "R4"])
case("事件联动死循环", loop, ["R13"])
case("人工操作无反馈", lambda s: tr(s, "订单", "O-05").pop("feedback"), ["R18"])
case("竞态点无裁决策略", lambda s: s["entities"]["订单"].pop("race_policy"), ["R20"])
case("需求未被场景覆盖", lambda s: s["requirements"].update({"REQ-099": "孤儿需求"}), ["R12"])
case("事件无人处理", lambda s: s["events"].update({"孤儿事件": {"source": "internal"}}), ["R5"])
case("字段只写不读", lambda s: s["entities"]["订单"]["fields"].update({"备注": {"writers": ["@创建"]}}), ["R9"])
case("不变量被违反", lambda s: tr(s, "支付单", "P-04").update({"to": "已结算"}), ["R14"])
case("引用不存在的状态", lambda s: tr(s, "订单", "O-01").update({"to": "不存在"}), ["R6"])
case("异步事件无任何ignores", ignores_gone, ["R17"])


def cli_case(name, body):
    global fails
    try:
        with tempfile.TemporaryDirectory(prefix="closure-selftest-") as directory:
            body(Path(directory))
    except AssertionError as error:
        fails += 1
        print(f"失败 | {name} | {error}")
    else:
        print(f"通过 | {name}")


def invoke(directory, command, data=None, args=(), checker=None, expected=0):
    spec_path = directory / "spec.json"
    if data is not None:
        spec_path.write_text(json.dumps(data, ensure_ascii=False), encoding="utf-8")
    result = subprocess.run(
        [sys.executable, str(checker or Path(HERE) / "check_closure.py"), command, str(spec_path), *args],
        capture_output=True, text=True,
    )
    assert result.returncode == expected, f"{command}: exit={result.returncode}\n{result.stdout}\n{result.stderr}"
    return result


def lifecycle(directory):
    invoke(directory, "gate", base, expected=1)
    invoke(directory, "check")
    invoke(directory, "gate")
    with (directory / "spec.json").open("a", encoding="utf-8") as spec_file:
        spec_file.write("\n")
    invoke(directory, "gate", expected=1)
    invoke(directory, "check")
    invoke(directory, "gate")
    checker = directory / "checker.py"
    checker.write_text(Path(HERE, "check_closure.py").read_text(encoding="utf-8") + "\n# changed checker\n", encoding="utf-8")
    invoke(directory, "gate", checker=checker, expected=1)
    invoke(directory, "check", checker=checker)
    invoke(directory, "gate", checker=checker)


def diagnostic_warn(directory):
    data = copy.deepcopy(base)
    data["entities"]["订单"].pop("race_policy")
    invoke(directory, "check", data, args=("--allow-warn",))
    status = json.loads((directory / ".closure-status.json").read_text(encoding="utf-8"))
    assert status["summary"]["warns"] > 0 and status["passed"] is False
    invoke(directory, "gate", expected=1)
    status["passed"] = True
    (directory / ".closure-status.json").write_text(json.dumps(status), encoding="utf-8")
    invoke(directory, "gate", expected=1)


def incomplete_model(directory):
    data = copy.deepcopy(base)
    invoke(directory, "check", data, args=("--max-states", "1", "--allow-warn"), expected=1)
    invoke(directory, "gate", expected=1)
    data["waivers"].append({"rule": "R15", "target": "global", "reason": "截断不能豁免成完整证据"})
    invoke(directory, "check", data, args=("--max-states", "1", "--allow-warn"), expected=1)
    invoke(directory, "gate", expected=1)


def skipped_model(directory):
    data = copy.deepcopy(base)
    data["entities"]["订单"]["states"].append("未连接终态")
    data["entities"]["订单"]["terminal"].append("未连接终态")
    data["waivers"].append({"rule": "R2", "target": "订单.未连接终态", "reason": "结构豁免不能替代模型执行"})
    invoke(directory, "check", data, expected=1)
    status = json.loads((directory / ".closure-status.json").read_text(encoding="utf-8"))
    assert status["summary"]["errors"] == 0 and status["summary"]["warns"] == 0
    assert status["stats"] is None and status["passed"] is False
    invoke(directory, "gate", expected=1)


def invalid_record(directory):
    invoke(directory, "check", base)
    state_path = directory / ".closure-status.json"
    good = json.loads(state_path.read_text(encoding="utf-8"))
    invalid = [[], {}, {"passed": True, "spec_sha256": good["spec_sha256"]}]
    for key, value in [("checker_sha256", "old"), ("stats", None), ("summary", {}), ("passed", "true")]:
        changed = copy.deepcopy(good)
        changed[key] = value
        invalid.append(changed)
    for state in invalid:
        state_path.write_text(json.dumps(state), encoding="utf-8")
        invoke(directory, "gate", expected=1)
    state_path.write_text("{", encoding="utf-8")
    invoke(directory, "gate", expected=1)


def audit_outputs(directory):
    spec_path = directory / "as-is.json"
    spec_path.write_text(json.dumps(base, ensure_ascii=False), encoding="utf-8")
    artifact = directory / ".arc" / "artifacts" / "selftest" / "closure"
    artifact.mkdir(parents=True)
    result = subprocess.run([
        sys.executable, str(Path(HERE) / "check_closure.py"), "check", str(spec_path),
        "--report", str(artifact / "report.md"), "--status", str(artifact / "status.json"),
    ], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr + result.stdout
    assert (artifact / "report.md").is_file() and (artifact / "status.json").is_file()
    assert not (directory / "04-closure-report.md").exists()
    assert not (directory / ".closure-status.json").exists()
    assert not (directory / "docs").exists() and not (directory / "CLAUDE.md").exists()


def refresh_project_tool(directory):
    initializer = Path(HERE) / "init_docs.py"
    result = subprocess.run([sys.executable, str(initializer), str(directory)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    project = directory / "docs" / "01-系统设计" / "业务闭环"
    assert (project / "README.md").is_file() and not (directory / "docs" / "arc").exists()
    spec_path = project / "03-closure-spec.yaml"
    spec_path.write_text(yaml.safe_dump(base, allow_unicode=True), encoding="utf-8")
    tool = project / "tools" / "check_closure.py"
    tool.write_text(tool.read_text(encoding="utf-8") + "\n# old project copy\n", encoding="utf-8")
    for command in ("check", "gate"):
        result = subprocess.run([sys.executable, str(tool), command, str(spec_path)], capture_output=True, text=True)
        assert result.returncode == 0, result.stderr + result.stdout
    original_spec = spec_path.read_bytes()
    result = subprocess.run([sys.executable, str(initializer), str(directory), "--refresh-tool"], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    assert spec_path.read_bytes() == original_spec
    assert tool.read_bytes() == Path(HERE, "check_closure.py").read_bytes()
    assert not (directory / "CLAUDE.md").exists()
    assert not (project / "06-audit-report.md").exists()
    result = subprocess.run([sys.executable, str(tool), "gate", str(spec_path)], capture_output=True, text=True)
    assert result.returncode == 1, result.stderr + result.stdout


def unusable_input(directory):
    invoke(directory, "check", base, args=("--max-states", "0"), expected=2)
    (directory / "spec.json").write_text("{", encoding="utf-8")
    invoke(directory, "check", expected=2)
    invoke(directory, "gate", expected=2)
    missing = subprocess.run([sys.executable, str(Path(HERE) / "check_closure.py"), "check", str(directory / "absent.json")],
                             capture_output=True, text=True)
    assert missing.returncode == 2, missing.stdout + missing.stderr
    assert "无法读取 spec" in missing.stderr
    assert not (directory / ".closure-status.json").exists() and not (directory / "04-closure-report.md").exists()


def render_diagrams(directory):
    out = invoke(directory, "render", base).stdout
    assert out.count("stateDiagram-v2") == len(base["entities"]), out
    for name, entity in base["entities"].items():
        assert f"## {name}" in out and f"[*] --> {entity['initial']}" in out, name
        for t in entity["transitions"]:
            assert f"{t['from']} --> {t['to']}: {t['event']}[{t['actor']}]" in out, t["id"]
    assert not (directory / ".closure-status.json").exists()


cli_case("CLI 闸门绑定 spec 和检查器指纹", lifecycle)
cli_case("非法参数与不可读 spec 退出 2 且不写记录", unusable_input)
cli_case("render 为每个实体输出完整状态图且不写记录", render_diagrams)
cli_case("--allow-warn 和 passed 标志均不能绕过严格闸门", diagnostic_warn)
cli_case("截断或豁免截断均不能通过闸门", incomplete_model)
cli_case("结构豁免后未执行模型检查仍失败", skipped_model)
cli_case("缺失、旧版、损坏检查记录均关闭闸门", invalid_record)
cli_case("审查输出只写显式 artifacts 路径", audit_outputs)
cli_case("项目工具更新保留 spec 并使旧闸门失效", refresh_project_tool)


def legacy_docs_arc_kept(directory):
    legacy = directory / "docs" / "arc"
    legacy.mkdir(parents=True)
    (legacy / "03-closure-spec.yaml").write_text(yaml.safe_dump(base, allow_unicode=True), encoding="utf-8")
    result = subprocess.run([sys.executable, str(Path(HERE) / "init_docs.py"), str(directory)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    assert (legacy / "tools" / "check_closure.py").is_file()
    assert not (directory / "docs" / "01-系统设计").exists()


cli_case("已有 docs/arc 的旧项目沿用原路径", legacy_docs_arc_kept)

print("\n自测", "全部通过" if not fails else f"有 {fails} 项失败")
sys.exit(1 if fails else 0)
