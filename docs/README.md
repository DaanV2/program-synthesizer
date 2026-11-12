# synth documentation

- [Getting started](getting-started.md): install, create a project, add and
  import examples, change the spec, validate. Start here.
- [Project format](project-format.md): the project folder, every key of
  `spec.yaml`, the type syntax, example files, validation rules and
  contradictions.
- [Architecture](architecture.md): for contributors. Packages, key types,
  data flow, how to add a command, and where the code stands against the
  milestones.
- [CLI reference](cli/synth.md): one page per command, generated from the
  help text. Do not edit these pages; change the help text in
  `cmd/` and run `just docs`.
- [Design](../spec.md): how the synthesizer is meant to work.

The CLI reference shows what the cobra command tree defines. Three things
are added when the binary runs and are not listed there: `synth --version`,
`synth completion <shell>` (shell completion scripts) and `synth man` (a
man page).
