---
description: Switch a service to Rule #11 redgreen mode — steps until the frozen cases are red, then production code until they pass
disable-model-invocation: true
argument-hint: "[./path/to/service… or ./...]"
---

The specrig hook has answered this command in this turn's context: which services
were switched, what redgreen mode allows, and whether the switch is the approval.
If it says a service was not switched, tell the human why and stop.

Arguments: $ARGUMENTS

Where the hook says the switch is the approval of the specification handed over in
spec mode, it freezes it: work as Rule #11 step 3 says, and stop at the hand-over.
Where it says the switch is not an approval, the services are in redgreen with no
Rule #11 task started: say so, and wait for the task.
