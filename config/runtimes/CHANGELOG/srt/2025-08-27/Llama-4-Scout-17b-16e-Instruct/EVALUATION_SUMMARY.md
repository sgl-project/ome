# Comprehensive Evaluation Summary

**Timestamp:** Fri Oct 24 22:02:39 UTC 2025
**Model:** meta-llama/Llama-4-Scout-17B-16E-Instruct
**Server:** http://localhost:8090

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2025-10-24 21:59:33,272 - INFO - Found non-ASCII token: ' модель'
2025-10-24 21:59:33,272 - INFO - 'Test for logprobs length and non-English characters in logprobs tokens (issue #16838)': All tokens are ASCII failed for response: {'id': 'cd24df1948934d24b381d5948146fda7', 'object': 'chat.completion', 'created': 1761343173, 'model': 'meta-llama/Llama-4-Scout-17B-16E-Instruct', 'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': "I'm a large language model, I don't have have", 'reasoning_content': None, 'tool_calls': None}, 'logprobs': {'content': [{'token': "I'm", 'bytes': [73, 39, 109], 'logprob': -0.2889370918273926, 'top_logprobs': [{'token': "I'm", 'bytes': [73, 39, 109], 'logprob': -0.2889370918273926}, {'token': 'The', 'bytes': [84, 104, 101], 'logprob': -1.71751070022583}, {'token': 'Today', 'bytes': [84, 111, 100, 97, 121], 'logprob': -2.7889370918273926}, {'token': 'There', 'bytes': [84, 104, 101, 114, 101], 'logprob': -4.931797504425049}, {'token': 'What', 'bytes': [87, 104, 97, 116], 'logprob': -6.71751070022583}]}, {'token': ' a', 'bytes': [32, 97], 'logprob': -0.05648994445800781, 'top_logprobs': [{'token': ' a', 'bytes': [32, 97], 'logprob': -0.05648994445800781}, {'token': ' not', 'bytes': [32, 110, 111, 116], 'logprob': -2.9136295318603516}, {'token': ' afraid', 'bytes': [32, 97, 102, 114, 97, 105, 100], 'logprob': -8.270776748657227}, {'token': ' happy', 'bytes': [32, 104, 97, 112, 112, 121], 'logprob': -8.985063552856445}, {'token': ' an', 'bytes': [32, 97, 110], 'logprob': -8.985063552856445}]}, {'token': ' large', 'bytes': [32, 108, 97, 114, 103, 101], 'logprob': -5.960466182841628e-07, 'top_logprobs': [{'token': ' large', 'bytes': [32, 108, 97, 114, 103, 101], 'logprob': -5.960466182841628e-07}, {'token': ' text', 'bytes': [32, 116, 101, 120, 116], 'logprob': -14.285714149475098}, {'token': ' language', 'bytes': [32, 108, 97, 110, 103, 117, 97, 103, 101], 'logprob': -17.857139587402344}, {'token': ' chatbot', 'bytes': [32, 99, 104, 97, 116, 98, 111, 116], 'logprob': -18.928573608398438}, {'token': ' computer', 'bytes': [32, 99, 111, 109, 112, 117, 116, 101, 114], 'logprob': -18.928573608398438}]}, {'token': ' language', 'bytes': [32, 108, 97, 110, 103, 117, 97, 103, 101], 'logprob': 0.0, 'top_logprobs': [{'token': ' language', 'bytes': [32, 108, 97, 110, 103, 117, 97, 103, 101], 'logprob': 0.0}, {'token': '-language', 'bytes': [45, 108, 97, 110, 103, 117, 97, 103, 101], 'logprob': -22.500003814697266}, {'token': '-scale', 'bytes': [45, 115, 99, 97, 108, 101], 'logprob': -23.21428680419922}, {'token': ' model', 'bytes': [32, 109, 111, 100, 101, 108], 'logprob': -24.285717010498047}, {'token': ' Language', 'bytes': [32, 76, 97, 110, 103, 117, 97, 103, 101], 'logprob': -24.642860412597656}]}, {'token': ' model', 'bytes': [32, 109, 111, 100, 101, 108], 'logprob': 0.0, 'top_logprobs': [{'token': ' model', 'bytes': [32, 109, 111, 100, 101, 108], 'logprob': 0.0}, {'token': ' модель', 'bytes': [32, 208, 188, 208, 190, 208, 180, 208, 181, 208, 187, 209, 140], 'logprob': -31.428573608398438}, {'token': ' ', 'bytes': [32], 'logprob': -31.428573608398438}, {'token': ',', 'bytes': [44], 'logprob': -31.785717010498047}, {'token': ' models', 'bytes': [32, 109, 111, 100, 101, 108, 115], 'logprob': -32.142860412597656}]}, {'token': ',', 'bytes': [44], 'logprob': -1.0848104466276709e-05, 'top_logprobs': [{'token': ',', 'bytes': [44], 'logprob': -1.0848104466276709e-05}, {'token': ' I', 'bytes': [32, 73], 'logprob': -11.428584098815918}, {'token': ' and', 'bytes': [32, 97, 110, 100], 'logprob': -21.071441650390625}, {'token': ';', 'bytes': [59], 'logprob': -22.500011444091797}, {'token': ' based', 'bytes': [32, 98, 97, 115, 101, 100], 'logprob': -23.214298248291016}]}, {'token': ' I', 'bytes': [32, 73], 'logprob': 0.0, 'top_logprobs': [{'token': ' I', 'bytes': [32, 73], 'logprob': 0.0}, {'token': ' so', 'bytes': [32, 115, 111], 'logprob': -19.642852783203125}, {'token': " I'm", 'bytes': [32, 73, 39, 109], 'logprob': -22.142852783203125}, {'token': ' it', 'bytes': [32, 105, 116], 'logprob': -22.857139587402344}, {'token': ' ', 'bytes': [32], 'logprob': -24.28571319580078}]}, {'token': " don't", 'bytes': [32, 100, 111, 110, 39, 116], 'logprob': -1.0848104466276709e-05, 'top_logprobs': [{'token': " don't", 'bytes': [32, 100, 111, 110, 39, 116], 'logprob': -1.0848104466276709e-05}, {'token': ' do', 'bytes': [32, 100, 111], 'logprob': -11.428576469421387}, {'token': ' have', 'bytes': [32, 104, 97, 118, 101], 'logprob': -27.500011444091797}, {'token': ' am', 'bytes': [32, 97, 109], 'logprob': -27.85715103149414}, {'token': ' don', 'bytes': [32, 100, 111, 110], 'logprob': -31.428577423095703}]}, {'token': ' have', 'bytes': [32, 104, 97, 118, 101], 'logprob': 0.0, 'top_logprobs': [{'token': ' have', 'bytes': [32, 104, 97, 118, 101], 'logprob': 0.0}, {'token': ' experience', 'bytes': [32, 101, 120, 112, 101, 114, 105, 101, 110, 99, 101], 'logprob': -30.0}, {'token': ' know', 'bytes': [32, 107, 110, 111, 119], 'logprob': -30.0}, {'token': ' ', 'bytes': [32], 'logprob': -31.428573608398438}, {'token': ' always', 'bytes': [32, 97, 108, 119, 97, 121, 115], 'logprob': -32.142860412597656}]}, {'token': ' have', 'bytes': [32, 104, 97, 118, 101], 'logprob': -0.04106971621513367, 'top_logprobs': [{'token': ' have', 'bytes': [32, 104, 97, 118, 101], 'logprob': -0.04106971621513367}, {'token': ' real', 'bytes': [32, 114, 101, 97, 108], 'logprob': -3.61250376701355}, {'token': ' access', 'bytes': [32, 97, 99, 99, 101, 115, 115], 'logprob': -4.3267903327941895}, {'token': ' the', 'bytes': [32, 116, 104, 101], 'logprob': -10.041069984436035}, {'token': ' direct', 'bytes': [32, 100, 105, 114, 101, 99, 116], 'logprob': -17.183929443359375}]}]}, 'finish_reason': 'length', 'matched_stop': None}], 'usage': {'prompt_tokens': 20, 'total_tokens': 30, 'completion_tokens': 10, 'prompt_tokens_details': None, 'reasoning_tokens': 0}, 'metadata': {'weight_version': 'default'}}
2025-10-24 21:59:33,272 - ERROR - 'Test for logprobs length and non-English characters in logprobs tokens (issue #16838)' validation failures:
2025-10-24 21:59:33,272 - ERROR -   - All tokens are ASCII: validation failed
2025-10-24 21:59:33,272 - INFO - ============================================================
2025-10-24 21:59:33,272 - INFO - FEATURE TEST SUMMARY
2025-10-24 21:59:33,272 - INFO - ============================================================
2025-10-24 21:59:33,272 - INFO - Total tests: 4
2025-10-24 21:59:33,272 - INFO - Passed tests: 3
2025-10-24 21:59:33,272 - INFO - Failed tests: 1
2025-10-24 21:59:33,272 - INFO - Pass rate: 75.00%
2025-10-24 21:59:33,272 - INFO - Failed test cases:
2025-10-24 21:59:33,272 - INFO -   - Test for logprobs length and non-English characters in logprobs tokens (issue #16838)
2025-10-24 21:59:33,272 - INFO - ============================================================
2025-10-24 21:59:33,272 - ERROR - Error during testing: 1 tests failed out of 4
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
2025-10-24 22:02:39,773 - INFO - ============================================================
2025-10-24 22:02:39,773 - INFO - CONSISTENCY CHECK SUMMARY
2025-10-24 22:02:39,773 - INFO - ============================================================
2025-10-24 22:02:39,773 - INFO - Sequential tests average consistency rate: 97.73%
2025-10-24 22:02:39,773 - INFO - Concurrent tests average consistency rate: 98.45%
2025-10-24 22:02:39,773 - INFO - Overall average consistency rate: 98.09%
2025-10-24 22:02:39,773 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.3.post1', 'server_api': 'http://localhost:8090/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Scout-17B-16E-Instruct'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7516|±  |0.0038|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251024_222958/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,47.58%
```

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [00:09<02:36,  9.20s/it]
Scoring batches:  11%|█         | 2/18 [00:17<02:14,  8.43s/it]
Scoring batches:  17%|█▋        | 3/18 [00:24<02:02,  8.14s/it]
Scoring batches:  22%|██▏       | 4/18 [00:31<01:47,  7.68s/it]
Scoring batches:  28%|██▊       | 5/18 [00:39<01:41,  7.82s/it]
Scoring batches:  33%|███▎      | 6/18 [00:48<01:36,  8.07s/it]
Scoring batches:  39%|███▉      | 7/18 [00:55<01:24,  7.67s/it]
Scoring batches:  44%|████▍     | 8/18 [01:02<01:14,  7.46s/it]
Scoring batches:  50%|█████     | 9/18 [01:09<01:05,  7.23s/it]
Scoring batches:  56%|█████▌    | 10/18 [01:15<00:56,  7.12s/it]
Scoring batches:  61%|██████    | 11/18 [01:23<00:50,  7.21s/it]
Scoring batches:  67%|██████▋   | 12/18 [01:29<00:40,  6.81s/it]
Scoring batches:  72%|███████▏  | 13/18 [01:36<00:34,  6.96s/it]
Scoring batches:  78%|███████▊  | 14/18 [01:43<00:28,  7.08s/it]
Scoring batches:  83%|████████▎ | 15/18 [01:51<00:21,  7.21s/it]
Scoring batches:  89%|████████▉ | 16/18 [01:59<00:14,  7.35s/it]
Scoring batches:  94%|█████████▍| 17/18 [02:06<00:07,  7.47s/it]
Scoring batches: 100%|██████████| 18/18 [02:09<00:00,  6.00s/it]
Scoring batches: 100%|██████████| 18/18 [02:09<00:00,  7.19s/it]
Average BERTScore (F1): 84.29%
```

### 7. Image Number Check
- **Status:** ✅ Completed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2025-10-24 19:33:20,819 - INFO - Testing with image saved as WEBP but declared as JPEG
2025-10-24 19:33:22,052 - INFO - Test WEBP-as-JPEG: Success
2025-10-24 19:33:22,053 - INFO - Testing with image saved as WEBP but declared as WEBP
2025-10-24 19:33:23,294 - INFO - Test WEBP-as-WEBP: Success
2025-10-24 19:33:23,300 - INFO - Results saved to /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251024_193230/image_number_check/image_number_check_SGLang_v0.5.3.post1.csv
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2025-10-24 19:31:47,612 - INFO - Testing image dimensions: 4097x4097
2025-10-24 19:31:55,509 - INFO - Dimensions 4097x4097: Success
2025-10-24 19:31:55,509 - INFO - Testing image dimensions: 8192x8192
2025-10-24 19:31:58,962 - INFO - Dimensions 8192x8192: Success
2025-10-24 19:31:58,986 - INFO - Results saved to /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251024_193136/image_size_check/image_size_check_SGLang_v0.5.3.post1.csv
```

