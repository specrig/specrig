---
description: Switch a service to Rule #11 redgreen mode — steps until the frozen cases are red, then production code until they pass
disable-model-invocation: true
argument-hint: "[./path/to/service… or ./...]"
---

The specrig hook has answered this command in this turn's context: which services
were switched and what redgreen mode allows. If it says a service was not
switched, tell the human why and stop.

Arguments: $ARGUMENTS

Switching to redgreen mode from spec mode is the approval of the specification
handed over there, and freezes it. Work as Rule #11 step 3 says, and stop at the
hand-over.
