/**
 * Example handler for the TEE
 *
 * This is what users write - a simple async function that takes
 * event and context parameters and returns a result.
 */

export async function handler(event: any, context: any) {
  console.log("Handler invoked!");
  console.log("Execution ID:", context.executionId);
  console.log("Environment ID:", context.environmentId);

  // Access input data
  const { a, b } = event.data || { a: 0, b: 0 };

  // Access environment variables
  const debug = event.env?.DEBUG || Deno.env.get("DEBUG");

  if (debug) {
    console.log(`Computing sum of ${a} + ${b}`);
  }

  // Can import other modules
  const { add, multiply } = await import("./utils.ts");

  const sum = add(a, b);
  const product = multiply(a, b);

  // Return result (will be JSON serialized)
  return {
    sum,
    product,
    timestamp: new Date().toISOString(),
    executionId: context.executionId,
  };
}
