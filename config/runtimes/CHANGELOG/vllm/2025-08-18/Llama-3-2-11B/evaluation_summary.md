# Comprehensive Evaluation Summary

**Timestamp:** Thu Aug 21 10:24:13 UTC 2025
**Model:** vllm-model
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2025-08-21 08:51:03,057 - INFO - ============================================================
2025-08-21 08:51:03,057 - INFO - Total tests: 4
2025-08-21 08:51:03,057 - INFO - Passed tests: 4
2025-08-21 08:51:03,057 - INFO - Failed tests: 0
2025-08-21 08:51:03,057 - INFO - Pass rate: 100.00%
2025-08-21 08:51:03,057 - INFO - All feature sanity checks completed successfully.
2025-08-21 08:51:03,057 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.9.2.881ad1a', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
    Position 17: 3 times (17.6%)
    Position 89: 3 times (17.6%)
    Position 19: 3 times (17.6%)
    Position 16: 2 times (11.8%)
    Position 105: 2 times (11.8%)

  Difference types:
    token_mismatch: 17 (100.0%)

  Difference timing:
    Early differences (pos < 10): 0 (0.0%)
    Late differences (pos >= 10): 17 (100.0%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.9.2.881ad1a
  Baseline Version: v0.6.4.post1.0c9082a1
  Total tests: 33
  Mismatches: 17
  Missing baseline: 0
  Detailed analysis saved to: /home/vasheno/sanity_check/evaluation_results_vLLM_v0_9_2_881ad1a_20250821_085101/version_comparison.csv

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2025-08-21 08:55:48,140 - INFO - ============================================================
2025-08-21 08:55:48,140 - INFO - CONSISTENCY CHECK SUMMARY
2025-08-21 08:55:48,140 - INFO - ============================================================
2025-08-21 08:55:48,140 - INFO - Sequential tests average consistency rate: 100.00%
2025-08-21 08:55:48,140 - INFO - Concurrent tests average consistency rate: 97.27%
2025-08-21 08:55:48,140 - INFO - Overall average consistency rate: 98.64%
2025-08-21 08:55:48,140 - INFO - ============================================================
Configuration: {'server_engine': 'vLLM', 'server_version': 'v0.9.2.881ad1a', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value|   |Stderr|
|--------|------:|--------------|------|-----------|---|----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.463|±  |0.0044|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /home/vasheno/sanity_check/evaluation_results_vLLM_v0_9_2_881ad1a_20250821_085101/bfcl_results
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
Scoring batches: 100%|██████████| 18/18 [02:02<00:00,  6.82s/it]
Average BERTScore (F1): 84.20%
```

## Files Generated
- evaluation.log
- feature_tests.log
- bfcl_results/score/data_multi_turn.csv
- bfcl_results/score/data_live.csv
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_long_context_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_irrelevance_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_simple_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_javascript_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_parallel_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_miss_param_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_miss_func_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_simple_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_parallel_multiple_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_relevance_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_parallel_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_multiple_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_base_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_parallel_multiple_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multiple_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_java_score.json
- bfcl_results/score/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_irrelevance_score.json
- bfcl_results/score/data_non_live.csv
- bfcl_results/score/data_overall.csv
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_irrelevance_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_javascript_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_simple_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_long_context_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_java_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multiple_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_relevance_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_miss_param_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_base_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_parallel_multiple_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_parallel_multiple_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_simple_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_parallel_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_multi_turn_miss_func_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_parallel_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_irrelevance_result.json
- bfcl_results/result/meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC/BFCL_v3_live_multiple_result.json
- lm_eval_results/vllm-model/results_2025-08-21T09-53-21.621765.json
- version_comparison.log
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_010_52053bfb_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_002_a5985e07_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_007_9151a846_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_000_4491cff4_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_003_1b24647b_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_001_72261cc0_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_006_f4dd7e8d_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_005_2ae6d653_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_009_90e12500_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_004_78fe0856_results.json
- vllm-model_vLLM_v0_6_4_post1_0c9082a1/prompt_008_82f83a61_results.json
- EVALUATION_SUMMARY.md
- consistency_check.log
- loogle.log
- lm_eval.log
- bfcl.log
- bfcl_eval.log
- version_comparison.csv
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_010_52053bfb_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_002_a5985e07_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_007_9151a846_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_000_4491cff4_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_003_1b24647b_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_001_72261cc0_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_006_f4dd7e8d_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_005_2ae6d653_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_009_90e12500_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_004_78fe0856_results.json
- vllm-model_vLLM_v0_9_2_881ad1a/prompt_008_82f83a61_results.json