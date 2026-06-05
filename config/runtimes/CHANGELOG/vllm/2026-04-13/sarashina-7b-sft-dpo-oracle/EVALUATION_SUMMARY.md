# Comprehensive Evaluation Summary

**Timestamp:** Mon Apr 13 17:29:57 EDT 2026
**Model:** vllm-model
**Server:** http://localhost:8086

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
Exception: 3 tests failed out of 4
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.18.0_vllm_7b_sft_dpo_oracle', 'server_api': 'http://localhost:8086/v1/chat/completions', 'model_name': 'vllm-model'}
Traceback (most recent call last):
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_chat_feature_tests.py", line 341, in <module>
    main()
  File "/Users/genyihuang/moirai-internal/sanity_check/openai/openai_chat_feature_tests.py", line 331, in main
    raise Exception(
Exception: 3 tests failed out of 4
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
Current Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.18.0_vllm_7b_sft_dpo_oracle', 'server_api': 'http://localhost:8086/v1/chat/completions', 'model_name': 'vllm-model', 'max_tokens': [100, 200, 300]}
Detected mode: generate_current

=== Generating Current Results (v0.18.0_vllm_7b_sft_dpo_oracle) ===

Generating results for v0.18.0_vllm_7b_sft_dpo_oracle (vllm-model)
Force regenerate: False
Using 8 concurrent workers
Executing 33 API requests concurrently...
Processing requests:   0%|          | 0/33 [00:00<?, ?req/s]Processing requests:   3%|▎         | 1/33 [00:02<01:11,  2.24s/req]Processing requests:  12%|█▏        | 4/33 [00:02<00:17,  1.69req/s]Processing requests:  21%|██        | 7/33 [00:03<00:08,  3.10req/s]Processing requests:  27%|██▋       | 9/33 [00:03<00:07,  3.10req/s]Processing requests:  33%|███▎      | 11/33 [00:03<00:05,  4.08req/s]Processing requests:  36%|███▋      | 12/33 [00:04<00:04,  4.34req/s]Processing requests:  42%|████▏     | 14/33 [00:04<00:03,  5.97req/s]Processing requests:  48%|████▊     | 16/33 [00:04<00:03,  4.58req/s]Processing requests:  55%|█████▍    | 18/33 [00:05<00:02,  5.41req/s]Processing requests:  61%|██████    | 20/33 [00:05<00:02,  5.50req/s]Processing requests:  67%|██████▋   | 22/33 [00:05<00:02,  5.40req/s]Processing requests:  76%|███████▌  | 25/33 [00:06<00:01,  5.43req/s]Processing requests:  82%|████████▏ | 27/33 [00:06<00:00,  6.41req/s]Processing requests:  85%|████████▍ | 28/33 [00:06<00:00,  6.07req/s]Processing requests:  97%|█████████▋| 32/33 [00:07<00:00,  6.64req/s]Processing requests: 100%|██████████| 33/33 [00:07<00:00,  6.14req/s]Processing requests: 100%|██████████| 33/33 [00:07<00:00,  4.39req/s]
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
2026-04-13 17:22:57,909 - INFO - ============================================================
2026-04-13 17:22:57,909 - INFO - CONSISTENCY CHECK SUMMARY
2026-04-13 17:22:57,909 - INFO - ============================================================
2026-04-13 17:22:57,909 - INFO - Sequential tests average consistency rate: 99.09%
2026-04-13 17:22:57,909 - INFO - Concurrent tests average consistency rate: 86.73%
2026-04-13 17:22:57,909 - INFO - Overall average consistency rate: 92.91%
2026-04-13 17:22:57,909 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.18.0_vllm_7b_sft_dpo_oracle', 'server_api': 'http://localhost:8086/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|------|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.3889|±  |   N/A|

```

### 5. BFCL (Function Calling)
- **Status:** ❌ Not run
- **Results:** Check BFCL_PROJECT_ROOT: /Users/genyihuang/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_18_0_vllm_7b_sft_dpo_oracle_20260413_171915/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
BFCL results CSV file not found at: /Users/genyihuang/moirai-internal/sanity_check/openai/evaluation_results_vLLM_v0_18_0_vllm_7b_sft_dpo_oracle_20260413_171915/bfcl_results/score/data_overall.csv
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Scoring batches:   0%|          | 0/9 [00:00<?, ?it/s]Scoring batches:  11%|█         | 1/9 [00:12<01:37, 12.13s/it]Scoring batches:  22%|██▏       | 2/9 [00:22<01:15, 10.82s/it]Scoring batches:  33%|███▎      | 3/9 [00:31<01:01, 10.23s/it]Scoring batches:  44%|████▍     | 4/9 [00:40<00:49,  9.88s/it]Scoring batches:  56%|█████▌    | 5/9 [00:50<00:39,  9.96s/it]Scoring batches:  67%|██████▋   | 6/9 [01:00<00:29,  9.97s/it]Scoring batches:  78%|███████▊  | 7/9 [01:10<00:19,  9.90s/it]Scoring batches:  89%|████████▉ | 8/9 [01:20<00:09,  9.74s/it]Scoring batches: 100%|██████████| 9/9 [01:25<00:00,  8.25s/it]Scoring batches: 100%|██████████| 9/9 [01:25<00:00,  9.46s/it]
Average BERTScore (F1): 80.38%
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
