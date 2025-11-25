#!/usr/bin/env -S deno run --allow-read=/workspace,/runtime --allow-env


// I'm actually thinking that here we should use SSE. Basically, as the user makes the handler request, we should open up a socket and stream back logs and everything else to the user. I think we're going to want to stream back with some kind of header on each line, maybe there's a log header and then there's a final result header or something like that for the final result. 

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
  data?: unknown;
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
  result?: unknown;
  error?: string;
  stack?: string;
  logs?: LogEntry[];
  timing?: TimingInfo;
}

interface LogEntry {
  level: "debug" | "info" | "warn" | "error";
  message: string;
  timestamp: string;
}

interface TimingInfo {
  stdinReadMs: number;
  moduleLoadMs: number;
  handlerExecutionMs: number;
  totalMs: number;
}

// Captured logs from user code
const capturedLogs: LogEntry[] = [];

// Timing information
const timings: Record<string, number> = {};
const startTime = performance.now();

// Check if debug mode is enabled
const DEBUG = Deno.env.get("TEE_DEBUG") === "true" || Deno.env.get("TEE_DEBUG") === "1";

/**
 * Log a debug message (only in debug mode, written to stderr)
 */
function debugLog(message: string, data?: Record<string, unknown>): void {
  if (!DEBUG) return;

  const logEntry = {
    level: "debug",
    component: "runtime",
    message,
    timestamp: new Date().toISOString(),
    ...data,
  };
  console.error(JSON.stringify(logEntry));
}

/**
 * Capture console methods to collect user logs
 */
function setupConsoleCapture(): void {
  const originalConsole = {
    log: console.log,
    info: console.info,
    warn: console.warn,
    error: console.error,
    debug: console.debug,
  };

  const captureLog = (level: LogEntry["level"]) => (...args: unknown[]) => {
    const message = args
      .map((arg) => (typeof arg === "string" ? arg : JSON.stringify(arg)))
      .join(" ");

    capturedLogs.push({
      level,
      message,
      timestamp: new Date().toISOString(),
    });

    // In debug mode, also write to stderr so logs are visible
    if (DEBUG) {
      originalConsole.error(JSON.stringify({
        level,
        component: "user",
        message,
        timestamp: new Date().toISOString(),
      }));
    }
  };

  console.log = captureLog("info");
  console.info = captureLog("info");
  console.warn = captureLog("warn");
  console.error = captureLog("error");
  console.debug = captureLog("debug");
}

/**
 * Record timing for a phase
 */
function recordTiming(phase: string, startMs: number): void {
  timings[phase] = performance.now() - startMs;
}

async function readStdin(): Promise<string> {
  const phaseStart = performance.now();
  debugLog("reading stdin");

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
  const content = decoder.decode(combined);

  recordTiming("stdinReadMs", phaseStart);
  debugLog("stdin read complete", { bytes: totalLength });

  return content;
}

async function main() {
  debugLog("runtime starting", {
    denoVersion: Deno.version.deno,
    v8Version: Deno.version.v8,
    typescriptVersion: Deno.version.typescript,
  });

  // Setup console capture before any user code runs
  setupConsoleCapture();

  try {
    // 1. Read stdin as JSON
    const stdinContent = await readStdin();

    if (!stdinContent.trim()) {
      throw new Error("No input provided via stdin");
    }

    const input: ExecutionInput = JSON.parse(stdinContent);

    debugLog("input parsed", {
      executionId: input.context.executionId,
      environmentId: input.context.environmentId,
      mainModule: input.mainModule,
      hasEnvVars: Boolean(input.event.env),
      hasData: Boolean(input.event.data),
    });

    // 2. Set environment variables
    if (input.event.env) {
      const envCount = Object.keys(input.event.env).length;
      debugLog("setting environment variables", { count: envCount });

      for (const [key, value] of Object.entries(input.event.env)) {
        Deno.env.set(key, String(value));
      }
    }

    // 3. Load user module
    const moduleLoadStart = performance.now();
    const modulePath = `/workspace/${input.mainModule}`;

    debugLog("loading module", { path: modulePath });

    const module = await import(modulePath);

    recordTiming("moduleLoadMs", moduleLoadStart);
    debugLog("module loaded", {
      exports: Object.keys(module),
      hasHandler: typeof module.handler === "function",
    });

    if (typeof module.handler !== "function") {
      throw new Error(
        `Module '${input.mainModule}' does not export a 'handler' function.\n` +
        `Expected: export async function handler(event, context) { ... }`
      );
    }

    // 4. Call user's handler
    const handlerStart = performance.now();
    debugLog("calling handler", {
      executionId: input.context.executionId,
    });

    const result = await module.handler(input.event, input.context);

    recordTiming("handlerExecutionMs", handlerStart);
    debugLog("handler completed", {
      resultType: typeof result,
      hasResult: result !== undefined && result !== null,
    });

    // 5. Build timing info
    const timing: TimingInfo = {
      stdinReadMs: Math.round(timings.stdinReadMs || 0),
      moduleLoadMs: Math.round(timings.moduleLoadMs || 0),
      handlerExecutionMs: Math.round(timings.handlerExecutionMs || 0),
      totalMs: Math.round(performance.now() - startTime),
    };

    // 6. Write success result to stdout
    const output: ExecutionOutput = {
      success: true,
      result: result,
      logs: capturedLogs.length > 0 ? capturedLogs : undefined,
      timing: DEBUG ? timing : undefined,
    };

    // Use the original stdout for the final output
    const encoder = new TextEncoder();
    await Deno.stdout.write(encoder.encode(JSON.stringify(output)));

    debugLog("execution completed successfully", { timing });
    Deno.exit(0);

  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    const errorStack = error instanceof Error ? error.stack : undefined;

    debugLog("execution failed", {
      error: errorMessage,
      stack: errorStack,
    });

    // Build timing info even for errors
    const timing: TimingInfo = {
      stdinReadMs: Math.round(timings.stdinReadMs || 0),
      moduleLoadMs: Math.round(timings.moduleLoadMs || 0),
      handlerExecutionMs: Math.round(timings.handlerExecutionMs || 0),
      totalMs: Math.round(performance.now() - startTime),
    };

    // Write error to stdout as structured JSON
    const output: ExecutionOutput = {
      success: false,
      error: errorMessage,
      stack: errorStack,
      logs: capturedLogs.length > 0 ? capturedLogs : undefined,
      timing: DEBUG ? timing : undefined,
    };

    const encoder = new TextEncoder();
    await Deno.stdout.write(encoder.encode(JSON.stringify(output)));
    Deno.exit(1);
  }
}

main();
