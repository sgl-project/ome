# Comprehensive Evaluation Summary

**Timestamp:** Wed Jan 14 20:46:50 PST 2026
**Model:** meta/llama-3-3-70b-instruct-fp8-dynamic
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-01-14 18:27:48,823 - INFO - ============================================================
2026-01-14 18:27:48,823 - INFO - Total tests: 4
2026-01-14 18:27:48,823 - INFO - Passed tests: 4
2026-01-14 18:27:48,823 - INFO - Failed tests: 0
2026-01-14 18:27:48,823 - INFO - Pass rate: 100.00%
2026-01-14 18:27:48,823 - INFO - All feature sanity checks completed successfully.
2026-01-14 18:27:48,823 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta/llama-3-3-70b-instruct-fp8-dynamic'}
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
2026-01-14 18:38:03,926 - INFO - ============================================================
2026-01-14 18:38:03,926 - INFO - CONSISTENCY CHECK SUMMARY
2026-01-14 18:38:03,926 - INFO - ============================================================
2026-01-14 18:38:03,926 - INFO - Sequential tests average consistency rate: 99.55%
2026-01-14 18:38:03,926 - INFO - Concurrent tests average consistency rate: 95.27%
2026-01-14 18:38:03,926 - INFO - Overall average consistency rate: 97.41%
2026-01-14 18:38:03,926 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta/llama-3-3-70b-instruct-fp8-dynamic'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7023|±  |0.0041|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260114_182740/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,28.11%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [00:06<01:50,  6.51s/it]
Scoring batches:  11%|█         | 2/18 [00:13<01:48,  6.80s/it]
Scoring batches:  17%|█▋        | 3/18 [00:20<01:43,  6.90s/it]
Scoring batches:  22%|██▏       | 4/18 [00:27<01:35,  6.82s/it]
Scoring batches:  28%|██▊       | 5/18 [00:35<01:36,  7.41s/it]
Scoring batches:  33%|███▎      | 6/18 [00:45<01:38,  8.21s/it]
Scoring batches:  39%|███▉      | 7/18 [00:54<01:32,  8.44s/it]
Scoring batches:  44%|████▍     | 8/18 [00:59<01:13,  7.33s/it]
Scoring batches:  50%|█████     | 9/18 [01:02<00:54,  6.02s/it]
Scoring batches:  56%|█████▌    | 10/18 [01:06<00:42,  5.29s/it]
Scoring batches:  61%|██████    | 11/18 [01:11<00:37,  5.30s/it]
Scoring batches:  67%|██████▋   | 12/18 [01:17<00:33,  5.54s/it]
Scoring batches:  72%|███████▏  | 13/18 [01:24<00:29,  5.95s/it]
Scoring batches:  78%|███████▊  | 14/18 [01:31<00:25,  6.41s/it]
Scoring batches:  83%|████████▎ | 15/18 [01:37<00:18,  6.03s/it]
Scoring batches:  89%|████████▉ | 16/18 [01:40<00:10,  5.28s/it]
Scoring batches:  94%|█████████▍| 17/18 [01:48<00:05,  5.99s/it]
Scoring batches: 100%|██████████| 18/18 [01:49<00:00,  4.63s/it]
Scoring batches: 100%|██████████| 18/18 [01:49<00:00,  6.09s/it]
Average BERTScore (F1): 84.93%
```

### 7. Image Number Check
- **Status:** ✅ Completed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2026-01-14 20:43:58,747 - INFO - Testing with image saved as WEBP but declared as JPEG
2026-01-14 20:44:00,997 - INFO - Test WEBP-as-JPEG: Success
2026-01-14 20:44:00,997 - INFO - Testing with image saved as WEBP but declared as WEBP
2026-01-14 20:44:03,248 - INFO - Test WEBP-as-WEBP: Success
2026-01-14 20:44:03,261 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260114_182740/image_number_check/image_number_check_SGLang_v0.5.7.csv
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2026-01-14 20:45:07,680 - INFO - Testing image dimensions: 4097x4097
2026-01-14 20:45:29,620 - INFO - Dimensions 4097x4097: Success
2026-01-14 20:45:29,620 - INFO - Testing image dimensions: 8192x8192
2026-01-14 20:46:24,068 - INFO - Dimensions 8192x8192: Success
2026-01-14 20:46:24,072 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260114_182740/image_size_check/image_size_check_SGLang_v0.5.7.csv
```

## Files Generated
