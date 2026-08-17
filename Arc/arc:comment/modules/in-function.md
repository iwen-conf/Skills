# In-Function Step Comments

Load only when editing numbered `//1.` style comments inside a function body.

Inside function bodies, describe meaningful execution steps with numbered `//` comments. Use imperative, outcome-oriented wording.

```go
func Example(ctx context.Context, id string) error {
    //1. 调用“xxx.函数名”获取xxx
    item, err := xxx.GetItem(ctx, id)
    if err != nil {
        return err
    }

    //2. 校验xxx是否满足业务条件
    if !item.Enabled {
        return ErrDisabled
    }

    //3. 调用“yyy.函数名”保存xxx
    return yyy.SaveItem(ctx, item)
}
```

Rules:

- Start each step with `//1.`, `//2.`, `//3.` and keep numbering continuous in the local function.
- Use `调用“包名或对象名.函数名”获取/创建/更新/删除xxx` when the line calls another component.
- Comment a block of code, not every line. Merge adjacent trivial statements under one step.
- Renumber comments after inserting, deleting, or reordering steps.
- Avoid stale comments: the named callee, action, and object in the comment must match the code below it.
