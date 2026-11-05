# UI Review Checklist

Use relevant sections for substantial implementation, refactoring, or review.
Findings must relate to usability, clarity, safety, accessibility, consistency,
or maintenance; the checklist must not invent product scope.

## Task and profile

- [ ] Can the intended user identify the surface's main purpose?
- [ ] Does essential information precede secondary explanation?
- [ ] Does the selected profile fit the task, content, interaction complexity,
      roles, lifetime, and existing stack?
- [ ] If the repository contains different profiles, is this surface internally
      consistent?

## Components and hierarchy

- [ ] Does the change continue the existing component system?
- [ ] Is a new system or dependency demonstrably necessary?
- [ ] Are full component systems kept separate within the surface?
- [ ] Are actions emphasized according to their actual workflow importance?
- [ ] Can any card, panel, wrapper, badge, icon, or decorative effect be removed?
- [ ] Does each extracted component own repetition, complex behavior,
      accessibility behavior, or a domain boundary?

## Responsive and accessible behavior

- [ ] Does the main task remain usable at narrow widths and high zoom?
- [ ] Are long, translated, missing, and empty content handled?
- [ ] Is table, code, and log overflow intentional?
- [ ] Are semantics, names, descriptions, keyboard behavior, focus, and contrast
      complete?
- [ ] Do composite controls manage focus, dismissal, and keyboard interaction?
- [ ] Is meaning available without color, and is reduced motion respected?

## States and copy

- [ ] Are applicable loading, empty, partial, error, success, warning, disabled,
      pending, and unauthorized states covered?
- [ ] Do state changes avoid duplicate actions and lost input?
- [ ] Are labels specific and operational?
- [ ] Do errors provide a recovery path?
- [ ] Are irreversible consequences explained at the decision point?

## Scope and handoff

- [ ] Does the change preserve business logic and avoid unrelated redesign?
- [ ] Are dependencies and abstractions proportionate to demonstrated needs?
- [ ] Were meaningful interaction tests added or updated where appropriate?
- [ ] Does the report identify the selected profile and component strategy?
- [ ] Does it distinguish inspected behavior from visual, responsive, or
      interaction behavior that remains unverified?
