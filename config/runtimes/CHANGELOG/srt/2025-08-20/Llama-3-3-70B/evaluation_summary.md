# Comprehensive Evaluation Summary

**Timestamp:** Mon Aug 25 17:07:52 UTC 2025
**Model:** vllm-model
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2025-08-25 15:33:46,450 - INFO - ============================================================
2025-08-25 15:33:46,450 - INFO - Total tests: 4
2025-08-25 15:33:46,450 - INFO - Passed tests: 4
2025-08-25 15:33:46,450 - INFO - Failed tests: 0
2025-08-25 15:33:46,450 - INFO - Pass rate: 100.00%
2025-08-25 15:33:46,450 - INFO - All feature sanity checks completed successfully.
2025-08-25 15:33:46,450 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.4.10.586971d', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
    Position 0: 15 times (45.5%)
    Position 1: 6 times (18.2%)
    Position 16: 3 times (9.1%)
    Position 28: 3 times (9.1%)
    Position 17: 3 times (9.1%)

  Difference types:
    token_mismatch: 33 (100.0%)

  Difference timing:
    Early differences (pos < 10): 21 (63.6%)
    Late differences (pos >= 10): 12 (36.4%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.4.10.586971d
  Baseline Version: v0.6.4.post1.0c9082a1
  Total tests: 33
  Mismatches: 33
  Missing baseline: 0
  Detailed analysis saved to: /home/vasheno/sanity_check/evaluation_results_SGLang_v0_4_10_586971d_20250825_153344/version_comparison.csv

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2025-08-25 15:40:04,276 - INFO - ============================================================
2025-08-25 15:40:04,276 - INFO - CONSISTENCY CHECK SUMMARY
2025-08-25 15:40:04,276 - INFO - ============================================================
2025-08-25 15:40:04,276 - INFO - Sequential tests average consistency rate: 100.00%
2025-08-25 15:40:04,277 - INFO - Concurrent tests average consistency rate: 93.27%
2025-08-25 15:40:04,277 - INFO - Overall average consistency rate: 96.64%
2025-08-25 15:40:04,277 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.4.10.586971d', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'vllm-model'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value|   |Stderr|
|--------|------:|--------------|------|-----------|---|----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.711|±  | 0.004|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /home/vasheno/sanity_check/evaluation_results_SGLang_v0_4_10_586971d_20250825_153344/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,53.58%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]Scoring batches:   6%|▌         | 1/18 [00:05<01:32,  5.45s/it]Scoring batches:  11%|█         | 2/18 [00:10<01:22,  5.16s/it]Scoring batches:  17%|█▋        | 3/18 [00:15<01:17,  5.15s/it]Scoring batches:  22%|██▏       | 4/18 [00:21<01:14,  5.31s/it]Scoring batches:  28%|██▊       | 5/18 [00:26<01:11,  5.46s/it]Scoring batches:  33%|███▎      | 6/18 [00:33<01:11,  5.98s/it]Scoring batches:  39%|███▉      | 7/18 [00:40<01:07,  6.13s/it]Scoring batches:  44%|████▍     | 8/18 [00:44<00:54,  5.41s/it]Scoring batches:  50%|█████     | 9/18 [00:47<00:41,  4.65s/it]Scoring batches:  56%|█████▌    | 10/18 [00:49<00:31,  3.94s/it]Scoring batches:  61%|██████    | 11/18 [00:53<00:27,  3.99s/it]Scoring batches:  67%|██████▋   | 12/18 [00:57<00:22,  3.83s/it]Scoring batches:  72%|███████▏  | 13/18 [01:00<00:19,  3.85s/it]Scoring batches:  78%|███████▊  | 14/18 [01:05<00:15,  3.93s/it]Scoring batches:  83%|████████▎ | 15/18 [01:08<00:11,  3.92s/it]Scoring batches:  89%|████████▉ | 16/18 [01:12<00:07,  3.82s/it]Scoring batches:  94%|█████████▍| 17/18 [01:18<00:04,  4.54s/it]Scoring batches: 100%|██████████| 18/18 [01:19<00:00,  3.46s/it]Scoring batches: 100%|██████████| 18/18 [01:19<00:00,  4.43s/it]
Average BERTScore (F1): 84.88%
```

## Files Generated
- evaluation.log
- feature_tests.log
- bfcl_results/score/data_multi_turn.csv
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_long_context_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_irrelevance_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_simple_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_javascript_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_parallel_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_miss_param_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_miss_func_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_simple_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_parallel_multiple_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_relevance_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_parallel_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_multiple_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_base_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_parallel_multiple_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multiple_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_java_score.json
- bfcl_results/score/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_irrelevance_score.json
- bfcl_results/score/data_live.csv
- bfcl_results/score/data_non_live.csv
- bfcl_results/score/data_overall.csv
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_irrelevance_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_javascript_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_simple_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_long_context_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_java_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multiple_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_relevance_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_miss_param_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_base_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_parallel_multiple_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_parallel_multiple_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_simple_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_parallel_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_multi_turn_miss_func_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_parallel_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_irrelevance_result.json
- bfcl_results/result/meta-llama_Llama-3.3-70B-Instruct-FC/BFCL_v3_live_multiple_result.json
- lm_eval_results/vllm-model/results_2025-08-25T16-04-55.451458.json
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
- vllm-model_SGLang_v0_4_10_586971d/prompt_010_52053bfb_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_002_a5985e07_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_007_9151a846_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_000_4491cff4_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_003_1b24647b_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_001_72261cc0_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_006_f4dd7e8d_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_005_2ae6d653_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_009_90e12500_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_004_78fe0856_results.json
- vllm-model_SGLang_v0_4_10_586971d/prompt_008_82f83a61_results.json
- EVALUATION_SUMMARY.md
- consistency_check.log
- loogle.log
- lm_eval.log
- bfcl.log
- bfcl_eval.log
- version_comparison.csv
