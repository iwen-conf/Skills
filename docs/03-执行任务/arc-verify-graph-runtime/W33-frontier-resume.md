# W33 Frontier 续探

状态：`[x]`

`discover status|resume|abort` 读写 `exploration_state`。截断时保持夹具锁，从检查点继续，不重放已执行动作。源码或模型摘要变化、以及 `failed_lock_retained` 会拒绝续探。
