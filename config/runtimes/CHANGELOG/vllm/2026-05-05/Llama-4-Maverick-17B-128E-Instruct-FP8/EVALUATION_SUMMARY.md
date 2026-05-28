# Comprehensive Evaluation Summary

**Timestamp:** Wed Apr 15 17:00:55 PDT 2026
**Model:** vllm-model
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-04-15 16:02:02,451 - INFO - ============================================================
2026-04-15 16:02:02,451 - INFO - Total tests: 4
2026-04-15 16:02:02,452 - INFO - Passed tests: 4
2026-04-15 16:02:02,452 - INFO - Failed tests: 0
2026-04-15 16:02:02,452 - INFO - Pass rate: 100.00%
2026-04-15 16:02:02,452 - INFO - All feature sanity checks completed successfully.
2026-04-15 16:02:02,452 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.19.0', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
    Position 8: 6 times (19.4%)
    Position 12: 3 times (9.7%)
    Position 1: 3 times (9.7%)
    Position 63: 3 times (9.7%)
    Position 24: 3 times (9.7%)

  Difference types:
    token_mismatch: 31 (100.0%)

  Difference timing:
    Early differences (pos < 10): 14 (45.2%)
    Late differences (pos >= 10): 17 (54.8%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.19.0
  Baseline Version: 0.5.7
  Total tests: 33
  Mismatches: 31
  Missing baseline: 0
  Detailed analysis saved to: /Users/vasheno/workspace/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_19_0_20260415_160151/version_comparison.csv

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-04-15 16:08:07,930 - INFO - ============================================================
2026-04-15 16:08:07,930 - INFO - CONSISTENCY CHECK SUMMARY
2026-04-15 16:08:07,930 - INFO - ============================================================
2026-04-15 16:08:07,930 - INFO - Sequential tests average consistency rate: 97.73%
2026-04-15 16:08:07,930 - INFO - Concurrent tests average consistency rate: 63.64%
2026-04-15 16:08:07,930 - INFO - Overall average consistency rate: 80.68%
2026-04-15 16:08:07,930 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.19.0', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.8098|±  |0.0035|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/vasheno/workspace/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_19_0_20260415_160151/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,19.73%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]Scoring batches:   6%|▌         | 1/18 [01:25<24:05, 85.01s/it]Scoring batches:  11%|█         | 2/18 [02:57<23:46, 89.14s/it]Scoring batches:  17%|█▋        | 3/18 [04:31<22:50, 91.34s/it]Scoring batches:  22%|██▏       | 4/18 [06:07<21:46, 93.35s/it]Scoring batches:  28%|██▊       | 5/18 [07:43<20:27, 94.40s/it]Scoring batches:  33%|███▎      | 6/18 [09:36<20:07, 100.60s/it]Scoring batches:  39%|███▉      | 7/18 [11:11<18:07, 98.85s/it] Scoring batches:  44%|████▍     | 8/18 [12:48<16:23, 98.30s/it]Scoring batches:  50%|█████     | 9/18 [14:18<14:21, 95.70s/it]Scoring batches:  56%|█████▌    | 10/18 [15:48<12:30, 93.79s/it]Scoring batches:  61%|██████    | 11/18 [17:25<11:03, 94.72s/it]Scoring batches:  67%|██████▋   | 12/18 [18:58<09:25, 94.28s/it]Scoring batches:  72%|███████▏  | 13/18 [20:33<07:53, 94.70s/it]Scoring batches:  78%|███████▊  | 14/18 [22:10<06:20, 95.14s/it]Scoring batches:  83%|████████▎ | 15/18 [23:44<04:44, 94.90s/it]Scoring batches:  89%|████████▉ | 16/18 [25:18<03:09, 94.61s/it]Scoring batches:  94%|█████████▍| 17/18 [26:55<01:35, 95.46s/it]Scoring batches: 100%|██████████| 18/18 [27:22<00:00, 74.67s/it]Scoring batches: 100%|██████████| 18/18 [27:22<00:00, 91.23s/it]
Average BERTScore (F1): 84.06%
```

## Files Generated
