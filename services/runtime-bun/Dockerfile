FROM oven/bun:1.1-alpine

USER bun
WORKDIR /workspace

# Copy runtime wrapper
COPY --chown=bun:bun runner.ts /runtime/runner.ts

# Expose for health checks (not used currently)
EXPOSE 3000

# Run the wrapper - it reads from stdin
ENTRYPOINT ["bun", "run", "/runtime/runner.ts"]
