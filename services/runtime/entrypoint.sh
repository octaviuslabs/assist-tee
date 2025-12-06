#!/bin/sh

if [ -z "$ALLOWED_ENV_VARS" ]; then
  exec deno run --allow-read=/workspace,/runtime --allow-env /runtime/runner.ts
else
  exec deno run --allow-read=/workspace,/runtime --allow-env="$ALLOWED_ENV_VARS" /runtime/runner.ts
fi
