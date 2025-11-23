#!/usr/bin/env -S deno run --allow-read=/workspace,/runtime --allow-env

/**
 * TEE Runtime Wrapper
 *
 * This script runs inside the execution container and:
 * 1. Reads execution parameters from stdin (JSON)
 * 2. Loads the user's module from /workspace
 * 3. Calls the user's exported `handler(event, context)` function
 * 4. Writes the result to stdout as JSON
 */

interface ExecutionEvent {
  env?: Record<string, string>;
  data?: any;
}

interface ExecutionContext {
  executionId: string;
  environmentId: string;
  requestId: string;
}

interface ExecutionInput {
  event: ExecutionEvent;
  context: ExecutionContext;
  mainModule: string;
}

interface ExecutionOutput {
  success: boolean;
  result?: any;
  error?: string;
  stack?: string;
}

async function readStdin(): Promise<string> {
  const chunks: Uint8Array[] = [];
  for await (const chunk of Deno.stdin.readable) {
    chunks.push(chunk);
  }

  const totalLength = chunks.reduce((acc, chunk) => acc + chunk.length, 0);
  const combined = new Uint8Array(totalLength);

  let offset = 0;
  for (const chunk of chunks) {
    combined.set(chunk, offset);
    offset += chunk.length;
  }

  const decoder = new TextDecoder();
  return decoder.decode(combined);
}

async function main() {
  try {
    // 1. Read stdin as JSON
    const stdinContent = await readStdin();

    if (!stdinContent.trim()) {
      throw new Error("No input provided via stdin");
    }

    const input: ExecutionInput = JSON.parse(stdinContent);

    // 2. Set environment variables
    if (input.event.env) {
      for (const [key, value] of Object.entries(input.event.env)) {
        Deno.env.set(key, String(value));
      }
    }

    // 3. Load user module
    const modulePath = `/workspace/${input.mainModule}`;
    const module = await import(modulePath);

    if (typeof module.handler !== "function") {
      throw new Error(
        `Module '${input.mainModule}' does not export a 'handler' function.\n` +
        `Expected: export async function handler(event, context) { ... }`
      );
    }

    // 4. Call user's handler
    const result = await module.handler(input.event, input.context);

    // 5. Write success result to stdout
    const output: ExecutionOutput = {
      success: true,
      result: result,
    };

    console.log(JSON.stringify(output));
    Deno.exit(0);

  } catch (error) {
    // Write error to stdout as structured JSON
    const output: ExecutionOutput = {
      success: false,
      error: error.message,
      stack: error.stack,
    };

    console.log(JSON.stringify(output));
    Deno.exit(1);
  }
}

main();
