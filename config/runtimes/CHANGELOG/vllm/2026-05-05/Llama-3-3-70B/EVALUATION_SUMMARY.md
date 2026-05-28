# Comprehensive Evaluation Summary

**Timestamp:** Wed May  6 14:19:12 PDT 2026
**Model:** vllm-model
**Server:** http://localhost:8081

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-05-06 13:14:42,492 - INFO - ============================================================
2026-05-06 13:14:42,492 - INFO - Total tests: 4
2026-05-06 13:14:42,492 - INFO - Passed tests: 4
2026-05-06 13:14:42,492 - INFO - Failed tests: 0
2026-05-06 13:14:42,492 - INFO - Pass rate: 100.00%
2026-05-06 13:14:42,492 - INFO - All feature sanity checks completed successfully.
2026-05-06 13:14:42,493 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.20.0', 'server_api': 'http://localhost:8081/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
  - Prompt 4, max_tokens=300: HTTPConnectionPool(host='localhost', port=8082): Max retries exceeded with url: /v1/chat/completions (Caused by NewConnectionError('<urllib3.connection.HTTPConnection object at 0x1040ea810>: Failed to establish a new connection: [Errno 61] Connection refused'))
  - Prompt 3, max_tokens=300: HTTPConnectionPool(host='localhost', port=8082): Max retries exceeded with url: /v1/chat/completions (Caused by NewConnectionError('<urllib3.connection.HTTPConnection object at 0x1040e9a90>: Failed to establish a new connection: [Errno 61] Connection refused'))
  ... and 23 more
Generated/loaded 0 baseline results

=== Comparing Results ===

Comparing v0.20.0 against baseline v0.4.10.post2

📊 No token-level differences found

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.20.0
  Baseline Version: v0.4.10.post2
  Total tests: 33
  Mismatches: 0
  Missing baseline: 33

  Warning: 33 baseline results not found
  You may need to generate baseline results for version v0.4.10.post2

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-05-06 13:22:20,803 - INFO - ============================================================
2026-05-06 13:22:20,803 - INFO - CONSISTENCY CHECK SUMMARY
2026-05-06 13:22:20,803 - INFO - ============================================================
2026-05-06 13:22:20,804 - INFO - Sequential tests average consistency rate: 97.73%
2026-05-06 13:22:20,804 - INFO - Concurrent tests average consistency rate: 84.27%
2026-05-06 13:22:20,804 - INFO - Overall average consistency rate: 91.00%
2026-05-06 13:22:20,804 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.20.0', 'server_api': 'http://localhost:8081/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7063|±  |0.0041|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/vasheno/workspace/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_20_0_20260506_143601/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,10.00%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [01:01<17:30, 61.80s/it]
Scoring batches:  11%|█         | 2/18 [02:36<21:35, 80.96s/it]
Scoring batches:  17%|█▋        | 3/18 [04:04<21:03, 84.22s/it]
Scoring batches:  22%|██▏       | 4/18 [05:53<21:57, 94.10s/it]
Scoring batches:  28%|██▊       | 5/18 [07:33<20:48, 96.05s/it]
Scoring batches:  33%|███▎      | 6/18 [09:27<20:29, 102.42s/it]
Scoring batches:  39%|███▉      | 7/18 [11:05<18:27, 100.72s/it]
Scoring batches:  44%|████▍     | 8/18 [12:29<15:56, 95.69s/it] 
Scoring batches:  50%|█████     | 9/18 [13:09<11:44, 78.23s/it]
Scoring batches:  56%|█████▌    | 10/18 [13:44<08:37, 64.64s/it]
Scoring batches:  61%|██████    | 11/18 [14:53<07:43, 66.26s/it]
Scoring batches:  67%|██████▋   | 12/18 [15:55<06:29, 64.87s/it]
Scoring batches:  72%|███████▏  | 13/18 [16:48<05:06, 61.39s/it]
Scoring batches:  78%|███████▊  | 14/18 [17:46<04:01, 60.36s/it]
Scoring batches:  83%|████████▎ | 15/18 [18:55<03:08, 62.84s/it]
Scoring batches:  89%|████████▉ | 16/18 [19:55<02:03, 61.97s/it]
Scoring batches:  94%|█████████▍| 17/18 [21:19<01:08, 68.72s/it]
Scoring batches: 100%|██████████| 18/18 [21:43<00:00, 55.27s/it]
Scoring batches: 100%|██████████| 18/18 [21:43<00:00, 72.44s/it]
Average BERTScore (F1): 84.77%
```

### 7. Image Number Check
- **Status:** ❌ Failed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
  File "/Users/vasheno/workspace/moirai-internal/sanity_check/openai/openai_image_number_check.py", line 397, in <module>
    image_number_check()
  File "/Users/vasheno/workspace/moirai-internal/sanity_check/openai/openai_image_number_check.py", line 394, in image_number_check
    raise Exception(error_msg)
Exception: 14 tests failed out of 14
```

### 8. Image Size Check
- **Status:** ❌ Failed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
  File "/Users/vasheno/workspace/moirai-internal/sanity_check/openai/openai_image_size_check.py", line 264, in <module>
    image_size_check()
  File "/Users/vasheno/workspace/moirai-internal/sanity_check/openai/openai_image_size_check.py", line 261, in image_size_check
    raise Exception(error_msg)
Exception: 8 tests failed out of 8
```

## Files Generated
