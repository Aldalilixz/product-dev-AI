# Project Rules for Agents

## Prompt Logging Rule

- On every user message, append the exact user prompt to `prompts.md`.
- Log only the user's prompt text.
- Do not log assistant responses, tool output, analysis, metadata, or system/developer messages.
- Preserve prompt meaning and wording; do not paraphrase unless the user explicitly asks.
- Keep entries append-only in chronological order.

## Engineering Practice Rule

When implementing, reviewing, or refactoring code, apply all of these practices:

- TDD: write or update tests first when practical, then implement to pass tests.
- SOLID: keep responsibilities clear and dependencies explicit.
- Code smells and refactor: detect smells early and refactor for readability and maintainability.
- DRY: remove duplication in logic, data transforms, and abstractions.
- YAGNI: avoid speculative abstractions and features not required by the current prompt.

## Senior Mindset Rule

Write code as if the next developer is senior too and has no patience for ambiguity.

Expected behavior:

- Prefer clear naming over cleverness.
- Make intent obvious in code structure and boundaries.
- Add concise comments only where non-obvious reasoning matters.
- Surface trade-offs, risks, and assumptions explicitly.
- Ship code that is easy to review, test, and change safely.
