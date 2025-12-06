#!/usr/bin/env bun

/**
 * TEE Runtime Wrapper (Bun)
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
const DEBUG = process.env.TEE_DEBUG === "true" || process.env.TEE_DEBUG === "1";

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
    log: console.log.bind(console),
    info: console.info.bind(console),
    warn: console.warn.bind(console),
    error: console.error.bind(console),
    debug: console.debug.bind(console),
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

  // Bun's approach to reading stdin
  const chunks: Buffer[] = [];
  const reader = Bun.stdin.stream().getReader();

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(Buffer.from(value));
  }

  const content = Buffer.concat(chunks).toString("utf-8");

  recordTiming("stdinReadMs", phaseStart);
  debugLog("stdin read complete", { bytes: content.length });

  return content;
}

async function main() {
  debugLog("runtime starting", {
    bunVersion: Bun.version,
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
        process.env[key] = String(value);
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

    // Write to stdout
    process.stdout.write(JSON.stringify(output));

    debugLog("execution completed successfully", { timing });
    process.exit(0);

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

    process.stdout.write(JSON.stringify(output));
    process.exit(1);
  }
}

main();
