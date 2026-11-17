const labels: Record<string, string> = {
  PASS: "通过",
  PARTIAL: "部分验证",
  FAIL: "确认失败",
  NOT_RUN: "尚未验证",
  BLOCKED: "阻塞",
  STALE: "待复验",
  FLAKY: "结果不稳定",
  EXCLUDED: "模型排除",
  LEGACY: "历史证据",
  UNKNOWN: "未知",
  UNSPECIFIED: "未定义",
  CONFLICT: "规则冲突",
};
export function Status({ value, raw = false }: { value: string; raw?: boolean }) {
  return (
    <span className={`status status-${value.toLowerCase()}`}>
      <span aria-hidden="true" />
      {raw ? value : (labels[value] ?? value)}
    </span>
  );
}
