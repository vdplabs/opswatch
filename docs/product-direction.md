# Product Direction

OpsWatch should optimize for timely operational warnings, not general screenshot intelligence.

## Core Job

Review what is on screen, map it against local context and policy, and alert fast enough that the operator can still change course.

If an alert arrives after the risky action is already complete, OpsWatch becomes a useful recorder, but not a strong safety tool. The product should prefer lower-latency extraction paths over more general model reasoning when both are available.

## Strategic Simplification

OpsWatch should not center the product around running a vision-language model on every frame.

The better architecture is:

1. use the cheapest reliable extractor for the selected screen
2. normalize into an operational event
3. evaluate policy
4. fall back to a slower VLM only when cheaper paths are not confident enough

## Default Extraction Ladder

1. terminal-specific extraction
2. Apple Vision OCR for text-heavy console pages
3. simple page heuristics for known operational surfaces
4. local VLM fallback for ambiguous or highly visual screens

## Performance Targets

- terminal actions should trend toward sub-2s analysis
- text-heavy cloud console pages should trend toward low single-digit seconds
- VLM fallback can be slower, but should be the exception rather than the norm

## What This Means For Implementation

- keep the policy engine as the stable center of the product
- keep events structured and model-agnostic
- treat OCR, parsers, heuristics, and VLMs as interchangeable adapters upstream of policy
- avoid adding runtime complexity when a cheaper extractor can solve the same screen
