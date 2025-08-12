# Comprehensive Evaluation Summary

**Timestamp:** Mon Aug 11 20:46:35 UTC 2025
**Model:** meta-llama/Llama-4-Scout-17B-16E-Instruct
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

### Feature Test Details:
```
2025-08-11 19:23:20,823 - INFO - 'Multi-turn tool-use': Response contains '1,000' failed for response: {'id': '8f0105fefea34246bedffdfac91f8c1a', 'object': 'chat.completion', 'created': 1754940200, 'model': 'meta-llama/Llama-4-Scout-17B-16E-Instruct', 'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': None, 'reasoning_content': None, 'tool_calls': [{'id': 'call_c221e6aeca644fde9d874bac', 'index': None, 'type': 'function', 'function': {'name': 'sales_search', 'arguments': '{"firstName": "John"}'}}]}, 'logprobs': None, 'finish_reason': 'tool_calls', 'matched_stop': None}], 'usage': {'prompt_tokens': 721, 'total_tokens': 730, 'completion_tokens': 9, 'prompt_tokens_details': None}}
```

### Conclusion
This is ok for Llama-4-scout as it is known not to be good at multi-turn tool evaluation.

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
📊 TOKEN DIFFERENCE ANALYSIS
──────────────────────────────────────────────────
  First differing token positions:
    Min: 9
    Max: 178
    Average: 46.5
    Median: 22
    Mode: 14

  Most common difference positions:
    Position 14: 3 times (13.0%)
    Position 68: 3 times (13.0%)
    Position 22: 3 times (13.0%)
    Position 12: 2 times (8.7%)
    Position 15: 2 times (8.7%)

  Difference types:
    token_mismatch: 23 (100.0%)

  Difference timing:
    Early differences (pos < 10): 1 (4.3%)
    Late differences (pos >= 10): 22 (95.7%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.4.10.post2
  Baseline Version: v0.4.6.post5
  Total tests: 33
  Mismatches: 23
  Missing baseline: 0
  Detailed analysis saved to: /home/ubuntu/sanity_check/openai/evaluation_results_SGLang_v0_4_10_post2_20250811_192318/version_comparison.csv

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 24 lines):
```
| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7542|±  |0.0038|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /home/ubuntu/sanity_check/openai/evaluation_results_SGLang_v0_4_10_post2_20250811_192318/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Evaluation Results (last 24 lines):
```
🦍 Model: meta-llama_Llama-4-Scout-17B-16E-Instruct-FC
🔍 Running test: multi_turn_long_context
✅ Test completed: multi_turn_long_context. 🎯 Accuracy: 0.06
🔍 Running test: irrelevance
✅ Test completed: irrelevance. 🎯 Accuracy: 0.575
🔍 Running test: simple
✅ Test completed: simple. 🎯 Accuracy: 0.945
🔍 Running test: live_relevance
✅ Test completed: live_relevance. 🎯 Accuracy: 0.9444444444444444
🔍 Running test: multi_turn_miss_func
✅ Test completed: multi_turn_miss_func. 🎯 Accuracy: 0.05
🔍 Running test: live_parallel
✅ Test completed: live_parallel. 🎯 Accuracy: 0.75
🔍 Running test: live_multiple
✅ Test completed: live_multiple. 🎯 Accuracy: 0.7454890788224121
🔍 Running test: live_parallel_multiple
✅ Test completed: live_parallel_multiple. 🎯 Accuracy: 0.6666666666666666
🔍 Running test: live_simple
✅ Test completed: live_simple. 🎯 Accuracy: 0.8062015503875969
🔍 Running test: live_irrelevance
✅ Test completed: live_irrelevance. 🎯 Accuracy: 0.31859410430839
🔍 Running test: multi_turn_base
✅ Test completed: multi_turn_base. 🎯 Accuracy: 0.035
🔍 Running test: javascript
✅ Test completed: javascript. 🎯 Accuracy: 0.8
🔍 Running test: multi_turn_miss_param
✅ Test completed: multi_turn_miss_param. 🎯 Accuracy: 0.04
🔍 Running test: multiple
✅ Test completed: multiple. 🎯 Accuracy: 0.955
🔍 Running test: java
✅ Test completed: java. 🎯 Accuracy: 0.62
🔍 Running test: parallel
✅ Test completed: parallel. 🎯 Accuracy: 0.8
🔍 Running test: parallel_multiple
✅ Test completed: parallel_multiple. 🎯 Accuracy: 0.795
📈 Aggregating data to generate leaderboard score table...
🏁 Evaluation completed. See /home/ubuntu/sanity_check/openai/evaluation_results_SGLang_v0_4_10_post2_20250811_192318/bfcl_results/score/data_overall.csv for overall evaluation results on BFCL V3.
See /home/ubuntu/sanity_check/openai/evaluation_results_SGLang_v0_4_10_post2_20250811_192318/bfcl_results/score/data_live.csv, /home/ubuntu/sanity_check/openai/evaluation_results_SGLang_v0_4_10_post2_20250811_192318/bfcl_results/score/data_non_live.csv and /home/ubuntu/sanity_check/openai/evaluation_results_SGLang_v0_4_10_post2_20250811_192318/bfcl_results/score/data_multi_turn.csv for detailed evaluation results on each sub-section categories respectively.
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 10 lines):
```
Average BERTScore (F1): 84.07%
```
