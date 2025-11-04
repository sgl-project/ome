# Comprehensive Evaluation Summary

**Timestamp:** Sun Oct 26 22:06:02 UTC 2025
**Model:** meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8
**Server:** http://localhost:8090

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2025-10-26 21:13:39,484 - INFO - ============================================================
2025-10-26 21:13:39,484 - INFO - Total tests: 4
2025-10-26 21:13:39,484 - INFO - Passed tests: 4
2025-10-26 21:13:39,484 - INFO - Failed tests: 0
2025-10-26 21:13:39,484 - INFO - Pass rate: 100.00%
2025-10-26 21:13:39,484 - INFO - All feature sanity checks completed successfully.
2025-10-26 21:13:39,484 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.3.post1', 'server_api': 'http://localhost:8090/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8'}
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
2025-10-26 21:15:58,753 - INFO - ============================================================
2025-10-26 21:15:58,753 - INFO - CONSISTENCY CHECK SUMMARY
2025-10-26 21:15:58,753 - INFO - ============================================================
2025-10-26 21:15:58,753 - INFO - Sequential tests average consistency rate: 96.36%
2025-10-26 21:15:58,753 - INFO - Concurrent tests average consistency rate: 92.27%
2025-10-26 21:15:58,753 - INFO - Overall average consistency rate: 94.32%
2025-10-26 21:15:58,753 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.3.post1', 'server_api': 'http://localhost:8090/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.8134|±  |0.0035|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251027_010155/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,53.65%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [00:07<02:03,  7.26s/it]
Scoring batches:  11%|█         | 2/18 [00:14<01:55,  7.25s/it]
Scoring batches:  17%|█▋        | 3/18 [00:21<01:46,  7.07s/it]
Scoring batches:  22%|██▏       | 4/18 [00:28<01:37,  6.97s/it]
Scoring batches:  28%|██▊       | 5/18 [00:34<01:29,  6.89s/it]
Scoring batches:  33%|███▎      | 6/18 [00:42<01:25,  7.10s/it]
Scoring batches:  39%|███▉      | 7/18 [00:49<01:17,  7.01s/it]
Scoring batches:  44%|████▍     | 8/18 [00:55<01:09,  6.93s/it]
Scoring batches:  50%|█████     | 9/18 [01:02<01:01,  6.88s/it]
Scoring batches:  56%|█████▌    | 10/18 [01:09<00:54,  6.78s/it]
Scoring batches:  61%|██████    | 11/18 [01:16<00:48,  6.97s/it]
Scoring batches:  67%|██████▋   | 12/18 [01:23<00:40,  6.82s/it]
Scoring batches:  72%|███████▏  | 13/18 [01:30<00:34,  6.82s/it]
Scoring batches:  78%|███████▊  | 14/18 [01:36<00:27,  6.85s/it]
Scoring batches:  83%|████████▎ | 15/18 [01:43<00:20,  6.87s/it]
Scoring batches:  89%|████████▉ | 16/18 [01:50<00:13,  6.86s/it]
Scoring batches:  94%|█████████▍| 17/18 [01:57<00:06,  6.91s/it]
Scoring batches: 100%|██████████| 18/18 [01:58<00:00,  5.16s/it]
Scoring batches: 100%|██████████| 18/18 [01:58<00:00,  6.60s/it]
Average BERTScore (F1): 84.22%
```

### 7. Image Number Check
- **Status:** ✅ Completed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2025-10-27 00:41:34,864 - INFO - Testing with image saved as WEBP but declared as JPEG
2025-10-27 00:41:35,882 - INFO - Test WEBP-as-JPEG: Success
2025-10-27 00:41:35,882 - INFO - Testing with image saved as WEBP but declared as WEBP
2025-10-27 00:41:36,902 - INFO - Test WEBP-as-WEBP: Success
2025-10-27 00:41:36,907 - INFO - Results saved to /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251027_004100/image_number_check/image_number_check_SGLang_v0.5.3.post1.csv
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2025-10-26 22:55:29,321 - INFO - Testing image dimensions: 4097x4097
2025-10-26 22:55:31,421 - INFO - Dimensions 4097x4097: Success
2025-10-26 22:55:31,421 - INFO - Testing image dimensions: 8192x8192
2025-10-26 22:55:34,435 - INFO - Dimensions 8192x8192: Success
2025-10-26 22:55:34,450 - INFO - Results saved to /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251026_225518/image_size_check/image_size_check_SGLang_v0.5.3.post1.csv
```


