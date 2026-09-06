# Repository guidance

## Project context

- This repository contains homework and learning results for AI Advent Challenge #9. Each top-level `weekN/` directory contains that week's work, while individual days and tasks are represented by commits rather than separate task directories or branches.
- When working anywhere inside a top-level directory whose name matches `weekN` (for example, `week0/`), first read both the repository-root `README.md` and that week's `README.md`. Use them to establish the project purpose, build and usage commands, and current documented behavior before inspecting only the implementation files relevant to the task.
- Treat source code and tests as authoritative when they disagree with documentation, and update stale documentation as part of the task when it is in scope.

## Feature workflow

- After adding or changing a user-facing feature, rebuild the project successfully before presenting or executing commands that demonstrate the feature. Follow the build instructions for the current week and ensure every demonstrated command invokes the newly rebuilt artifact rather than a stale binary.
- Run the relevant tests before the demonstration. If the build or tests fail, report that clearly and do not present the feature as ready to run.
- When more than one executable or build variant exists, state the exact artifact path used in the demonstration so the command is reproducible.
