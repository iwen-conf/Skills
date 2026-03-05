# **SPA 项目评估大盘标准化需求规格说明书 (V1.0)**

## **1\. 概述 (Overview)**

本项目旨在构建一个专家级的项目评估大盘，采用单页面应用（SPA）架构。大盘基于“全局上帝视角 \+ 多维深度下钻”的设计理念，通过九个核心 Tab 模块（1个全局总览 \+ 4个业务深度剖析 \+ 4个底层技术与效能维度），全面、实时、可视化地展示系统的健康度、业务流转及底层技术状态。

## **2\. 全局标准与非功能性要求 (Global Standards & NFRs)**

* **架构标准**：采用前后端分离，前端 SPA（React/Vue3）配合 WebGL 驱动的可视化库（如 Apache ECharts, G6, D3.js） \+ BFF 层（Backend for Frontend，数据聚合网关）。前端仅消费标准化 JSON，不做复杂数据计算。  
* **交互规范**：  
  * **全局联动**：顶部提供统一的“时间沙盘”和“环境/租户筛选”，切换时九个 Tab 数据无缝联动更新。  
  * **渐进式下钻**：所有图表支持点击下钻（Drill-down），从宏观指标直达底层 Trace ID 或具体 API 节点。  
* **页面生命周期规范**：首屏内容渲染 (FCP) ![][image1] 秒；图表渲染延迟 ![][image2] 毫秒（支持 10,000+ 节点拓扑图 WebGL 硬件加速渲染）。  
* **UI/UX 规范**：  
  * **网格系统**：24 栅格系统，卡片式布局，组件间距标准化 (16px/24px)。  
  * **状态色板**：统一采用标准语义色。成功 (Green \#52C41A)，警告 (Yellow \#FAAD14)，危险/阻断 (Red \#F5222D)，离线/未知 (Grey \#BFBFBF)。

## **3\. 核心模块标准化定义 (9 Tabs)**

所有 Tab 遵循“**定位 \-\> 标准指标 \-\> 可视化组件规范**”的标准范式。

### **Tab 1: 全局总览 (Executive Overview)**

* **定位**：给决策层看的“一屏知天下”，高度聚合七大维度的健康分数与核心业务 KPI。  
* **标准指标 (KPIs/SLIs)**：  
  * **Apdex (应用性能指数)**：满意度基准分数 ![][image3]。  
  * **七维雷达得分**：将后八个 Tab 的核心数据归一化为 0-100 的分值。  
* **可视化组件规范**：  
  * Radar Chart：展示七个维度的标准多边形综合评估雷达图。  
  * 3D Donut Gauge：3D 环形打分板，展示核心项目的总体进度。  
  * Statistic Cards：顶部并排核心数据卡片，附带日环比/周同比 (YoY/MoM) 趋势小火花线 (Sparkline)。

### **Tab 2: 业务完成情况 (Business Completion Status)**

* **定位**：研发与产品视角的项目交付进度与里程碑追踪。  
* **标准指标**：  
  * **需求交付率 (Delivery Rate)**：已上线需求数 / 计划需求数 ![][image4]。  
  * **燃尽率 (Burn-down Rate)**：Epic/Story Points 的消耗速度。  
* **可视化组件规范**：  
  * Sunburst Diagram：多层级旭日图，展示宏观 Epic 到微观 Task 的占比与进度。  
  * Interactive Gantt Chart：带依赖关系连线与关键路径 (Critical Path) 标识的动态甘特图。

### **Tab 3: 业务连接情况 (Business Connection Topology)**

* **定位**：展示系统中各个业务节点、微服务或 Agent 之间的静态依赖与物理/逻辑拓扑结构。  
* **标准指标**：  
  * **节点度数 (Node Degree)**：入度 (In-degree) 与出度 (Out-degree)，识别系统单点故障风险。  
  * **依赖层级深度 (Dependency Depth)**：最长调用链的深度。  
* **可视化组件规范**：  
  * Force-Directed Graph：力导向网络拓扑图。节点大小代表权重，连线代表依赖关系，支持不同图标区分组件类型。  
  * 3D Architecture Perspective：分层立体展示网关层、业务逻辑层、数据持久层。

### **Tab 4: 业务连通率 (Business Connectivity Rate)**

* **定位**：动态数据分析，评估节点间通信的成功率、丢包率及数据流转质量。  
* **标准指标**：  
  * **接口成功率**：![][image5]。  
  * **消息投递成功率**：跨服务/跨模块异步消息准确送达的比例。  
* **可视化组件规范**：  
  * Sankey Diagram：全链路桑基图。展示流量漏斗与数据连通率，红色分支代表连通失败或阻断的死信流量。  
  * Hexbin Map：蜂窝热力图。每个六边形代表一个接口或服务，颜色区分连通率状态。

### **Tab 5: 业务逻辑通顺性 (Business Logic Fluency)**

* **定位**：基于日志与埋点链路追踪，评估实际业务流转是否符合预期领域模型设计。  
* **标准指标**：  
  * **主流程转化率 (Conversion Rate)**：关键路径节点间的无缝跳转率。  
  * **异常回退率 (Fallback Rate)**：触发降级策略或非预期逻辑循环的频率。  
* **可视化组件规范**：  
  * Process Mining Map (DAG)：有向无环流程挖掘图。基于真实日志生成，线条粗细代表流量大小，红色高亮非预期跳转。  
  * Multi-dimensional Funnel：带转化率标签的多维标准漏斗图。

### **Tab 6: 架构健康度与扩展性 (Architecture Health)**

* **定位**：评估系统技术债、代码模块化质量与动态扩容能力。  
* **标准指标**：  
  * **圈复杂度 (Cyclomatic Complexity)**：核心模块代码的分支复杂度。  
  * **模块耦合度 (Coupling Score)**：基于依赖矩阵计算的内聚度与耦合度比值。  
* **可视化组件规范**：  
  * Dependency Matrix：架构依赖热力矩阵图，高亮显示非法的反向依赖或循环依赖。  
  * Polar Bar Chart：极坐标柱状图，展示各模块的资源弹性扩展评估得分。

### **Tab 7: 性能与稳定性 (Performance & Stability)**

* **定位**：系统运行时的硬件水位与软件性能监控。  
* **标准指标**：  
  * **响应耗时基线**：P95 与 P99 延迟 (Latency)。  
  * **吞吐量 (Throughput)**：并发 QPS/TPS 峰值与均值。  
* **可视化组件规范**：  
  * Time-Series Line Chart：带置信区间的时序折线图（阴影区域表示 AI 预测的正常阈值范围），Y 轴支持对数刻度。  
  * Flame Graph：下钻展示微服务 CPU 性能消耗时间片占比的火焰图。

### **Tab 8: 安全治理与合规 (Security & Governance)**

* **定位**：展示系统的防御能力、API 安全治理及细粒度权限审计状态。  
* **标准指标**：  
  * **鉴权拦截率**：细粒度权限校验命中拦截的次数 / 总异常请求数。  
  * **高危 API 暴露率**：未经过完整安全网关审计的开放 API 比例。  
* **可视化组件规范**：  
  * Threat Map：攻击来源气泡地图/威胁态势图。  
  * Nightingale Rose Chart：多层玫瑰图，按安全事件类型（如越权、Token失效、SQL注入等）进行标准化分类统计。

### **Tab 9: 资源利用与成本 (Resource & FinOps Analysis)**

* **定位**：面向 FinOps 的算力资源利用率与技术 ROI 分析。  
* **标准指标**：  
  * **资源利用率 (Utilization)**：CPU/内存的日均实际使用量 / 分配额度。  
  * **闲置成本 (Idle Cost)**：过度配置 (Over-provisioning) 造成的算力浪费估算。  
* **可视化组件规范**：  
  * Treemap：矩形树图。色块面积代表资源消耗大小，颜色梯度（红到绿）代表利用率，快速定位僵尸服务。  
  * Stacked Area Chart：堆叠面积图，展示长期趋势下的资源成本消耗对比。

## **4\. 标准化数据接口契约 (BFF Data Contract)**

为了保证前端渲染的绝对流畅与标准化，BFF 层必须向下屏蔽微服务差异，向前端输出高度统一的 JSON 结构。所有图表的数据源统一遵循以下 Schema 规范：

{  
  "code": 200,  
  "msg": "success",  
  "data": {  
    "metricId": "conn\_rate\_001",  
    "timestamp": 1718000000,  
    "dimensions": \["service\_name", "status"\],  
    "metrics": \["request\_count", "success\_rate"\],  
    "dataset": \[  
      {"service\_name": "auth-agent", "status": "200", "request\_count": 5432, "success\_rate": 0.99},  
      {"service\_name": "payment-api", "status": "500", "request\_count": 12, "success\_rate": 0.01}  
    \]  
  }  
}  


[image1]: <data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAC0AAAAXCAYAAACf+8ZRAAABsElEQVR4Xu2VPS8EURSGlygUGomNWLvzgYbONj5DJDqFhE4ifgA6jZKIWqHcVhQKGp2EUFKoRCsiRKEg2bAFnru5K+PY+bizKyGZJ3mzmXPec+e9dyY7qVRCQkKDLITQZNt2SRb9wNuPjlCey0bLsrocx9ni90B6Q2GRCfSBNmSvGviutb8s2feDcJPeOa1H6QuEReb14JLsRYG5Z5PQnOo4/kNUQOvpdLpFenxheFWHnZE9E0xDc0ij3HtN1gPhBtvoncFB2YtDjNAjkUOz8D56VS++7NWCaWi8Q2hHP+VdVGQTp9KndresTcOyVysxQufRjaipbGfe2hc0VrRhVvbiYhq6Gsw/hK6BYU6HX5Q9U+oU+kSt4bpuu+z9ANOYMvP6bMpeVExD68P65uf6QtV4t5u99UCy2WwPQ2+oIHthBIVWIWzxNHXoPVEr+a0RCifeyvCxrAeBv+h3Qx1QPck+T+1S/e15rjuUhw0uVGq/Bjd6QffoVusOPeVyue6KhyDT1M69cwpqV3pD5RNWn3bp+Z/wznayo6koqteXsmYymUwbYQaiiOC9cj4h4Q/xCRPolBFNvacIAAAAAElFTkSuQmCC>

[image2]: <data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADEAAAAXCAYAAACiaac3AAACaklEQVR4Xu2VT6hMcRTH59WTnSJZjGl+M2ZKslGzIORP7OzYvdjZCCtKNlaSlIXC23gpCwvJkx15KSvsyM5GSlj4G8ZDns+5zm+cOfOb5+bO6nW/dbq/8z3/f797f7dSKVGiRFGMeWIYGo3GtRDCHDJbr9ePebugVqu1sT1Uv7veHoHtLPIdeYv/Zm/PBYK3a6GT3paC+Lbb7SVWlyasD0NuFd7o66weAffR1mXdxfe09ZkXTL1PGzjkbcPQbDa3aMxM5Fi/Fw7basPN0cyBqCsnu/0g6tTf4Qfj9JZ5LgmSH9dGdntbDoxr7JlIsP4qXDydVqu1QnR5/g3L6t6xDepQAw0Lx4B7PZ8B43nkF8k2eFsR6FC9ZmjgRKo5+MtuCIn7YX0iT49PPDmNfCPJqj5DccipPJWNqZhLgTo3U0PAXUgM8cn6GL7bI0h4WMmNxq8wyDdB7kmeb9i1W9aGfj81BPw54XnvV4qufb3zfsoPxIvhqBr3eFtRkPO1Fs1Og/XVVBPhz+ss/Ljq0s8H5xZ5Od00ME6o00Fv+1+Q64jmfCX6sG8CbsryGjNrfQz/zPMDiFclBU9523wg5gry03Lyg9LCWYPom2Sd43bqxVhoX5OeHwr5q8puIJe8LYVYmIa2GW6/8i+dX98Vjv45mG+A9UXxsz5gTLhOp7PI8f8Gky8l+J7nPfCZlg/XcdKc/CcWR458t0P/iWXNEdswXNz1tVHH/CgkbqyRgyI3pDjyXJ9dO4Dxe4x8Qa5rszu9D6910Bwz2F8Ec5oLC3InM92uPDLqP/nIUK1Wl9Pc+jzCIGt8fIkSCwi/Act45DuvfZ6TAAAAAElFTkSuQmCC>

[image3]: <data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADcAAAAXCAYAAACvd9dwAAACgklEQVR4Xu2WPYgTYRCG4y9aKBaiZ0yyCYlEU16UE/wBQStBC0GxthFFsNFOsbrmsLC84xTOwuYqGxEOQQQLtTlBTrA4BbEQBEXBX059Jjsrk0m+JHsiyLEvDLvzzjuzM9/ufru5XIYMGZYMoii6hH3EPmOnfLwXyuXybXJ+Yc9xV/o4/Hc05yuVyuZCobC2VCodhHvldTkCh7TQGR9bLKg1h80Y/xn20Gq6gYY3SC8ct4vP+RbxG43GaqvTfr0dsJo2MOQeFY35WBrUarX1UsfzwknznrdIGnWcLMyC46TWFY6THE/YWE9wm7eR9A275WODgLxZ36Dy0vik5y206ReOu+rreT818vn8Roq852IPfKwXdIiOi4f4BLw/kWpmLc8TdVmHbj2qgl51UoGiayj2EpvDXeHjHqEhQryFDtF25/CnNPeY1WEz2Bvsjviyudi8gVGtVjdR4AOreNfHPEJDhHgL1fj366fyF6xO7nTi09fFfrU7QIF6FG+7Uz4WQmiIEG/B+75VNFxvSHya3ot/X7hisbjP6y20/j3Pd4Ch9ouY4qM+1g+hIUK8R7PZXIVuOoofucPYDc37870TjUlpoW99Vuykihb9zSP3U7eLaF35KKcCC/zI1uP8uvjcyV1Wp/V/WK4FCpzThCM+lhbUOh4ajlgz8XWjOus08z5XfLTXjH8zit/LZQlXr9fX6XATCddCOf4YDreRfwlt6LTxx7o1rQM3DPfF6qgx7vPActFZIop3c6/7N5D/PR3wMcen2NecWWkBsaPwTyyX/N1g77AFbN7GE/CE7VTdWz2+9pqlD91+ZVfqa6z2bp//X0N+s2h6ZBBjwB0+P0OGDAPjNxvU69bnbYyiAAAAAElFTkSuQmCC>

[image4]: <data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADwAAAAXCAYAAABXlyyHAAAC+klEQVR4Xu1WPWhUQRC+xJ+AqKjk/Lm/vT89OASF60QQBMVwnWAhaqN9kIiKkE4IIhbBwkJBEFP4U9kIYqPYKFoloCA2IUEF0crKS178xps5JuPuexEiJPA+GN6bb2Z299vdt/symRQpViScc+1ms7ne8hrlcvmQ5VYlIHahUqkcxXOqVCqN2TgBsVHYO8svK5I6QPwxDRb2PpfLbbBxAmJnYV9gc1ihazYObhJ2T3xuj+w2xB+BDQun65YN6OCN6tTbSavVWkexarVaYmoN5+/Sea47IZ+VPwH7YXKorq38n/KOVd/B3ESxWNwr/H8BOhkJCQb/CjZruBs2n/21lsOKHjN+79uE/0LeCYVCYZtL2Gl/4Lqz3W95DeQMWU6QIHgBdstwB3Q+RIz76rn2k/axq04of17eJa79WHDyohkWIPYr9N0RYgTL9h3VJLafY/44+dS+r55z9MQ8gf9BxadV7NE/b2VqnL45w3Wy2exGzVm4gGAMYj8PekTz9Xo9SzxW6zL5VpjAx8P/jrqXzPcRV6vVtsN/q/OWDGpIROO902g0NtkcCxJkB8b8YRY2rHn4W1nMHc77S1gcb2Fy+uFPwWboXfFhcEcdrMRmG/PBBQTjZN5DPFb6vObxeQxyH1fJDwkL8Rq0zTGBTfEpX3ZkUm0PSIzIIHjAxnwICQb6WPAVTWILFlnMKfLxnPPVJwnGN5tD/LX4eD+n8zERp3Fe7BPfCxKqZihK+oUjxAiWQXtPacd3MZ7PffWcE1leYGsg8JnmMNFbwF3SOYtAjdsDijh7kFm4eMEROp7UHA3CDGynr54FX7A8AfxT3Lu7NYd2Hup2+Ky4qHN6QGA+dPXQoDOBK4uA+HXqiE5fG0OnB60YFnLTcBFyHyi/besE+Xy+4MxPBwHckBF8xrul8S3UkrYuFVvOde/Pr7BZ2Aw/v8Hu6zxZUV4Bqrmr4wKKIeej6/6d9Q4fCy3KArFIbpa4vFUDTMhJZ/7DLRCfJsss9VpKkSJFipWI31SFEGbM37F+AAAAAElFTkSuQmCC>

[image5]: <data:image/png;base64,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>