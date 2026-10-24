---
description: Grant one named area on top of the current Rule #11 mode, until the mode changes
disable-model-invocation: true
argument-hint: "[area] [./path/to/service…]"
---

The specrig hook has answered this command in this turn's context: what was
granted, or which areas the current mode takes as grants when none was named. If
it says nothing was granted, tell the human why and stop.

Arguments: $ARGUMENTS

A grant covers exactly the area it names. If the work needs a second one, ask for
it — do not extend this one. If no area fits, quote the `agents.areas` entry for
the service's marker that would, as Rule #11 "Modes" says.
