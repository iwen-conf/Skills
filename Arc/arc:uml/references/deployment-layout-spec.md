# 部署图结构化规格

当部署图需要画得整齐、稳定、可重复时，不要让模型直接脑补 `drawio` XML 坐标。先整理结构化规格，再调用生成脚本：

```bash
python3 Arc/arc:uml/scripts/generate_deployment_drawio.py \
  --spec diagram-specs/deployment.json \
  --output diagrams/deployment.drawio
```

## 目标

- 把“画什么”与“怎么排版”拆开
- 由结构化规格描述分区、节点和连接
- 由脚本统一计算列布局、节点间距和正交连线

## 规格示例

```json
{
  "title": "支付订单部署图",
  "zones": [
    {
      "id": "client",
      "label": "客户端区",
      "nodes": [
        {
          "id": "frontend",
          "label": "前端页面",
          "kind": "client",
          "subtitle": "React"
        }
      ]
    },
    {
      "id": "app",
      "label": "应用层",
      "nodes": [
        {
          "id": "orders",
          "label": "Orders Service",
          "kind": "service"
        },
        {
          "id": "payment",
          "label": "Payment Gateway",
          "kind": "gateway",
          "subtitle": "支付接入"
        }
      ]
    },
    {
      "id": "data",
      "label": "数据层",
      "nodes": [
        {
          "id": "mysql",
          "label": "MySQL",
          "kind": "database"
        }
      ]
    }
  ],
  "edges": [
    {
      "source": "frontend",
      "target": "orders",
      "label": "HTTPS"
    },
    {
      "source": "orders",
      "target": "payment",
      "label": "支付请求"
    },
    {
      "source": "orders",
      "target": "mysql",
      "label": "订单读写"
    }
  ]
}
```

## 更省事的 seed 文本

如果你不想先手写 JSON，可以先写一份简洁种子文本，再转换成 `deployment.json`：

```text
title: 支付订单部署图

zone client | 客户端区
node frontend | 前端页面 | client | React

zone app | 应用层
node orders | Orders Service | service
node payment | Payment Gateway | gateway | 支付接入

zone data | 数据层
node mysql | MySQL | database

edge frontend -> orders | HTTPS
edge orders -> payment | 支付请求
edge orders -> mysql | 订单读写
```

转换命令：

```bash
python3 Arc/arc:uml/scripts/draft_deployment_spec.py \
  --input diagram-specs/deployment.seed.txt \
  --output diagram-specs/deployment.json
```

## 字段说明

### 顶层字段

- `title`: 图标题
- `zones`: 列分区数组，必须非空
- `edges`: 连接数组，可为空但通常不建议

### `zones[]`

- `id`: 分区唯一标识
- `label`: 分区标题
- `nodes`: 节点数组，必须非空

### `zones[].nodes[]`

- `id`: 节点唯一标识
- `label`: 节点标题
- `kind`: 节点类型
  - 可选值：`client`、`gateway`、`service`、`runtime`、`queue`、`database`、`external`
- `subtitle`: 节点补充说明，可选

### `edges[]`

- `source`: 起点节点 id
- `target`: 终点节点 id
- `label`: 连线标签，可选

## 生成规则

- 每个 `zone` 固定为一列，按数组顺序从左到右排布
- 每列节点统一宽度，按纵向等间距堆叠
- 连线统一使用正交路由
- 连线必须绑定 `source/target`
- 同列连接默认走上下方向，不跨列漂浮

## 适用边界

- 适合应用拓扑、系统边界、服务关系、数据库和中间件连接表达
- 不适合需要复杂嵌套容器、自由形态网络拓扑或非正交艺术化排版的场景
- 如果一列内节点超过 5 到 7 个，建议拆成子图，避免单列过高
