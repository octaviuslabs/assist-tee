FROM denoland/deno:alpine-1.40.0

USER deno
WORKDIR /workspace

# Copy runtime wrapper
COPY --chown=deno:deno runner.ts /runtime/runner.ts

# Expose for health checks (not used currently)
EXPOSE 3000

# Run the wrapper - it reads from stdin
ENTRYPOINT ["deno", "run", \
            "--allow-read=/workspace,/runtime", \
            "--allow-env", \
            "/runtime/runner.ts"]
