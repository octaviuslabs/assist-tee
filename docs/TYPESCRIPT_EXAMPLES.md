# TypeScript Usage Examples

This guide shows how to write TypeScript code for the TEE runtime with practical examples.

## Table of Contents

- [Basic Handler](#basic-handler)
- [Using Multiple Modules](#using-multiple-modules)
- [Async Operations](#async-operations)
- [Error Handling](#error-handling)
- [Environment Variables](#environment-variables)
- [Working with Data](#working-with-data)
- [Complete Examples](#complete-examples)

## Basic Handler

The simplest handler receives an event and returns a result:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  return {
    message: "Hello from TEE!",
    timestamp: new Date().toISOString()
  };
}
```

### Testing

```bash
curl -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  return {\n    message: \"Hello from TEE!\",\n    timestamp: new Date().toISOString()\n  };\n}"
    },
    "ttlSeconds": 3600
  }'

# Get ENV_ID from response, then:
curl -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{"data": {}}'
```

## Using Multiple Modules

Split your code into multiple files for better organization:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { Calculator } = await import("./calculator.ts");
  const { Logger } = await import("./logger.ts");

  const logger = new Logger(context.executionId);
  const calc = new Calculator();

  logger.info("Starting calculation");
  const result = calc.add(event.data.a, event.data.b);
  logger.info(`Result: ${result}`);

  return {
    result,
    executionId: context.executionId
  };
}
```

```typescript
// calculator.ts
export class Calculator {
  add(a: number, b: number): number {
    return a + b;
  }

  multiply(a: number, b: number): number {
    return a * b;
  }

  divide(a: number, b: number): number {
    if (b === 0) throw new Error("Division by zero");
    return a / b;
  }
}
```

```typescript
// logger.ts
export class Logger {
  constructor(private executionId: string) {}

  info(message: string) {
    console.error(`[${this.executionId}] INFO: ${message}`);
  }

  error(message: string) {
    console.error(`[${this.executionId}] ERROR: ${message}`);
  }
}
```

### Setup Request

```bash
curl -X POST http://localhost:8080/environments/setup \
  -H "Content-Type: application/json" \
  -d '{
    "mainModule": "main.ts",
    "modules": {
      "main.ts": "export async function handler(event, context) {\n  const { Calculator } = await import(\"./calculator.ts\");\n  const calc = new Calculator();\n  return { result: calc.add(event.data.a, event.data.b) };\n}",
      "calculator.ts": "export class Calculator {\n  add(a: number, b: number): number { return a + b; }\n  multiply(a: number, b: number): number { return a * b; }\n}"
    },
    "ttlSeconds": 3600
  }'
```

## Async Operations

Handle asynchronous operations with async/await:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { delay, fetchData } = await import("./utils.ts");

  // Simulate async work
  await delay(100);

  // Process data asynchronously
  const results = await Promise.all([
    processItem(event.data.items[0]),
    processItem(event.data.items[1]),
    processItem(event.data.items[2])
  ]);

  return {
    results,
    processingTime: 100,
    executionId: context.executionId
  };
}

async function processItem(item: string): Promise<string> {
  // Simulate processing
  return `processed_${item}`;
}
```

```typescript
// utils.ts
export function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export async function fetchData(url: string): Promise<any> {
  // In a real scenario, you'd need --allow-net permission
  // For now, simulate fetching
  await delay(50);
  return { data: "simulated" };
}
```

## Error Handling

Proper error handling with try/catch:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { Validator } = await import("./validator.ts");
  const validator = new Validator();

  try {
    // Validate input
    validator.validateInput(event.data);

    // Process
    const result = processData(event.data);

    return {
      success: true,
      result,
      executionId: context.executionId
    };
  } catch (error) {
    // Return error in response (don't throw)
    return {
      success: false,
      error: error.message,
      executionId: context.executionId
    };
  }
}

function processData(data: any): any {
  if (!data.value) {
    throw new Error("Missing required field: value");
  }
  return { processed: data.value * 2 };
}
```

```typescript
// validator.ts
export class Validator {
  validateInput(data: any): void {
    if (!data) {
      throw new Error("Data is required");
    }

    if (typeof data !== "object") {
      throw new Error("Data must be an object");
    }

    if (!data.value || typeof data.value !== "number") {
      throw new Error("value must be a number");
    }

    if (data.value < 0) {
      throw new Error("value must be non-negative");
    }
  }
}
```

## Environment Variables

Access environment variables passed in the execution request:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  // Access environment variables
  const debugMode = Deno.env.get("DEBUG") === "true";
  const apiKey = Deno.env.get("API_KEY");
  const environment = Deno.env.get("ENVIRONMENT") || "production";

  if (debugMode) {
    console.error(`Debug mode enabled in ${environment}`);
    console.error(`Event data:`, JSON.stringify(event.data));
  }

  return {
    debugMode,
    environment,
    hasApiKey: !!apiKey,
    result: processWithConfig(event.data, debugMode)
  };
}

function processWithConfig(data: any, debug: boolean): any {
  if (debug) {
    console.error("Processing data:", data);
  }
  return { processed: true };
}
```

### Execution Request with Environment Variables

```bash
curl -X POST http://localhost:8080/environments/$ENV_ID/execute \
  -H "Content-Type: application/json" \
  -d '{
    "data": { "value": 42 },
    "env": {
      "DEBUG": "true",
      "ENVIRONMENT": "staging",
      "API_KEY": "secret-key-123"
    }
  }'
```

## Working with Data

Examples of common data processing patterns:

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { DataProcessor } = await import("./processor.ts");
  const processor = new DataProcessor();

  const { operation, data } = event.data;

  switch (operation) {
    case "filter":
      return processor.filterData(data);
    case "transform":
      return processor.transformData(data);
    case "aggregate":
      return processor.aggregateData(data);
    default:
      throw new Error(`Unknown operation: ${operation}`);
  }
}
```

```typescript
// processor.ts
export class DataProcessor {
  filterData(items: any[]): any {
    return {
      filtered: items.filter(item => item.active === true),
      totalProcessed: items.length
    };
  }

  transformData(items: any[]): any {
    return {
      transformed: items.map(item => ({
        ...item,
        processed: true,
        timestamp: new Date().toISOString()
      }))
    };
  }

  aggregateData(items: any[]): any {
    const sum = items.reduce((acc, item) => acc + (item.value || 0), 0);
    const avg = items.length > 0 ? sum / items.length : 0;

    return {
      count: items.length,
      sum,
      average: avg,
      min: Math.min(...items.map(i => i.value || 0)),
      max: Math.max(...items.map(i => i.value || 0))
    };
  }
}
```

## Complete Examples

### Example 1: Data Processing Pipeline

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { Pipeline } = await import("./pipeline.ts");
  const { Validators } = await import("./validators.ts");
  const { Transformers } = await import("./transformers.ts");

  // Create pipeline
  const pipeline = new Pipeline([
    Validators.notEmpty,
    Validators.hasRequiredFields(["id", "value"]),
    Transformers.addTimestamp,
    Transformers.normalize
  ]);

  // Process data
  const results = pipeline.process(event.data.items);

  return {
    success: true,
    processed: results.length,
    items: results,
    executionId: context.executionId
  };
}
```

```typescript
// pipeline.ts
type PipelineStep = (item: any) => any;

export class Pipeline {
  constructor(private steps: PipelineStep[]) {}

  process(items: any[]): any[] {
    return items.map(item => {
      let result = item;
      for (const step of this.steps) {
        result = step(result);
      }
      return result;
    });
  }
}
```

```typescript
// validators.ts
export const Validators = {
  notEmpty: (item: any) => {
    if (!item) throw new Error("Item is empty");
    return item;
  },

  hasRequiredFields: (fields: string[]) => (item: any) => {
    for (const field of fields) {
      if (!(field in item)) {
        throw new Error(`Missing required field: ${field}`);
      }
    }
    return item;
  }
};
```

```typescript
// transformers.ts
export const Transformers = {
  addTimestamp: (item: any) => ({
    ...item,
    processedAt: new Date().toISOString()
  }),

  normalize: (item: any) => ({
    ...item,
    value: typeof item.value === "string"
      ? parseFloat(item.value)
      : item.value
  })
};
```

### Example 2: Mathematical Operations

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { MathEngine } = await import("./math.ts");
  const engine = new MathEngine();

  const { operation, numbers } = event.data;

  return {
    operation,
    input: numbers,
    result: engine.calculate(operation, numbers),
    executionId: context.executionId
  };
}
```

```typescript
// math.ts
export class MathEngine {
  calculate(operation: string, numbers: number[]): number {
    switch (operation) {
      case "sum":
        return numbers.reduce((a, b) => a + b, 0);
      case "product":
        return numbers.reduce((a, b) => a * b, 1);
      case "average":
        return numbers.reduce((a, b) => a + b, 0) / numbers.length;
      case "min":
        return Math.min(...numbers);
      case "max":
        return Math.max(...numbers);
      default:
        throw new Error(`Unknown operation: ${operation}`);
    }
  }
}
```

### Example 3: State Machine

```typescript
// main.ts
export async function handler(event: any, context: any) {
  const { StateMachine } = await import("./statemachine.ts");
  const { actions } = await import("./actions.ts");

  const machine = new StateMachine(actions);
  const result = await machine.run(event.data.initialState, event.data.events);

  return {
    finalState: result.state,
    history: result.history,
    executionId: context.executionId
  };
}
```

```typescript
// statemachine.ts
export class StateMachine {
  constructor(private actions: Record<string, Function>) {}

  async run(initialState: any, events: any[]): Promise<any> {
    let state = initialState;
    const history = [state];

    for (const event of events) {
      const action = this.actions[event.type];
      if (!action) {
        throw new Error(`Unknown action: ${event.type}`);
      }

      state = await action(state, event.payload);
      history.push(state);
    }

    return { state, history };
  }
}
```

```typescript
// actions.ts
export const actions = {
  increment: (state: any, payload: any) => ({
    ...state,
    count: (state.count || 0) + (payload.amount || 1)
  }),

  decrement: (state: any, payload: any) => ({
    ...state,
    count: (state.count || 0) - (payload.amount || 1)
  }),

  reset: (state: any) => ({
    ...state,
    count: 0
  })
};
```

## Tips and Best Practices

1. **Always use async handlers** - Even if you don't have async operations now, it makes adding them later easier
2. **Return objects, don't throw** - Return `{ success: false, error: "..." }` instead of throwing errors
3. **Use console.error for logs** - stdout is reserved for the handler result
4. **Validate inputs** - Always validate `event.data` before processing
5. **Keep modules focused** - Each module should have a single responsibility
6. **Use TypeScript types** - Add type annotations for better code quality (optional but recommended)
7. **Handle edge cases** - Check for null/undefined, empty arrays, division by zero, etc.
8. **Use environment variables for config** - Don't hardcode configuration values
9. **Process in parallel when possible** - Use `Promise.all()` for independent operations
10. **Clean up resources** - Although containers are destroyed after execution, it's good practice

## Common Patterns

### Singleton Pattern

```typescript
// cache.ts
class Cache {
  private static instance: Cache;
  private data: Map<string, any>;

  private constructor() {
    this.data = new Map();
  }

  static getInstance(): Cache {
    if (!Cache.instance) {
      Cache.instance = new Cache();
    }
    return Cache.instance;
  }

  set(key: string, value: any): void {
    this.data.set(key, value);
  }

  get(key: string): any {
    return this.data.get(key);
  }
}

export default Cache;
```

### Factory Pattern

```typescript
// factory.ts
interface Processor {
  process(data: any): any;
}

class JSONProcessor implements Processor {
  process(data: any): any {
    return JSON.parse(data);
  }
}

class CSVProcessor implements Processor {
  process(data: any): any {
    return data.split(",");
  }
}

export class ProcessorFactory {
  static create(type: string): Processor {
    switch (type) {
      case "json":
        return new JSONProcessor();
      case "csv":
        return new CSVProcessor();
      default:
        throw new Error(`Unknown processor type: ${type}`);
    }
  }
}
```

### Builder Pattern

```typescript
// builder.ts
export class ResponseBuilder {
  private response: any = {};

  success(): this {
    this.response.success = true;
    return this;
  }

  error(message: string): this {
    this.response.success = false;
    this.response.error = message;
    return this;
  }

  data(data: any): this {
    this.response.data = data;
    return this;
  }

  metadata(key: string, value: any): this {
    if (!this.response.metadata) {
      this.response.metadata = {};
    }
    this.response.metadata[key] = value;
    return this;
  }

  build(): any {
    return this.response;
  }
}
```

## Next Steps

- Check out [design.md](design.md) for architecture details
- Read [GVISOR.md](GVISOR.md) for security configuration
- See [BUILD.md](BUILD.md) for build and deployment
- Run `./scripts/test-full-flow.sh` to test the system
