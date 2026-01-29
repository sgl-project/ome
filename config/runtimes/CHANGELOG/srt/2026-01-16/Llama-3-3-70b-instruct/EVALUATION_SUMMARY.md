# Comprehensive Evaluation Summary

**Timestamp:** Wed Jan 14 17:52:55 PST 2026
**Model:** meta-llama/Llama-3.3-70B-Instruct
**Server:** http://localhost:8081

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-01-14 15:38:27,476 - INFO - ============================================================
2026-01-14 15:38:27,476 - INFO - Total tests: 4
2026-01-14 15:38:27,476 - INFO - Passed tests: 4
2026-01-14 15:38:27,476 - INFO - Failed tests: 0
2026-01-14 15:38:27,476 - INFO - Pass rate: 100.00%
2026-01-14 15:38:27,476 - INFO - All feature sanity checks completed successfully.
2026-01-14 15:38:27,476 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8081/v1/chat/completions', 'model_name': 'meta-llama/Llama-3.3-70B-Instruct'}
```

### 2. Version Comparison
- **Status:** ❌ Not run
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
Version comparison log file not found
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-01-14 15:49:56,042 - INFO - ============================================================
2026-01-14 15:49:56,042 - INFO - CONSISTENCY CHECK SUMMARY
2026-01-14 15:49:56,042 - INFO - ============================================================
2026-01-14 15:49:56,042 - INFO - Sequential tests average consistency rate: 100.00%
2026-01-14 15:49:56,042 - INFO - Concurrent tests average consistency rate: 82.45%
2026-01-14 15:49:56,042 - INFO - Overall average consistency rate: 91.23%
2026-01-14 15:49:56,042 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8081/v1/chat/completions', 'model_name': 'meta-llama/Llama-3.3-70B-Instruct'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7077|±  |0.0041|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260114_153819/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,29.76%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [00:07<02:08,  7.56s/it]
Scoring batches:  11%|█         | 2/18 [00:15<02:08,  8.03s/it]
Scoring batches:  17%|█▋        | 3/18 [00:21<01:42,  6.83s/it]
Scoring batches:  22%|██▏       | 4/18 [00:29<01:42,  7.33s/it]
Scoring batches:  28%|██▊       | 5/18 [00:36<01:34,  7.28s/it]
Scoring batches:  33%|███▎      | 6/18 [00:46<01:37,  8.11s/it]
Scoring batches:  39%|███▉      | 7/18 [00:54<01:30,  8.19s/it]
Scoring batches:  44%|████▍     | 8/18 [01:00<01:12,  7.29s/it]
Scoring batches:  50%|█████     | 9/18 [01:03<00:54,  6.11s/it]
Scoring batches:  56%|█████▌    | 10/18 [01:06<00:41,  5.21s/it]
Scoring batches:  61%|██████    | 11/18 [01:12<00:38,  5.47s/it]
Scoring batches:  67%|██████▋   | 12/18 [01:15<00:28,  4.74s/it]
Scoring batches:  72%|███████▏  | 13/18 [01:22<00:26,  5.21s/it]
Scoring batches:  78%|███████▊  | 14/18 [01:27<00:21,  5.26s/it]
Scoring batches:  83%|████████▎ | 15/18 [01:32<00:15,  5.26s/it]
Scoring batches:  89%|████████▉ | 16/18 [01:37<00:10,  5.12s/it]
Scoring batches:  94%|█████████▍| 17/18 [01:45<00:06,  6.08s/it]
Scoring batches: 100%|██████████| 18/18 [01:47<00:00,  4.69s/it]
Scoring batches: 100%|██████████| 18/18 [01:47<00:00,  5.96s/it]
Average BERTScore (F1): 84.80%
```

### 7. Image Number Check
- **Status:** ✅ Completed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2026-01-14 17:51:33,219 - INFO - Testing with image saved as WEBP but declared as JPEG
2026-01-14 17:51:35,931 - INFO - Test WEBP-as-JPEG: Success
2026-01-14 17:51:35,931 - INFO - Testing with image saved as WEBP but declared as WEBP
2026-01-14 17:51:38,567 - INFO - Test WEBP-as-WEBP: Success
2026-01-14 17:51:38,587 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260114_153819/image_number_check/image_number_check_SGLang_v0.5.7.csv
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2026-01-14 17:51:53,819 - INFO - Testing image dimensions: 4097x4097
2026-01-14 17:51:56,764 - INFO - Dimensions 4097x4097: Success
2026-01-14 17:51:56,764 - INFO - Testing image dimensions: 8192x8192
2026-01-14 17:52:01,024 - INFO - Dimensions 8192x8192: Success
2026-01-14 17:52:01,029 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260114_153819/image_size_check/image_size_check_SGLang_v0.5.7.csv
```

## Files Generated
