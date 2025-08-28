# Comprehensive Evaluation Summary

**Timestamp:** Thu Aug  7 18:49:49 UTC 2025
**Model:** meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

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
    Max: 171
    Average: 44.0
    Median: 18
    Mode: 8

  Most common difference positions:
    Position 8: 6 times (22.2%)
    Position 18: 3 times (11.1%)
    Position 0: 3 times (11.1%)
    Position 3: 3 times (11.1%)
    Position 156: 2 times (7.4%)

  Difference types:
    token_mismatch: 27 (100.0%)

  Difference timing:
    Early differences (pos < 10): 12 (44.4%)
    Late differences (pos >= 10): 15 (55.6%)

============================================================
COMPARISON SUMMARY
============================================================
  Current Version: v0.4.10.post2
  Baseline Version: v0.4.6.post5.hotfix.9569323
  Total tests: 33
  Mismatches: 27
  Missing baseline: 0
  Detailed analysis saved to: /home/ubuntu/sanity_check_v2/evaluation_results_SGLang_v0_4_10_post2_20250807_175929/version_comparison.csv
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Results:
```
2025-08-07 17:59:40,066 - INFO - Checking consistency for {'server_engine': 'SGLang', 'server_version': 'v0.4.10.post2', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8'}
2025-08-07 17:59:40,066 - INFO - Running sequential requests...
2025-08-07 17:59:51,590 - INFO - Consistent results for 0th prompt 'In the early history of the Earth, the planet was formed from the solar nebula approximately 4.5 bil...': 20/20
2025-08-07 18:00:02,971 - INFO - Consistent results for 1th prompt 'Write a Python function to find the nth Fibonacci number. The function should be efficient and handl...': 20/20
2025-08-07 18:00:14,397 - INFO - Consistent results for 2th prompt 'asdkj l3@#lkj 29d8fj asd@#12 randomtext qpwoeiruty, 100%#^&* randomcharact3rs mixed in....': 20/20
2025-08-07 18:00:25,789 - INFO - Consistent results for 3th prompt '这是一个双语文本 with English and 中文 characters. Analyze the structure and provide a brief summary of both E...': 20/20
2025-08-07 18:00:37,201 - INFO - Consistent results for 4th prompt 'Discuss the pathophysiology of myocardial infarction, focusing on the role of atherosclerosis and th...': 20/20
2025-08-07 18:00:48,710 - INFO - Consistent results for 5th prompt 'Given the JSON data: {'employees': [{'firstName':'John', 'lastName':'Doe'}, {'firstName':'Anna', 'la...': 20/20
2025-08-07 18:01:00,105 - INFO - Consistent results for 6th prompt 'Prove the Pythagorean theorem using a geometric approach. Then, provide an algebraic proof and discu...': 20/20
2025-08-07 18:01:11,533 - INFO - Consistent results for 7th prompt 'Analyze the causes and consequences of the Fall of the Roman Empire, focusing on economic, military,...': 20/20
2025-08-07 18:01:22,956 - INFO - Consistent results for 8th prompt 'Discuss the concept of free will versus determinism. Provide arguments from both perspectives and co...': 20/20
2025-08-07 18:01:34,414 - INFO - Consistent results for 9th prompt 'Write a short story set in a dystopian future where artificial intelligence governs the world, and h...': 20/20
2025-08-07 18:01:46,474 - INFO - Consistent results for 10th prompt ' Your primary goal is to provide factual, accurate, and helpful answers to user questions. You are e...': 20/20
2025-08-07 18:01:46,475 - INFO - Running concurrent requests...
2025-08-07 18:01:47,546 - INFO - Consistent results for 0th prompt 'In the early history of the Earth, the planet was formed from the solar nebula approximately 4.5 bil...': 93/100
2025-08-07 18:01:48,565 - INFO - Consistent results for 1th prompt 'Write a Python function to find the nth Fibonacci number. The function should be efficient and handl...': 100/100
2025-08-07 18:01:49,639 - INFO - Consistent results for 2th prompt 'asdkj l3@#lkj 29d8fj asd@#12 randomtext qpwoeiruty, 100%#^&* randomcharact3rs mixed in....': 60/100
2025-08-07 18:01:50,654 - INFO - Consistent results for 3th prompt '这是一个双语文本 with English and 中文 characters. Analyze the structure and provide a brief summary of both E...': 100/100
2025-08-07 18:01:51,729 - INFO - Consistent results for 4th prompt 'Discuss the pathophysiology of myocardial infarction, focusing on the role of atherosclerosis and th...': 100/100
2025-08-07 18:01:52,821 - INFO - Consistent results for 5th prompt 'Given the JSON data: {'employees': [{'firstName':'John', 'lastName':'Doe'}, {'firstName':'Anna', 'la...': 94/100
2025-08-07 18:01:53,863 - INFO - Consistent results for 6th prompt 'Prove the Pythagorean theorem using a geometric approach. Then, provide an algebraic proof and discu...': 100/100
2025-08-07 18:01:54,904 - INFO - Consistent results for 7th prompt 'Analyze the causes and consequences of the Fall of the Roman Empire, focusing on economic, military,...': 100/100
2025-08-07 18:01:55,943 - INFO - Consistent results for 8th prompt 'Discuss the concept of free will versus determinism. Provide arguments from both perspectives and co...': 98/100
2025-08-07 18:01:57,101 - INFO - Consistent results for 9th prompt 'Write a short story set in a dystopian future where artificial intelligence governs the world, and h...': 94/100
2025-08-07 18:01:58,330 - INFO - Consistent results for 10th prompt ' Your primary goal is to provide factual, accurate, and helpful answers to user questions. You are e...': 84/100
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.4.10.post2', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8'}      
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 24 lines):
```
| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.8108|±  |0.0035|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /home/ubuntu/sanity_check_v2/evaluation_results_SGLang_v0_4_10_post2_20250807_175929/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Evaluation Results (last 24 lines):
```
🦍 Model: meta-llama_Llama-4-Maverick-17B-128E-Instruct-FP8-FC
🔍 Running test: multi_turn_long_context
✅ Test completed: multi_turn_long_context. 🎯 Accuracy: 0.145
🔍 Running test: irrelevance
✅ Test completed: irrelevance. 🎯 Accuracy: 0.7041666666666667
🔍 Running test: simple
✅ Test completed: simple. 🎯 Accuracy: 0.9525
🔍 Running test: live_relevance
✅ Test completed: live_relevance. 🎯 Accuracy: 1.0
🔍 Running test: multi_turn_miss_func
✅ Test completed: multi_turn_miss_func. 🎯 Accuracy: 0.205
🔍 Running test: live_parallel
✅ Test completed: live_parallel. 🎯 Accuracy: 0.75
🔍 Running test: live_multiple
✅ Test completed: live_multiple. 🎯 Accuracy: 0.7473884140550807
🔍 Running test: live_parallel_multiple
✅ Test completed: live_parallel_multiple. 🎯 Accuracy: 0.6666666666666666
🔍 Running test: live_simple
✅ Test completed: live_simple. 🎯 Accuracy: 0.8372093023255814
🔍 Running test: live_irrelevance
✅ Test completed: live_irrelevance. 🎯 Accuracy: 0.31859410430839
🔍 Running test: multi_turn_base
✅ Test completed: multi_turn_base. 🎯 Accuracy: 0.235
🔍 Running test: javascript
✅ Test completed: javascript. 🎯 Accuracy: 0.72
🔍 Running test: multi_turn_miss_param
✅ Test completed: multi_turn_miss_param. 🎯 Accuracy: 0.14
🔍 Running test: multiple
✅ Test completed: multiple. 🎯 Accuracy: 0.93
🔍 Running test: java
✅ Test completed: java. 🎯 Accuracy: 0.62
🔍 Running test: parallel
✅ Test completed: parallel. 🎯 Accuracy: 0.895
🔍 Running test: parallel_multiple
✅ Test completed: parallel_multiple. 🎯 Accuracy: 0.885
🏁 Evaluation completed. See /home/ubuntu/sanity_check_v2/evaluation_results_SGLang_v0_4_10_post2_20250807_175929/bfcl_results/score/data_overall.csv for overall evaluation results on BFCL V3.
See /home/ubuntu/sanity_check_v2/evaluation_results_SGLang_v0_4_10_post2_20250807_175929/bfcl_results/score/data_live.csv, /home/ubuntu/sanity_check_v2/evaluation_results_SGLang_v0_4_10_post2_20250807_175929/bfcl_results/score/data_non_live.csv and /home/ubuntu/sanity_check_v2/evaluation_results_SGLang_v0_4_10_post2_20250807_175929/bfcl_results/score/data_multi_turn.csv for detailed evaluation results on each sub-section categories respectively.
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 10 lines):
```
Average BERTScore (F1): 84.15%
```
