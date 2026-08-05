## What this changes

<!-- One paragraph. What behaviour is different afterwards. -->

## Why

<!--
Link the ADR if this is a structural change, or the annotation if the
reasoning is bound to a place in the code. If neither exists yet and a future
reader could ask "why is it like this?", write one before opening this.
-->

## What you verified

<!--
Paste real output. "Tests pass" means you ran them and can show it.
Say explicitly what you did not verify.
-->

```
```

## Checklist

- [ ] `mise run test` passes, and the output is quoted above.
- [ ] `mise run annotations`, `mise run comments` and `mise run agent-policy` pass.
- [ ] No inline comment was added that a name, an extraction, a named type or a
      koment annotation could have replaced.
- [ ] A non-obvious decision has an ADR, and the ADR names the alternatives it
      rejected.
- [ ] The commit subject is conventional, and a breaking change uses `feat!:`
      with the evidence the back-compat principle requires.
- [ ] I have signed the [CLA](../CLA.md).
