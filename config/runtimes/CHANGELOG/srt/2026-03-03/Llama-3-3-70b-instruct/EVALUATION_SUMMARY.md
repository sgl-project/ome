# Comprehensive Evaluation Summary

**Timestamp:** Mon Mar  2 12:35:03 PST 2026
**Model:** meta-llama/Llama-3.3-70B-Instruct
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-03-02 10:22:34,713 - INFO - ============================================================
2026-03-02 10:22:34,713 - INFO - Total tests: 4
2026-03-02 10:22:34,713 - INFO - Passed tests: 4
2026-03-02 10:22:34,713 - INFO - Failed tests: 0
2026-03-02 10:22:34,713 - INFO - Pass rate: 100.00%
2026-03-02 10:22:34,713 - INFO - All feature sanity checks completed successfully.
2026-03-02 10:22:34,713 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7-HiCache-Flashinfer', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-3.3-70B-Instruct'}
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
    Max: 105
    Average: 44.2
    Median: 42.0
    Mode: 24

  Most common difference positions:
    Position 24: 4 times (13.3%)
    Position 63: 3 times (10.0%)
    Position 41: 3 times (10.0%)
    Position 0: 3 times (10.0%)
    Position 57: 3 times (10.0%)

  Difference types:
    token_mismatch: 30 (100.0%)

  Difference timing:
    Early differences (pos < 10): 5 (16.7%)
    Late differences (pos >= 10): 25 (83.3%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.5.7-HiCache-Flashinfer
  Baseline Version: v0.5.7-noHiCache
  Total tests: 33
  Mismatches: 30
  Missing baseline: 0
  Detailed analysis saved to: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_7-HiCache-Flashinfer_20260302_102224/version_comparison.csv

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-03-02 10:33:46,698 - INFO - ============================================================
2026-03-02 10:33:46,698 - INFO - CONSISTENCY CHECK SUMMARY
2026-03-02 10:33:46,698 - INFO - ============================================================
2026-03-02 10:33:46,699 - INFO - Sequential tests average consistency rate: 97.27%
2026-03-02 10:33:46,699 - INFO - Concurrent tests average consistency rate: 85.64%
2026-03-02 10:33:46,699 - INFO - Overall average consistency rate: 91.45%
2026-03-02 10:33:46,699 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7-HiCache-Flashinfer', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-3.3-70B-Instruct'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value|   |Stderr|
|--------|------:|--------------|------|-----------|---|----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.705|±  |0.0041|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_7-HiCache-Flashinfer_20260302_102224/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,29.78%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]Scoring batches:   6%|▌         | 1/18 [00:08<02:27,  8.65s/it]Scoring batches:  11%|█         | 2/18 [00:16<02:11,  8.22s/it]Scoring batches:  17%|█▋        | 3/18 [00:24<01:59,  7.94s/it]Scoring batches:  22%|██▏       | 4/18 [00:31<01:49,  7.81s/it]Scoring batches:  28%|██▊       | 5/18 [00:40<01:44,  8.04s/it]Scoring batches:  33%|███▎      | 6/18 [00:50<01:45,  8.77s/it]Scoring batches:  39%|███▉      | 7/18 [00:58<01:35,  8.64s/it]Scoring batches:  44%|████▍     | 8/18 [01:03<01:15,  7.53s/it]Scoring batches:  50%|█████     | 9/18 [01:07<00:56,  6.28s/it]Scoring batches:  56%|█████▌    | 10/18 [01:10<00:42,  5.37s/it]Scoring batches:  61%|██████    | 11/18 [01:17<00:40,  5.81s/it]Scoring batches:  67%|██████▋   | 12/18 [01:22<00:33,  5.65s/it]Scoring batches:  72%|███████▏  | 13/18 [01:30<00:31,  6.21s/it]Scoring batches:  78%|███████▊  | 14/18 [01:35<00:23,  5.93s/it]Scoring batches:  83%|████████▎ | 15/18 [01:39<00:16,  5.37s/it]Scoring batches:  89%|████████▉ | 16/18 [01:46<00:11,  5.72s/it]Scoring batches:  94%|█████████▍| 17/18 [01:54<00:06,  6.52s/it]Scoring batches: 100%|██████████| 18/18 [01:56<00:00,  5.01s/it]Scoring batches: 100%|██████████| 18/18 [01:56<00:00,  6.45s/it]
Average BERTScore (F1): 84.83%
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
