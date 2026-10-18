# Commit message conventions

Derived from all 30 commits reachable through local Git refs before Week6 Task0.
This describes observed practice, not a validator or a requirement to copy long
messages. The repository root README establishes that tasks are commits, while
weeks are directories.

## Subject

Task commits use `WeekN TaskM: <imperative description>`, for example:

```text
Week5 Task1: integrate local Ollama generation into advent-agent
Week4 Task0: index Kotlin documentation with local embeddings
Week3 Task0: add MCP discovery and execution
```

Use the week and task being implemented, then a concise English imperative
such as add, integrate, optimize, or deploy. Subjects do not end with a period.
Do not switch to Conventional Commits prefixes such as `feat:`: they are not
the format observed here.

These examples come from commits `28037d7`, `ad81d2b`, and `ffd0369`,
respectively. The history reviewed spans `50cccb4` through `336c736`.

## Optional body

Many commits put a blank line after the subject, followed by prose paragraphs.
Later messages generally describe the implementation and resulting behavior,
then validation or measured results and limitations. Mention tests/builds that
actually passed and distinguish observed results from claims not verified.
There are no mandatory section labels, fixed paragraph count, or line-length
rule established by this history. Earlier commits sometimes have short bodies
or no body at all, notably `Week1 Task2: add persistent token accounting`.

## Exceptions and task numbering

`Initial commit` and `add README.md` are setup commits without week/task prefixes.
Two separate commits use `Week4 Task3`: one adds reranking/filtering and another
requires quotes and refusal with weak context. Do not infer one commit per task,
consecutive task numbers, or a prohibition on follow-up commits.

This document records the convention. The assistant can explain or suggest a
message, but Task0 has no ability to create commits or modify Git state.
