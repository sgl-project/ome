# Comprehensive Evaluation Summary

**Timestamp:** Mon Apr 13 14:02:14 EDT 2026
**Model:** vllm-model
**Server:** http://localhost:8089

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
Exception: 1 tests failed out of 4
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.18.0_vllm_sarashina_35b', 'server_api': 'http://localhost:8089/v1/chat/completions', 'model_name': 'vllm-model'}
Traceback (most recent call last):
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_chat_feature_tests.py", line 341, in <module>
    main()
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_chat_feature_tests.py", line 331, in main
    raise Exception(
Exception: 1 tests failed out of 4
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
Current Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.18.0_vllm_sarashina_35b', 'server_api': 'http://localhost:8089/v1/chat/completions', 'model_name': 'vllm-model', 'max_tokens': [100, 200, 300]}
Detected mode: generate_current

=== Generating Current Results (v0.18.0_vllm_sarashina_35b) ===

Generating results for v0.18.0_vllm_sarashina_35b (vllm-model)
Force regenerate: False
Using 8 concurrent workers
Executing 33 API requests concurrently...
Processing requests:   0%|          | 0/33 [00:00<?, ?req/s]Processing requests:   3%|▎         | 1/33 [00:02<01:16,  2.39s/req]Processing requests:   9%|▉         | 3/33 [00:02<00:21,  1.38req/s]Processing requests:  12%|█▏        | 4/33 [00:02<00:16,  1.75req/s]Processing requests:  15%|█▌        | 5/33 [00:03<00:15,  1.82req/s]Processing requests:  18%|█▊        | 6/33 [00:03<00:12,  2.16req/s]Processing requests:  21%|██        | 7/33 [00:04<00:12,  2.14req/s]Processing requests:  24%|██▍       | 8/33 [00:04<00:10,  2.37req/s]Processing requests:  27%|██▋       | 9/33 [00:04<00:08,  2.83req/s]Processing requests:  30%|███       | 10/33 [00:05<00:07,  3.04req/s]Processing requests:  33%|███▎      | 11/33 [00:05<00:07,  3.02req/s]Processing requests:  36%|███▋      | 12/33 [00:05<00:08,  2.60req/s]Processing requests:  39%|███▉      | 13/33 [00:06<00:09,  2.02req/s]Processing requests:  45%|████▌     | 15/33 [00:06<00:06,  2.95req/s]Processing requests:  48%|████▊     | 16/33 [00:07<00:05,  3.03req/s]Processing requests:  52%|█████▏    | 17/33 [00:07<00:06,  2.36req/s]Processing requests:  55%|█████▍    | 18/33 [00:08<00:07,  2.06req/s]Processing requests:  58%|█████▊    | 19/33 [00:08<00:05,  2.58req/s]Processing requests:  61%|██████    | 20/33 [00:09<00:04,  2.66req/s]Processing requests:  64%|██████▎   | 21/33 [00:09<00:04,  2.50req/s]Processing requests:  67%|██████▋   | 22/33 [00:09<00:04,  2.72req/s]Processing requests:  70%|██████▉   | 23/33 [00:10<00:03,  2.85req/s]Processing requests:  73%|███████▎  | 24/33 [00:10<00:04,  2.21req/s]Processing requests:  76%|███████▌  | 25/33 [00:10<00:02,  2.80req/s]Processing requests:  79%|███████▉  | 26/33 [00:11<00:02,  3.35req/s]Processing requests:  82%|████████▏ | 27/33 [00:11<00:01,  3.09req/s]Processing requests:  85%|████████▍ | 28/33 [00:11<00:01,  3.77req/s]Processing requests:  88%|████████▊ | 29/33 [00:11<00:00,  4.55req/s]Processing requests:  94%|█████████▍| 31/33 [00:12<00:00,  2.91req/s]Processing requests:  97%|█████████▋| 32/33 [00:13<00:00,  2.65req/s]Processing requests: 100%|██████████| 33/33 [00:16<00:00,  1.05s/req]Processing requests: 100%|██████████| 33/33 [00:16<00:00,  2.05req/s]
✅ 33 requests successful, ❌ 0 failed
Saving 33 results to disk...
Generated/loaded 33 current results

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-04-13 13:54:26,793 - INFO - ============================================================
2026-04-13 13:54:26,793 - INFO - CONSISTENCY CHECK SUMMARY
2026-04-13 13:54:26,793 - INFO - ============================================================
2026-04-13 13:54:26,793 - INFO - Sequential tests average consistency rate: 97.73%
2026-04-13 13:54:26,793 - INFO - Concurrent tests average consistency rate: 75.00%
2026-04-13 13:54:26,793 - INFO - Overall average consistency rate: 86.36%
2026-04-13 13:54:26,793 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.18.0_vllm_sarashina_35b', 'server_api': 'http://localhost:8089/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|------|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.6667|±  |   N/A|

```

### 5. BFCL (Function Calling)
- **Status:** ❌ Not run
- **Results:** Check BFCL_PROJECT_ROOT: /Users/genyihuang/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_18_0_vllm_sarashina_35b_20260413_134705/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
BFCL results CSV file not found at: /Users/genyihuang/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_18_0_vllm_sarashina_35b_20260413_134705/bfcl_results/score/data_overall.csv
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Scoring batches:   0%|          | 0/9 [00:00<?, ?it/s]Scoring batches:  11%|█         | 1/9 [00:09<01:15,  9.43s/it]Scoring batches:  22%|██▏       | 2/9 [00:18<01:05,  9.43s/it]Scoring batches:  33%|███▎      | 3/9 [00:28<00:56,  9.46s/it]Scoring batches:  44%|████▍     | 4/9 [00:37<00:46,  9.32s/it]Scoring batches:  56%|█████▌    | 5/9 [00:46<00:37,  9.32s/it]Scoring batches:  67%|██████▋   | 6/9 [00:56<00:28,  9.54s/it]Scoring batches:  78%|███████▊  | 7/9 [01:05<00:18,  9.44s/it]Scoring batches:  89%|████████▉ | 8/9 [01:15<00:09,  9.33s/it]Scoring batches: 100%|██████████| 9/9 [01:22<00:00,  8.68s/it]Scoring batches: 100%|██████████| 9/9 [01:22<00:00,  9.15s/it]
Average BERTScore (F1): 81.57%
```

### 7. Image Number Check
- **Status:** ❌ Failed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_image_number_check.py", line 397, in <module>
    image_number_check()
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_image_number_check.py", line 394, in image_number_check
    raise Exception(error_msg)
Exception: 14 tests failed out of 14
```

### 8. Image Size Check
- **Status:** ❌ Failed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_image_size_check.py", line 264, in <module>
    image_size_check()
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_image_size_check.py", line 261, in image_size_check
    raise Exception(error_msg)
Exception: 8 tests failed out of 8
```

## Files Generated
