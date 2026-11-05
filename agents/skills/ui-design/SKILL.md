---
name: ui-design
description: >
  Design, implement, refactor, or review UI when a task materially affects
  information hierarchy, application layout, responsive or accessible behavior,
  visual clutter, or selection of Carbon, native UI, or an existing component
  system. Use for enterprise and business applications as well as personal,
  PoC, internal, documentation, and content-first interfaces.
disable-model-invocation: true
---

# UI Design

Treat UI as a tool for completing work. Start from the user's task, content,
decisions, and actions, not from a component catalog or a preferred visual style.

Respect the working repository's `AGENTS.md`, architecture, components, tokens,
and conventions.

## Workflow

1. Understand the user and task.
2. Inspect the existing application and conventions.
3. Select the UI profile.
4. Load only the selected profile reference.
5. Identify content, actions, and important states.
6. Establish hierarchy and interaction flow.
7. Choose the component strategy.
8. Implement the smallest complete UI.
9. Review accessibility and responsive behavior.
10. Perform a deletion pass.
11. Load the review checklist for substantial work.
12. Report the selected profile, important decisions, and unverified behavior.

Ask during the deletion pass:

> What can be removed without reducing usability, clarity, safety, or
> accessibility?

## Select the profile

Classify the surface before designing. Do not decide from screen count or file
count alone. Consider:

- the primary user task and content volume;
- interaction complexity and recurring forms, tables, dialogs, and navigation;
- user roles, permissions, and business states;
- expected product lifetime and cross-screen consistency needs; and
- the existing stack, design system, and component conventions.

Select a profile for the surface being changed, not automatically for the whole
repository. A SaaS product may use Carbon in its authenticated business
application and engineering content-first design in its technical documentation.
Do not mix two full component systems within one surface.

### Existing applications

Existing conventions take priority:

- Continue using the established design or component system.
- Do not migrate to Carbon because the product belongs to a business domain.
- Migrate design systems only when the user explicitly requests it.
- Do not add Carbon beside another full component system.
- If the project uses shadcn/ui, keep it and apply the common principles here;
  do not add Carbon automatically.

### Greenfield applications

Select **Enterprise Carbon** for a compatible medium-sized or larger business
product, multi-user SaaS, recurring operational workflows, data-heavy
administration, many forms, tables, filters, dialogs or permissions, strong
cross-screen consistency needs, or a product expected to evolve for years.

Select **Engineering content-first** for a personal application, proof of
concept, quick prototype, single-purpose tool, small internal utility,
documentation or knowledge interface, read-oriented or content-heavy surface,
or a workflow where native controls cover most interaction needs.

For a compatible greenfield Enterprise Carbon surface, prefer Carbon by default.
For low-risk ambiguity, choose the smaller approach and state the assumption.
Ask only when the choice materially affects dependencies, architecture, or
migration cost.

## Choose components

- Existing project components and tokens come first.
- For Enterprise Carbon, use official Carbon packages when selected and
  compatible. Load its reference before adding a dependency.
- For Engineering content-first, prefer semantic HTML, native capabilities,
  focused CSS, and dependencies already present.
- Do not introduce a component system merely to make the UI look professional.
- Extract a component for meaningful repetition, complex interaction behavior,
  accessibility behavior, or a clear domain boundary. Do not extract simple
  markup merely to wrap it.

## Common implementation rules

- Build the smallest complete interface.
- Establish information hierarchy before visual styling.
- Make the primary action clear. Several actions may share emphasis when the
  workflow genuinely gives them equal importance.
- Avoid wall-of-UI design: no card for every section, unnecessary nested panels,
  uniformly prominent widgets, decorative dashboards, or excessive badges,
  pills, icons, shadows, and gradients.
- Prefer typography, spacing, alignment, simple dividers, lists, and tables.
- Treat accessibility, responsive behavior, and relevant UI states as
  implementation requirements.
- Preserve business logic and keep UI state local when practical.
- Keep changes focused and avoid unrelated redesign.
- Follow the applicable `AGENTS.md` verification policy.
- Never claim visual, responsive, or interaction behavior that was not inspected.

## Load references

- Read [references/enterprise-carbon-design.md](references/enterprise-carbon-design.md)
  only for a selected greenfield Enterprise Carbon surface, an existing Carbon
  repository, an explicit Carbon request, or a Carbon-inspired enterprise
  direction.
- Read [references/engineering-content-design.md](references/engineering-content-design.md)
  only for a selected personal, PoC, small-tool, documentation, knowledge, or
  content-first surface.
- Read [references/review-checklist.md](references/review-checklist.md) for
  substantial implementation, refactoring, or review. Use only relevant items
  for a narrow change.

References refine the selected profile; they must not expand product scope.

## Report

For substantial work, state the selected profile and why, the component-system
decision, important accessibility and responsive decisions, tests changed,
commands run, assumptions, unverified behavior, and remaining risks.

For a narrow change, report only decisions and risks that materially affect the
result.
