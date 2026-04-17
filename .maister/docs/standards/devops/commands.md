# DevOps: Command Conventions

The project uses RTK (Rust Token Killer) as a token-optimized CLI proxy. All shell commands go through it.

### RTK Prefix Mandatory for All Shell Commands
Every shell command must be prefixed with `rtk` — no exceptions. Applies to `go`, `git`, `make`, `helm`, `docker`, `bun`, `cat`, `grep`, `awk`, `sed`, `find`, `ls`, `wc`, `sort`, `uniq`, `cut`, `chmod`, `mkdir`, `rm`, and any other shell tool. Safe passthrough when no filter exists. Each command in a pipeline gets its own `rtk` prefix. Examples: `rtk go test ./...`, `rtk git commit -m "msg"`, `rtk cat file.go | rtk grep func`. Sources: CLAUDE.md, AGENTS.md.
