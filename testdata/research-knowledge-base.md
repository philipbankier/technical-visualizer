# AI Agent Skills - Research Knowledge Base

## Executive Summary

Agent skills are reusable, composable units of procedural knowledge for AI coding and research agents. The core pattern is to encode a task recipe, required context, tool constraints, and validation gates so agents can perform specialized work without relearning the workflow.

The strongest findings across the corpus are:

- Skill libraries improve repeatability when each skill has a clear trigger, minimal context, and a concrete validation loop.
- SkillOpt reports +23.5 accuracy on held-out tasks after optimizing skill instructions.
- The best operational pattern is a 5-layer skill stack: trigger, context, procedure, tools, and validation.
- Human review remains necessary for high-risk outputs because agents can overfit to examples and omit source uncertainty.

## Core Papers

### 1. Toolformer

- Authors: Schick et al.
- Date: 2023-02
- Identifier: arXiv:2302.04761
- Finding: language models can learn API-use examples from self-supervised traces.
- Relevance: tool-use examples can become reusable procedural memory.

### 2. Voyager

- Authors: Wang et al.
- Date: 2023-05
- Identifier: arXiv:2305.16291
- Finding: an agent can build a skill library from exploration and reuse skills in new tasks.
- Result: solved 52/52 discovered tasks in the Minecraft evaluation.

### 3. Reflexion

- Authors: Shinn et al.
- Date: 2023-03
- Identifier: arXiv:2303.11366
- Finding: verbal reinforcement improves future attempts through compact feedback memory.
- Result: improved pass@1 on coding tasks after 1-4 edits.

### 4. SkillOpt

- Authors: Example Research Lab
- Date: 2025-01
- Identifier: arXiv:2501.01234
- Finding: automatic skill prompt optimization improves agent task success.
- Result: +23.5 accuracy, 85-2K tokens per optimized skill.

## Key Concepts & Taxonomy

| Layer | Purpose | Failure Mode | Validation |
| --- | --- | --- | --- |
| Trigger | Decides when to activate | Over-triggering | Negative examples |
| Context | Loads relevant facts | Context bloat | Token budget check |
| Procedure | Lists ordered actions | Vague steps | Red-green task proof |
| Tools | Names allowed tools | Tool mismatch | Dry run |
| Validation | Defines done criteria | False confidence | Independent review |

## SkillOpt Deep Dive

SkillOpt treats a skill as an optimizable prompt program:

1. collect task traces
2. identify recurring failures
3. mutate skill instructions
4. evaluate on held-out tasks
5. keep revisions with better success and lower token cost

Deep learning analogy:

```text
task traces -> loss signal -> prompt mutation -> validation split -> skill checkpoint
```

ASCII pipeline:

```text
[Tasks] -> [Failures] -> [Skill Mutator] -> [Evaluator] -> [Skill Library]
             ^                                             |
             +---------------- feedback -------------------+
```

## Timeline

- 2023-02: Toolformer shows self-supervised tool-use examples.
- 2023-03: Reflexion shows verbal feedback memory improves coding attempts.
- 2023-05: Voyager demonstrates autonomous skill libraries.
- 2024-09: Production agent teams standardize skill registries.
- 2025-01: SkillOpt reports automated skill prompt optimization.

## Scaling Laws

| Variable | Small | Medium | Large |
| --- | --- | --- | --- |
| Skill length | 85 tokens | 600 tokens | 2K tokens |
| Review burden | 1 reviewer | 2 reviewers | 3 reviewers |
| Failure recovery | manual | semi-automatic | policy-driven |

Key statistic: skill quality improves fastest when review examples stay below 12 per skill and each edit changes one behavior.

## Transfer Results

- Documentation skills transfer well across repos when tool names are abstracted.
- Debugging skills transfer poorly unless environment assumptions are explicit.
- Security skills need stricter validation because false negatives are costly.

## Open Questions

1. How should agents choose between overlapping skills?
2. Can skill quality be measured without task-specific gold labels?
3. What is the best way to decay stale skills?
4. How much source text should a skill cite?
5. Can skill libraries be safely shared across organizations?
6. What review protocol catches hallucinated tool affordances?
7. How should skills expose privacy boundaries?
8. Can optimized skills overfit to benchmark phrasing?
9. What is the minimum metadata for discoverability?
10. How should agents explain why a skill was triggered?
