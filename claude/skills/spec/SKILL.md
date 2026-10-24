---
description: Switch a service to Rule #11 spec mode — the cases and the delta list
disable-model-invocation: true
argument-hint: "[./path/to/service… or ./...] [task]"
---

The specrig hook has answered this command in this turn's context: which services
were switched and what spec mode allows. If it says a service was not switched,
tell the human why and stop.

Arguments: $ARGUMENTS

The paths name the services; the rest, if any, names the task. The requirements come from
this conversation; if there are none yet, ask for them and stay in spec mode
while you wait. Work as Rule #11 steps 0–2 say, and stop at the hand-over.
