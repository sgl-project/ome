# Comprehensive Evaluation Summary

**Timestamp:** Tue May 19 19:49:02 UTC 2026
**BFCL/LooGLE completion timestamp:** Fri May 29 21:15:37 UTC 2026
**lm_eval completion timestamp:** Fri May 29 22:21:43 UTC 2026
**Model:** /models/meta/llama-3-3-70b-instruct-fp8-dynamic
**Server:** http://localhost:8091

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-05-19 19:51:14,567 - INFO - ============================================================
2026-05-19 19:51:14,567 - INFO - Total tests: 4
2026-05-19 19:51:14,567 - INFO - Passed tests: 4
2026-05-19 19:51:14,567 - INFO - Failed tests: 0
2026-05-19 19:51:14,567 - INFO - Pass rate: 100.00%
2026-05-19 19:51:14,567 - INFO - All feature sanity checks completed successfully.
2026-05-19 19:51:14,567 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.10.post1.a78758ff-cu129', 'server_api': 'http://localhost:8091/v1/chat/completions', 'model_name': '/models/meta/llama-3-3-70b-instruct-fp8-dynamic'}
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
    Position 50: 3 times (11.1%)
    Position 136: 2 times (7.4%)
    Position 21: 2 times (7.4%)
    Position 13: 2 times (7.4%)
    Position 164: 2 times (7.4%)

  Difference types:
    token_mismatch: 27 (100.0%)

  Difference timing:
    Early differences (pos < 10): 2 (7.4%)
    Late differences (pos >= 10): 25 (92.6%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.5.10.post1.a78758ff-cu129
  Baseline Version: v0.5.7.dcf17400f-cu129
  Total tests: 33
  Mismatches: 27
  Missing baseline: 0
  Detailed analysis saved to: SGLang_vv0_5_10_post1_a78758ff-cu129/version_comparison.csv

Done!
```


### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-05-19 19:55:50,486 - INFO - ============================================================
2026-05-19 19:55:50,486 - INFO - CONSISTENCY CHECK SUMMARY
2026-05-19 19:55:50,486 - INFO - ============================================================
2026-05-19 19:55:50,486 - INFO - Sequential tests average consistency rate: 100.00%
2026-05-19 19:55:50,486 - INFO - Concurrent tests average consistency rate: 71.18%
2026-05-19 19:55:50,486 - INFO - Overall average consistency rate: 85.59%
2026-05-19 19:55:50,486 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.10.post1.a78758ff-cu129', 'server_api': 'http://localhost:8091/v1/chat/completions', 'model_name': '/models/meta/llama-3-3-70b-instruct-fp8-dynamic'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** evaluation-lm-eval/lm_eval_results/
- **Log:** evaluation-lm-eval/lm_eval.log
- **Run endpoint:** http://localhost:8080/v1

#### lm_eval Results:

| Task | Metric | Score | StdErr |
|---|---|---:|---:|
| mmlu_pro | exact_match/custom-extract | 70.28% | ±0.41% |

#### MMLU Pro Category Summary:

| Category | Score |
|---|---:|
| Biology | 85.63% |
| Business | 74.52% |
| Chemistry | 68.99% |
| Computer Science | 72.93% |
| Economics | 78.67% |
| Engineering | 53.15% |
| Health | 72.86% |
| History | 64.04% |
| Law | 52.04% |
| Math | 78.16% |
| Other | 71.43% |
| Philosophy | 63.33% |
| Physics | 70.98% |
| Psychology | 78.70% |

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** evaluation-bfcl-loogle/bfcl/score/data_overall.csv
- **Logs:** evaluation-bfcl-loogle/bfcl/bfcl.log, evaluation-bfcl-loogle/bfcl/bfcl_eval.log
- **Run endpoint:** http://localhost:8080/v1

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,28.06%
```

#### BFCL Category Summary:
```
Non-Live Overall Acc: 87.67%
Live Overall Acc: 76.24%
Multi Turn Overall Acc: 16.00%
Agentic Overall Acc: 3.87%
Relevance Detection: 100.00%
Irrelevance Detection: 53.20%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** evaluation-bfcl-loogle/loogle/
- **Log:** evaluation-bfcl-loogle/loogle/loogle.log
- **Run endpoint:** http://localhost:8091/v1

#### LooGLE Results (last 2 lines):
```
Average BERTScore (F1): 85.67%
```

### 7. Image Number Check
- **Status:** ❌ Not run
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
Log file not found
```

### 8. Image Size Check
- **Status:** ❌ Not run
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
Log file not found
```
