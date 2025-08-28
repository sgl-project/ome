<!-- Important: fill in all the sections in this template -->
## [Title: Concise Summary of Update]
<!-- Mention which models or components are updated, e.g., "Update llama-4-scout with new sglang image version" -->

### Related Branch or Tag
<!-- Provide GitHub branch/tag reference with link -->
[+repo+/+branch-or-tag+](https://github.com/org/repo/tree/branch-or-tag)

### Artifacts
<!-- List all OCIR image paths involved -->
- ord.ocir.io/.../image:tag
- fra.ocir.io/.../image:tag

### Tag & Source
<!-- Mention Git tag and base branch/tag used -->
- Based on tag: `vX.Y.Z`
- From branch: `feature-branch-name` (if applicable)

---

### Changes
<!-- List all relevant code or behavior changes -->
- Change 1: Describe briefly
- Change 2: Fixes, improvements, or enhancements
- Change 3: Include links to relevant JIRA tickets if needed
- ...
<!-- Follow with critical bug fixes, support for new features, internal refactors, etc. -->

---

### Performance Benchmarks
<!-- Provide benchmark details per model; include test version, setup, and metrics. Do not link the confluence page,
instead add the content here-->
Benchmark results using `genai-bench vX.Y.Z`

#### [Model Name (e.g., llama-4-scout-17b)]
Tested with [# GPUs] x [GPU Model]

| Scenario                           | TTFT (sec) | Max Server Output Throughput (tokens/sec) |
|------------------------------------|------------|-------------------------------------------|
| Scenario 1: Fusion N(...)          |            |                                           |
| Scenario 2: Chatbot/Dialog (...)   |            |                                           |
| Scenario 3: Generation Heavy (...) |            |                                           |
| Scenario 4: Typical RAG (...)      |            |                                           |
| Scenario 5: Heavier RAG (...)      |            |                                           |
| Scenario 6: Heaviest RAG (...)     |            |                                           |

---

### Performance comparison plots
<!-- Speed vs throughput graphs against previous deployed. Please ensure no regression in performance -->

### Sanity & Eval Results
<!-- Include pass/fail thresholds and evaluation scores -->

- [ ] Put version comparison results to features branch on github. File link: [Link]

| Benchmark Suite    | Result (%) | Threshold (%)                                              |
|--------------------|------------|------------------------------------------------------------|
| Version Comparison |            |                                                            |
| Consistency        |            |                                                            |
| Features Test      |            |                                                            |
| Loogle Eval        |            |                                                            |
| GSM8K              |            |                                                            |
| MMLU Pro           |            |                                                            |
| BFCL               |            | Check out https://gorilla.cs.berkeley.edu/leaderboard.html |
| [Other Benchmarks] |            |                                                            |

---

### Known Limitations
<!-- Note any known bugs, regressions, or gaps -->
- Limitation 1: Description and impact
- Limitation 2: Reference JIRA if applicable

---

### Breaking Changes
<!-- Clearly state if any API/config/model behaviors break backward compatibility -->
- N/A
<!-- OR -->
- Change 1: Description
- Change 2: Description
<!-- List any Nvidia driver, hardware requirements for this release -->

---

### Model Coverage
<!-- List all models affected or updated in this change -->
- Model 1 (e.g., llama-4-maverick-17b-128e-instruct)
- Model 2
- ...

---

### OCIR Image Path(s)
<!-- List full paths of images pushed -->
- ord.ocir.io/...:tag
- fra.ocir.io/...:tag

---

### Sign-off
<!-- Tag responsible parties or approvers -->
@yourname  
@teammate  
