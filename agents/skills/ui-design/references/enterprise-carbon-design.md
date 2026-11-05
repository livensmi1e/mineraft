# Enterprise Carbon Design

Load this profile only when Carbon was selected for a greenfield business or
enterprise application, the repository already uses Carbon, the user explicitly
requests Carbon, or the direction is Carbon-inspired.

This profile is for productive application UI: operational workflows, forms,
data, navigation, administration, and decisions. It is not a guide for cloning
IBM.com, IBM marketing pages, or expressive brand experiences.

## Decide whether Carbon is appropriate

Carbon is a strong default when a compatible greenfield product needs consistent
behavior across many screens, recurring enterprise interactions, multiple user
roles, permissions, dense data, and long-term maintenance.

Do not add Carbon when:

- an existing design system already solves the surface;
- the application stack is incompatible;
- the surface is small enough for semantic HTML and focused CSS;
- adopting it would create two full component systems; or
- the user has not approved the new dependency when approval is required.

## Distinguish installed Carbon from Carbon-inspired design

### When Carbon is installed

- Treat the installed version, its package exports, Storybook, and matching
  official documentation as the source of truth.
- Reuse official components, patterns, themes, and role-based tokens.
- Do not recreate Carbon components manually or maintain local imitations.
- Do not copy numeric values from this reference when the package defines them.
- Preserve component accessibility and interaction contracts when composing or
  styling them.
- Do not mix Carbon with shadcn/ui or another full component system in the same
  surface.
- Add only packages and components required by the current task.

### When the direction is only Carbon-inspired

- Apply transferable principles: productive density, clear hierarchy, semantic
  color roles, disciplined spacing, surface layering, and predictable workflows.
- Use project-owned semantic tokens; do not require IBM-specific `--cds-*` names.
- Treat colors, fonts, radii, spacing, and dimensions as adaptable project
  decisions rather than Carbon compliance requirements.
- Do not use IBM logos or imply IBM endorsement.
- Do not claim that the result is Carbon-compliant.
- Do not clone IBM.com or imitate an IBM product.
- Prefer existing accessible primitives over hand-building complex widgets.

## Design productive application UI

Keep users focused on completing tasks. Favor compact, stable layouts over
marketing composition, oversized typography, dramatic imagery, or decorative
motion. Use expressive moments only when they help orientation or comprehension,
not as the default language of an operational screen.

Make primary actions and current state easy to find. Multiple actions may share
emphasis when the workflow gives them equal importance. Keep destructive,
infrequent, and contextual actions distinct without hiding them unnecessarily.

Avoid card grids as a default layout. Use containers for real grouping,
interaction boundaries, or reusable objects. Prefer grid alignment, typography,
spacing, dividers, structured lists, and tables for ordinary organization.

## Use semantic tokens and layers

- Name color roles by purpose: background, layer, text, icon, border, link,
  focus, support, danger, and status.
- Keep most surfaces neutral. Reserve action colors for interactive meaning and
  support colors for status.
- Never rely on color alone to communicate state.
- Use background layers and borders to show containment and hierarchy.
- Reserve shadows for elements that genuinely float above content, such as
  menus, popovers, and dialogs.
- Keep tokens consistent across light, dark, and nested surfaces.

When Carbon is installed, use its current theme and layer tokens. When it is only
an influence, create project-owned role tokens that express the same separation
of role from value.

## Apply grid, spacing, and typography with discipline

- Use the installed Carbon grid and spacing tokens when available.
- Otherwise choose a small spacing scale and a grid appropriate to the product.
- Align page titles, forms, filters, tables, and supporting content to shared
  columns.
- Preserve readable line lengths for prose while giving operational data enough
  horizontal space.
- Use productive typography for forms, controls, tables, and sustained tasks.
- Keep type roles consistent within a component or discrete workflow.
- Prefer repository typography unless Carbon adoption explicitly includes its
  type system.

Do not paste fixed breakpoint, type, radius, or spacing tables into application
code. Read the installed version's tokens and official documentation.

## Build enterprise workflows

### Navigation

- Derive navigation from user roles, information architecture, and recurring
  tasks.
- Keep global, local, and contextual navigation visually distinct.
- Preserve location and state across multi-step workflows where appropriate.
- Do not add sidebars, tabs, breadcrumbs, or switchers without a navigation need.

### Forms

- Group fields by user decision or business concept.
- Use labels, helper text, constraints, validation, and errors deliberately.
- Keep dependent fields and conditional sections predictable.
- Protect entered data during errors, retries, and multi-step transitions.
- Place irreversible consequences near the responsible action.

### Tables and filtering

- Use tables when users compare values across rows or act on records.
- Give important tables enough width; avoid trapping them inside small cards.
- Put search, filters, display controls, and global actions near the data they
  affect.
- Show active filters and provide a predictable reset path.
- Reveal batch actions after selection and keep row actions associated with their
  record.
- Choose pagination, virtualization, expansion, or a detail view from data volume
  and task needs rather than decoration.

### Permissions and workflow states

- Represent permissions in both available actions and explanatory feedback.
- Distinguish unavailable, disabled, pending, read-only, and unauthorized states.
- Make ownership, approval state, progress, status, and next steps legible.
- Handle loading, empty, partial-data, success, warning, error, and stale-data
  states where relevant.
- Do not expose controls that can never succeed for the current user unless the
  explanation itself is useful.

## Responsive behavior

- Design from task and content constraints rather than device mockups.
- Preserve the primary workflow and critical status information at narrow widths.
- Reflow page regions before hiding information.
- Collapse secondary controls before primary actions.
- For complex tables, deliberately choose horizontal scrolling, column priority,
  or an alternate compact view.
- Keep touch, keyboard, pointer, and zoom use practical.
- Test long labels, translated copy, long values, validation messages, and empty
  content.

## Accessibility

Carbon components improve the baseline but do not make the composed application
accessible automatically.

- Preserve semantic structure, labels, descriptions, and meaningful heading
  order.
- Maintain logical keyboard order, visible focus, and expected escape behavior.
- Use accessible names and focus management for dialogs, menus, and popovers.
- Associate validation and status messages with the relevant control or region.
- Maintain sufficient contrast across themes and states.
- Announce asynchronous workflow changes when users need that information.
- Respect reduced-motion preferences.

## Official sources

Prefer these sources over copied specifications:

- [About Carbon](https://www.carbondesignsystem.com/getting-started/about-carbon)
- [Designing overview](https://www.carbondesignsystem.com/getting-started/designing/overview)
- [Themes and tokens](https://www.carbondesignsystem.com/building-blocks/foundations/themes/overview)
- [Color roles and layering](https://www.carbondesignsystem.com/building-blocks/foundations/color/overview)
- [Typography strategies](https://www.carbondesignsystem.com/building-blocks/foundations/typography/style-strategies)
- [2x Grid](https://www.carbondesignsystem.com/building-blocks/foundations/2x-grid/guidelines)
- [Accessibility](https://www.carbondesignsystem.com/building-blocks/foundations/accessibility/overview)
- [Component overview](https://www.carbondesignsystem.com/building-blocks/core/components/overview/components)
- [React Storybook](https://react.carbondesignsystem.com/)

For component-specific work, open the official usage, accessibility, and code
documentation for the installed version before implementation.
