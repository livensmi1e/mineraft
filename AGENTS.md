# Engineering Principles

This file defines baseline engineering policy for Codex across working repositories.
Apply it only when a more specific instruction does not exist.

## 1. Smallest Complete Solution

- Prefer the smallest complete solution that satisfies the current requirement.
- Do not over-engineer or add speculative abstractions, extension points,
  configuration, fallbacks, or future-proofing.
- Implement only what the current task requires.
- Optimize for clarity, correctness, simplicity, and maintainability, not cleverness
  or minimum line count.
- Reuse existing code, utilities, patterns, and conventions before introducing new
  ones.
- Prefer deletion and simplification when they safely solve the problem.
- Accept some duplication when an abstraction would be harder to understand than
  the duplicated code.

## 2. Repository Conventions

- The working repository's existing conventions take precedence.
- Before changing code, inspect the relevant implementation, callers, tests,
  fixtures, configuration, exports, and documentation.
- Follow established architecture and naming unless the task explicitly requires
  changing them.
- Keep changes focused on the requested scope.
- Do not perform unrelated refactoring, renaming, cleanup, formatting, or
  modernization.
- Preserve public APIs and backward compatibility unless a breaking change is
  explicitly requested.

## 3. Google Engineering Guidance

- Use Google Engineering Practices as general guidance for code review, change
  scope, readability, testing, and maintainability.
- Use the relevant official Google language style guide when the working
  repository does not define a rule.
- Repository-specific conventions override external guidance unless they create
  correctness, security, compatibility, or maintainability problems.
- Do not mechanically apply external guidance or create broad formatting-only
  diffs.
- Do not copy complete external style guides into project instructions.

## 4. Go-Inspired Philosophy

Apply the general engineering philosophy of Go throughout design, planning,
implementation, testing, and review, even in other languages:

- Treat simplicity as a feature.
- Prefer explicit behavior over hidden magic.
- Prefer readable, unsurprising code over clever code.
- Prefer concrete solutions before abstractions.
- Prefer composition over unnecessary inheritance or frameworks.
- Keep APIs and interfaces small and purpose-driven.
- Introduce interfaces only when actual consumers, multiple implementations, or
  meaningful test boundaries justify them.
- Make errors visible and handle them deliberately.
- Avoid unnecessary layers, indirection, concurrency, and dependencies.
- Make default and zero-value behavior useful and safe where appropriate.
- Generalize only after a real repeated pattern emerges.

Do not blindly apply Go-specific syntax or idioms to other languages. Apply the
underlying philosophy while respecting the language and ecosystem in use.

## 5. Dependencies and Abstractions

- Prefer the standard library, platform capabilities, and dependencies already
  present in the project.
- Do not add a dependency for functionality that can be implemented clearly in a
  few lines.
- Every new dependency, abstraction, layer, interface, configuration option, or
  background process must solve a demonstrated requirement.
- Avoid wrappers that only rename or forward an API without meaningful behavior.
- Prefer local, reversible decisions while requirements remain uncertain.

## 6. Design and Planning

- Scale planning effort to the task's size, uncertainty, and risk.
- Do not turn a small local change into an architecture exercise.
- Start from concrete requirements and constraints, then identify the smallest
  viable approach.
- For consequential decisions, describe alternatives, tradeoffs, risks, and the
  reason for the selected approach.
- Fix root causes rather than masking symptoms.
- Ask for clarification only when ambiguity materially affects behavior,
  compatibility, data, security, or architecture.
- For low-risk ambiguity, choose the smallest reasonable interpretation and state
  the assumption.

## 7. Implementation Discipline

- Preserve validation at trust boundaries.
- Do not simplify away error handling, security controls, accessibility,
  observability, or data integrity.
- Keep error behavior explicit and consistent with the existing codebase.
- Use comments to explain non-obvious reasons, constraints, or tradeoffs, not to
  restate the code.
- Do not introduce behavior outside the requested scope.
- Avoid destructive operations unless explicitly requested and the exact target
  has been verified.
- Never expose secrets, credentials, tokens, or sensitive information.

## 8. Tests as Verification

- Add or update tests for every meaningful behavior change.
- Treat tests as evidence that the implementation works, not as ceremony.
- Test observable behavior, important boundaries, regressions, and meaningful
  failure cases.
- Prefer the smallest set of high-value tests that provides sufficient confidence.
- Avoid duplicative, brittle, trivial, or implementation-coupled tests.
- Bug fixes should normally include a regression test that fails before the fix
  and passes after it.
- Do not weaken or remove valid tests merely to make an implementation pass.
- Trivial changes that cannot meaningfully fail do not require artificial tests.

## 9. Command Execution Policy

- Do not run tests, lint, formatters, builds, benchmarks, code generation, or other
  verification commands unless the user explicitly requests execution.
- Avoid continuous checks of diffs, staging, or working-tree changes.
- Add or update test code when appropriate even when tests must not be run.
- Never claim that a command passed unless it was actually executed.
- If commands were not run, explicitly state that the work remains unverified by
  execution.
- Limit formatting to changed code, and perform it only when explicitly requested.

## 10. Tool Selection

- Use repository-provided tools and conventions before introducing alternatives.
- Use `rg` for filenames and literal or regular-expression text searches.
- Use `ast-grep` when a search or rewrite depends on syntax structure; do not use
  it for simple text matching. Read `tools/ast-grep/TOOL.md` when available.
- Use RTK to reduce large, noisy command output when compression will not hide
  required detail. Bypass it when exact output matters. Read `tools/rtk/TOOL.md`
  when available.
- Treat these tools as optional. Use a standard alternative when unavailable, and
  do not install or configure them unless the user requests it.

## 11. Communication

At the end of a task, report concisely:

- what changed;
- which tests were added or updated;
- which commands were run and which were not run; and
- assumptions, limitations, and remaining risks.

Lead with the outcome. Keep explanations understandable in one read. Clearly
distinguish implemented work from inspected, inferred, or unverified work.

## 12. Instruction Precedence

Apply instructions in this order:

1. Explicit user instructions have highest priority.
2. The working project's nearest `AGENTS.md` takes precedence over this baseline.
3. Existing repository conventions take precedence over generic external guidance.
4. This file supplies defaults only when no more specific instruction exists.
