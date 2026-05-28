# Environment setup script template for fish shell.
# Copy this file to env.fish:
#   cp env.example.fish env.fish
#
# Then apply it to your current shell using:
#   source env.fish

# --- Consolidated Telemetry Envs (Used by both Vue frontends and Go backends) ---
set -gx VITE_SENTRY_DSN ""
set -gx VITE_POSTHOG_KEY ""
set -gx VITE_POSTHOG_HOST "https://us.i.posthog.com"

echo "✅ Environment variables loaded into current fish session."
