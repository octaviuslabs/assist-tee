FROM denoland/deno:alpine-1.40.0

USER deno
WORKDIR /workspace

# Copy runtime wrapper and entrypoint
COPY --chown=deno:deno runner.ts /runtime/runner.ts
COPY --chown=deno:deno entrypoint.sh /runtime/entrypoint.sh

# Expose for health checks (not used currently)
EXPOSE 3000

# Run via entrypoint script for dynamic permission handling
ENTRYPOINT ["/runtime/entrypoint.sh"]
