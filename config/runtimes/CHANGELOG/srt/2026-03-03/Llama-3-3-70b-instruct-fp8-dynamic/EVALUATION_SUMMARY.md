# Comprehensive Evaluation Summary

**Timestamp:** Fri Feb 27 22:51:25 PST 2026
**Model:** meta-llama/llama-3-3-70b-instruct-fp8-dynamic
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-02-27 20:58:09,548 - INFO - ============================================================
2026-02-27 20:58:09,548 - INFO - Total tests: 4
2026-02-27 20:58:09,548 - INFO - Passed tests: 4
2026-02-27 20:58:09,548 - INFO - Failed tests: 0
2026-02-27 20:58:09,548 - INFO - Pass rate: 100.00%
2026-02-27 20:58:09,548 - INFO - All feature sanity checks completed successfully.
2026-02-27 20:58:09,548 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7-HiCache-Flashinfer', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/llama-3-3-70b-instruct-fp8-dynamic'}
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
📊 TOKEN DIFFERENCE ANALYSIS
──────────────────────────────────────────────────
  First differing token positions:
    Min: 0
    Max: 144
    Average: 38.0
    Median: 40
    Mode: 2

  Most common difference positions:
    Position 2: 3 times (10.3%)
    Position 9: 3 times (10.3%)
    Position 79: 3 times (10.3%)
    Position 0: 3 times (10.3%)
    Position 48: 3 times (10.3%)

  Difference types:
    token_mismatch: 29 (100.0%)

  Difference timing:
    Early differences (pos < 10): 10 (34.5%)
    Late differences (pos >= 10): 19 (65.5%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.5.7-HiCache-Flashinfer
  Baseline Version: v0.5.7-noHiCache-Default-Backend
  Total tests: 33
  Mismatches: 29
  Missing baseline: 0
  Detailed analysis saved to: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_7-HiCache-Flashinfer_20260227_205801/version_comparison.csv

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-02-27 21:07:42,907 - INFO - ============================================================
2026-02-27 21:07:42,907 - INFO - CONSISTENCY CHECK SUMMARY
2026-02-27 21:07:42,907 - INFO - ============================================================
2026-02-27 21:07:42,908 - INFO - Sequential tests average consistency rate: 100.00%
2026-02-27 21:07:42,908 - INFO - Concurrent tests average consistency rate: 68.18%
2026-02-27 21:07:42,908 - INFO - Overall average consistency rate: 84.09%
2026-02-27 21:07:42,908 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7-HiCache-Flashinfer', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/llama-3-3-70b-instruct-fp8-dynamic'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7067|±  |0.0041|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_7-HiCache-Flashinfer_20260227_205801/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,27.75%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]Scoring batches:   6%|▌         | 1/18 [00:07<02:12,  7.82s/it]Scoring batches:  11%|█         | 2/18 [00:13<01:48,  6.79s/it]Scoring batches:  17%|█▋        | 3/18 [00:20<01:41,  6.79s/it]Scoring batches:  22%|██▏       | 4/18 [00:28<01:42,  7.33s/it]Scoring batches:  28%|██▊       | 5/18 [00:37<01:41,  7.80s/it]Scoring batches:  33%|███▎      | 6/18 [00:46<01:39,  8.33s/it]Scoring batches:  39%|███▉      | 7/18 [00:55<01:32,  8.37s/it]Scoring batches:  44%|████▍     | 8/18 [00:59<01:11,  7.20s/it]Scoring batches:  50%|█████     | 9/18 [01:03<00:54,  6.00s/it]Scoring batches:  56%|█████▌    | 10/18 [01:06<00:42,  5.26s/it]Scoring batches:  61%|██████    | 11/18 [01:12<00:37,  5.42s/it]Scoring batches:  67%|██████▋   | 12/18 [01:17<00:31,  5.25s/it]Scoring batches:  72%|███████▏  | 13/18 [01:23<00:26,  5.32s/it]Scoring batches:  78%|███████▊  | 14/18 [01:29<00:22,  5.73s/it]Scoring batches:  83%|████████▎ | 15/18 [01:35<00:16,  5.62s/it]Scoring batches:  89%|████████▉ | 16/18 [01:40<00:10,  5.50s/it]Scoring batches:  94%|█████████▍| 17/18 [01:49<00:06,  6.52s/it]Scoring batches: 100%|██████████| 18/18 [01:51<00:00,  5.17s/it]Scoring batches: 100%|██████████| 18/18 [01:51<00:00,  6.18s/it]
Average BERTScore (F1): 84.92%
```

### 7. Image Number Check
- **Status:** ❌ Failed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
python: can't open file '/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/openai_image_number_check.py': [Errno 2] No such file or directory
```

### 8. Image Size Check
- **Status:** ❌ Failed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
python: can't open file '/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/openai_image_size_check.py': [Errno 2] No such file or directory
```

## Files Generated
