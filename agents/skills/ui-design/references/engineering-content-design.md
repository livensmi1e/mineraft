# Engineering Content-First Design

Load this profile for personal applications, proofs of concept, quick tools,
small internal utilities, documentation, knowledge interfaces, read-oriented
surfaces, and content-heavy applications with limited recurring interaction.

The goal is a modern, durable engineering interface: direct, readable, useful,
and light on dependencies. It is not an instruction to imitate an old website or
make every personal application look like a wiki.

## Start from the information architecture

- Identify what users need to read, compare, find, decide, and do.
- Organize content around those tasks, not around visual components.
- Put common paths and essential information first.
- Use progressive disclosure for secondary detail without hiding primary work.
- Create stable, descriptive URLs and page titles when the application supports
  navigation.
- Use internal links, related links, categories, and search when the content
  volume makes browsing alone insufficient.
- Add a table of contents when a long page has meaningful sections.

Prefer a small number of obvious navigation regions. Do not add a sidebar, tabs,
command menu, or dashboard merely to make the application feel substantial.

## Prefer semantic HTML and native behavior

- Choose landmarks, headings, links, buttons, lists, tables, forms, details, and
  other semantic elements before custom widgets.
- Use native controls when their behavior meets the task.
- Preserve text selection, link destinations, browser history, form submission,
  autofill, find-in-page, and open-in-new-tab behavior.
- Use JavaScript for interaction that HTML and CSS cannot express clearly.
- Add an accessible primitive or existing project component when native behavior
  is insufficient for a complex dialog, menu, combobox, or similar widget.

Raw engineering UI does not mean careless UI. Controls still need labels,
validation, feedback, keyboard operation, visible focus, and clear states.

## Make technical content readable

- Use a restrained type scale with obvious heading levels.
- Choose a comfortable measure for prose; do not force tables, code, or logs into
  the same narrow width.
- Keep paragraphs focused and use lists only when they improve scanning.
- Use descriptive link text and preserve visible link affordance.
- Present code in semantic code blocks with wrapping or scrolling chosen
  deliberately.
- Label examples, warnings, notes, and commands by purpose rather than decoration.
- Use tables for cross-row comparison, not as a generic layout mechanism.
- Keep captions, units, timestamps, versions, and source metadata close to the
  values they explain.

Useful density is allowed. A reference page may expose many links or values when
grouping, typography, and labels keep it scannable. Do not confuse simplicity
with sparse content.

## Keep visual treatment quiet

- Let typography, spacing, alignment, lists, tables, and simple dividers carry
  most hierarchy.
- Use mostly neutral surfaces with deliberate action and status colors.
- Use cards only for self-contained objects or interaction units.
- Avoid nested panels, decorative dashboards, oversized hero type, glass effects,
  ornamental gradients, heavy shadows, and excessive rounding.
- Use icons only when they clarify an action or status.
- Do not turn every piece of metadata into a badge or pill.
- Follow existing project tokens when present; otherwise define only the few
  semantic values the surface needs.

Do not decorate empty space. Whitespace should separate concepts, preserve
reading rhythm, or make controls easier to operate.

## Support interactive personal tools

A content-first profile can still contain editors, forms, previews, filters,
commands, or data views. Give the main task the structure it requires.

- Keep input and output relationships obvious.
- Place controls near the content they affect.
- Make primary actions clear; allow equal emphasis when the workflow genuinely
  has several peer actions.
- Preserve user input through recoverable errors where practical.
- Show constraints before submission and recovery steps after failure.
- Add dependencies only when they provide meaningful interaction or accessibility
  behavior that native controls cannot.

Do not remove necessary controls, feedback, or domain complexity in the name of a
raw aesthetic.

## Design for responsive and accessible use

- Reflow content naturally and preserve the primary task at narrow widths.
- Collapse secondary navigation before hiding important information.
- Let wide tables and code blocks scroll only when their structure requires it.
- Consider long labels, long values, missing sections, empty data, and translated
  copy.
- Use semantic structure, meaningful heading order, labels, keyboard access,
  visible focus, and sufficient contrast.
- Never communicate meaning by color alone.
- Associate errors with controls and announce important asynchronous changes.
- Respect reduced-motion preferences.
- Implement relevant hover, focus, selected, disabled, loading, empty,
  partial-data, success, warning, and error states.

## Keep the implementation small

- Prefer platform features and dependencies already present.
- Do not create a design system for a small tool.
- Keep state local to the interaction that owns it.
- Extract components for repetition, complex behavior, accessibility, or a clear
  domain boundary, not to wrap simple markup.
- Keep styling close to the surface unless the repository has an established
  organization.
- Avoid speculative themes, layout modes, customization, and configuration.

## Inspiration, not templates

Use these sources to study transferable decisions:

- [cppreference](https://cppreference.com/): dense reference navigation,
  stable terminology, cross-linking, and scannable technical tables.
- [ArchWiki](https://wiki.archlinux.org/title/Main_page): task-oriented entry
  points, search, related articles, and maintained knowledge structure.
- [Chess Programming Wiki](https://www.chessprogramming.org/Main_Page): distinct
  paths for beginners, major concepts, and deep reference material.
- [Go documentation](https://go.dev/doc/): clear task categories, concise
  descriptions, and direct routes from learning to reference.
- [Simon Willison](https://simonwillison.net/), [Julia Evans](https://jvns.ca/),
  and [Russ Cox](https://research.swtch.com/): descriptive links, content-led
  hierarchy, compact metadata, and durable publishing structures.

Extract the principle that fits the current task. Do not copy branding, personal
voice, page composition, or outdated interaction patterns. Evaluate each source
against the current audience, content, device constraints, and product lifetime.
