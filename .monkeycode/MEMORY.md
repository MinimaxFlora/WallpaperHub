# User Instruction Memory

This file records user instructions, preferences, and teachings for reference in future interactions.

## Format

### User Instruction Entry
User instruction entries should follow this format:

[User Instruction Summary]
- Date: [YYYY-MM-DD]
- Context: [Mentioned scenario or time]
- Instructions:
  - [Content of user teaching or instruction, described line by line]

### Project Knowledge Entry
Entries discovered by the Agent during task execution should follow this format:

[Project Knowledge Summary]
- Date: [YYYY-MM-DD]
- Context: Discovered by Agent while performing [specific task description]
- Category: [Operations & Deployment|Build Methods|Testing Methods|Troubleshooting & Debugging|Workflow & Collaboration|Environment Configuration]
- Instructions:
  - [Specific knowledge points, described line by line]

## Deduplication Strategy
- Before adding a new entry, check for similar or identical instructions.
- If a duplicate is found, skip the new entry or merge it with the existing one.
- When merging, update the context or date information.
- This helps avoid redundant entries and keeps the memory file tidy.

## Entries

[Project Knowledge Summary]
- Date: 2026-09-30
- Context: Discovered by Agent while verifying the wallpaper-api service
- Category: Environment Configuration
- Instructions:
  - The development environment has no `docker` / `docker compose` CLI, so Dockerfile and docker-compose cannot be verified locally. Verify behavior by building the binary (`go build`) and running it against a temporary images directory, then exercising endpoints with `curl`.
  - Go toolchain available: go1.25.6; module proxy is `https://proxy.golang.org,direct`.

[Project Knowledge Summary]
- Date: 2026-09-30
- Context: Discovered by Agent while upgrading wallpaper-api to go1.27.1
- Category: Build Methods
- Instructions:
  - The project requires the go1.27.1 toolchain (`go` directive in go.mod), but the local toolchain is older; run Go commands with `GOTOOLCHAIN=auto` so the required toolchain is downloaded on demand.
  - The Docker image pins `golang:1.27.1-alpine`, so the toolchain is not downloaded during container builds.
  - Offline HTTPS/ACME verification: generate a self-signed ECDSA cert for a host with at least two dot-separated components (autocert rejects single-label names such as `localhost`), concatenate PRIVATE KEY + CERTIFICATE into `<cache_dir>/<domain>`, then start the service with `WALLPAPER_DOMAIN` set and test with `curl -k --resolve`.
