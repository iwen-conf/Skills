## Common Questions

### Q: How to handle JSON.parse return value?

```typescript
// Error
let data = JSON.parse(str);

// Correct
let data: Record<string, Object> = JSON.parse(str);
```

### Q: How to define object types?

```typescript
// TypeScript syntax (not supported in ArkTS)
type Person = { name: string, age: number }

// ArkTS syntax
interface Person {
  name: string;
  age: number;
}

// Using object literal
let p: Person = { name: 'John', age: 25 };
```

### Q: How to replace globalThis?

```typescript
// Error
globalThis.value = 'xxx';

// Use singleton pattern
export class GlobalContext {
  private constructor() {}
  private static instance: GlobalContext;
  private _objects = new Map<string, Object>();

  public static getContext(): GlobalContext {
    if (!GlobalContext.instance) {
      GlobalContext.instance = new GlobalContext();
    }
    return GlobalContext.instance;
  }

  getObject(key: string): Object | undefined {
    return this._objects.get(key);
  }

  setObject(key: string, value: Object): void {
    this._objects.set(key, value);
  }
}
```

### Q: How to handle error types in catch?

```typescript
// Error
try {} catch (e: BusinessError) {}

// Correct
try {} catch (error) {
  let e: BusinessError = error as BusinessError;
}
```

### Q: How to use Record type?

```typescript
// TypeScript index signature
function foo(data: { [key: string]: string }) {}

// ArkTS Record
function foo(data: Record<string, string>) {}

// Usage example
let map: Record<string, number> = {
  'John': 25,
  'Mary': 21,
};
```

### Q: How to replace constructor signatures with factory functions?

```typescript
// TypeScript constructor signature
type ControllerCtor = {
  new (value: string): Controller;
}

// ArkTS factory function
type ControllerFactory = () => Controller;

class Menu {
  createController: ControllerFactory = () => {
    return new Controller('default');
  }
}
```

