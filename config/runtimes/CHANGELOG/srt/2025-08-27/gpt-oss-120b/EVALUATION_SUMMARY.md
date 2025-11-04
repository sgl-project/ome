# Comprehensive Evaluation Summary

**Timestamp:** Thu Oct 23 18:12:50 UTC 2025
**Model:** openai/gpt-oss-120b
**Server:** http://localhost:8091

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
2025-10-24 19:00:38,821 - INFO - Found non-ASCII token: ' 分'
2025-10-24 19:00:38,822 - INFO - 'Test for logprobs length and non-English characters in logprobs tokens (issue #16838)': All tokens are ASCII failed for response: {'id': '467f60bd06c6485bbe08017cf55441e9', 'object': 'chat.completion', 'created': 1761332438, 'model': 'openai/gpt-oss-120b', 'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': None, 'reasoning_content': 'User wants a short sentence about the', 'tool_calls': None}, 'logprobs': {'content': [{'token': '<|channel|>', 'bytes': [60, 124, 99, 104, 97, 110, 110, 101, 108, 124, 62], 'logprob': 0.0, 'top_logprobs': [{'token': '<|channel|>', 'bytes': [60, 124, 99, 104, 97, 110, 110, 101, 108, 124, 62], 'logprob': 0.0}, {'token': '<|constrain|>', 'bytes': [60, 124, 99, 111, 110, 115, 116, 114, 97, 105, 110, 124, 62], 'logprob': -28.571428298950195}, {'token': ' ', 'bytes': [32], 'logprob': -32.32142639160156}, {'token': ' (', 'bytes': [32, 40], 'logprob': -33.57142639160156}, {'token': ' 分', 'bytes': [32, 229, 136, 134], 'logprob': -35.0}]}, {'token': 'analysis', 'bytes': [97, 110, 97, 108, 121, 115, 105, 115], 'logprob': 0.0, 'top_logprobs': [{'token': 'analysis', 'bytes': [97, 110, 97, 108, 121, 115, 105, 115], 'logprob': 0.0}, {'token': ' analysis', 'bytes': [32, 97, 110, 97, 108, 121, 115, 105, 115], 'logprob': -37.5}, {'token': 'Analysis', 'bytes': [65, 110, 97, 108, 121, 115, 105, 115], 'logprob': -39.46428680419922}, {'token': '_analysis', 'bytes': [95, 97, 110, 97, 108, 121, 115, 105, 115], 'logprob': -40.71428680419922}, {'token': 'analyse', 'bytes': [97, 110, 97, 108, 121, 115, 101], 'logprob': -40.89285659790039}]}, {'token': '<|message|>', 'bytes': [60, 124, 109, 101, 115, 115, 97, 103, 101, 124, 62], 'logprob': 0.0, 'top_logprobs': [{'token': '<|message|>', 'bytes': [60, 124, 109, 101, 115, 115, 97, 103, 101, 124, 62], 'logprob': 0.0}, {'token': ':', 'bytes': [58], 'logprob': -25.535715103149414}, {'token': '<|start|>', 'bytes': [60, 124, 115, 116, 97, 114, 116, 124, 62], 'logprob': -25.892858505249023}, {'token': ':\n\n', 'bytes': [58, 10, 10], 'logprob': -26.785715103149414}, {'token': '<|constrain|>', 'bytes': [60, 124, 99, 111, 110, 115, 116, 114, 97, 105, 110, 124, 62], 'logprob': -27.58928680419922}]}, {'token': 'User', 'bytes': [85, 115, 101, 114], 'logprob': -0.425400048494339, 'top_logprobs': [{'token': 'User', 'bytes': [85, 115, 101, 114], 'logprob': -0.425400048494339}, {'token': 'The', 'bytes': [84, 104, 101], 'logprob': -1.1396868228912354}, {'token': 'We', 'bytes': [87, 101], 'logprob': -3.6396868228912354}, {'token': 'Need', 'bytes': [78, 101, 101, 100], 'logprob': -8.10396957397461}, {'token': 'They', 'bytes': [84, 104, 101, 121], 'logprob': -12.211112976074219}]}, {'token': ' wants', 'bytes': [32, 119, 97, 110, 116, 115], 'logprob': -0.5547356009483337, 'top_logprobs': [{'token': ' wants', 'bytes': [32, 119, 97, 110, 116, 115], 'logprob': -0.5547356009483337}, {'token': ' asks', 'bytes': [32, 97, 115, 107, 115], 'logprob': -0.9118790030479431}, {'token': ':', 'bytes': [58], 'logprob': -3.7690186500549316}, {'token': ' requests', 'bytes': [32, 114, 101, 113, 117, 101, 115, 116, 115], 'logprob': -7.697592258453369}, {'token': ' request', 'bytes': [32, 114, 101, 113, 117, 101, 115, 116], 'logprob': -8.590448379516602}]}, {'token': ' a', 'bytes': [32, 97], 'logprob': -0.004317710641771555, 'top_logprobs': [{'token': ' a', 'bytes': [32, 97], 'logprob': -0.004317710641771555}, {'token': ':', 'bytes': [58], 'logprob': -6.075744152069092}, {'token': ' "', 'bytes': [32, 34], 'logprob': -6.611461162567139}, {'token': ' short', 'bytes': [32, 115, 104, 111, 114, 116], 'logprob': -7.325744152069092}, {'token': ' one', 'bytes': [32, 111, 110, 101], 'logprob': -12.504318237304688}]}, {'token': ' short', 'bytes': [32, 115, 104, 111, 114, 116], 'logprob': -4.172333774477011e-06, 'top_logprobs': [{'token': ' short', 'bytes': [32, 115, 104, 111, 114, 116], 'logprob': -4.172333774477011e-06}, {'token': ' single', 'bytes': [32, 115, 105, 110, 103, 108, 101], 'logprob': -12.500003814697266}, {'token': ' "', 'bytes': [32, 34], 'logprob': -15.000003814697266}, {'token': ' sentence', 'bytes': [32, 115, 101, 110, 116, 101, 110, 99, 101], 'logprob': -15.714290618896484}, {'token': ' simple', 'bytes': [32, 115, 105, 109, 112, 108, 101], 'logprob': -18.035717010498047}]}, {'token': ' sentence', 'bytes': [32, 115, 101, 110, 116, 101, 110, 99, 101], 'logprob': 0.0, 'top_logprobs': [{'token': ' sentence', 'bytes': [32, 115, 101, 110, 116, 101, 110, 99, 101], 'logprob': 0.0}, {'token': ' statement', 'bytes': [32, 115, 116, 97, 116, 101, 109, 101, 110, 116], 'logprob': -20.714284896850586}, {'token': ' single', 'bytes': [32, 115, 105, 110, 103, 108, 101], 'logprob': -21.071428298950195}, {'token': ' (', 'bytes': [32, 40], 'logprob': -21.60714340209961}, {'token': '_sentence', 'bytes': [95, 115, 101, 110, 116, 101, 110, 99, 101], 'logprob': -21.785715103149414}]}, {'token': ' about', 'bytes': [32, 97, 98, 111, 117, 116], 'logprob': -1.5258905477821827e-05, 'top_logprobs': [{'token': ' about', 'bytes': [32, 97, 98, 111, 117, 116], 'logprob': -1.5258905477821827e-05}, {'token': ' describing', 'bytes': [32, 100, 101, 115, 99, 114, 105, 98, 105, 110, 103], 'logprob': -11.250015258789062}, {'token': '.', 'bytes': [46], 'logprob': -13.75001335144043}, {'token': ':', 'bytes': [58], 'logprob': -14.285728454589844}, {'token': ',', 'bytes': [44], 'logprob': -14.285728454589844}]}, {'token': ' the', 'bytes': [32, 116, 104, 101], 'logprob': -0.03574333339929581, 'top_logprobs': [{'token': ' the', 'bytes': [32, 116, 104, 101], 'logprob': -0.03574333339929581}, {'token': ' weather', 'bytes': [32, 119, 101, 97, 116, 104, 101, 114], 'logprob': -3.4285998344421387}, {'token': " today's", 'bytes': [32, 116, 111, 100, 97, 121, 39, 115], 'logprob': -5.928599834442139}, {'token': ' today', 'bytes': [32, 116, 111, 100, 97, 121], 'logprob': -11.107172012329102}, {'token': ' "', 'bytes': [32, 34], 'logprob': -13.250028610229492}]}]}, 'finish_reason': 'length', 'matched_stop': None}], 'usage': {'prompt_tokens': 76, 'total_tokens': 86, 'completion_tokens': 10, 'prompt_tokens_details': None, 'reasoning_tokens': 0}, 'metadata': {'weight_version': 'default'}}
2025-10-24 19:00:38,822 - ERROR - 'Test for logprobs length and non-English characters in logprobs tokens (issue #16838)' validation failures:
2025-10-24 19:00:38,822 - ERROR -   - All tokens are ASCII: validation failed
2025-10-24 19:00:38,822 - INFO - ============================================================
2025-10-24 19:00:38,822 - INFO - FEATURE TEST SUMMARY
2025-10-24 19:00:38,822 - INFO - ============================================================
2025-10-24 19:00:38,822 - INFO - Total tests: 4
2025-10-24 19:00:38,822 - INFO - Passed tests: 3
2025-10-24 19:00:38,822 - INFO - Failed tests: 1
2025-10-24 19:00:38,822 - INFO - Pass rate: 75.00%
2025-10-24 19:00:38,822 - INFO - Failed test cases:
2025-10-24 19:00:38,822 - INFO -   - Test for logprobs length and non-English characters in logprobs tokens (issue #16838)
2025-10-24 19:00:38,822 - INFO - ============================================================
2025-10-24 19:00:38,822 - ERROR - Error during testing: 1 tests failed out of 4

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
2025-10-23 18:12:50,107 - INFO - ============================================================
2025-10-23 18:12:50,108 - INFO - CONSISTENCY CHECK SUMMARY
2025-10-23 18:12:50,108 - INFO - ============================================================
2025-10-23 18:12:50,108 - INFO - Sequential tests average consistency rate: 98.64%
2025-10-23 18:12:50,108 - INFO - Concurrent tests average consistency rate: 100.00%
2025-10-23 18:12:50,108 - INFO - Overall average consistency rate: 99.32%
2025-10-23 18:12:50,108 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.3.post1', 'server_api': 'http://localhost:8091/v1/chat/completions', 'model_name': 'openai/gpt-oss-120b'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ❌ Not run
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```
Log file not found
```

### 5. BFCL (Function Calling)
- **Status:** ❌ Not run
- **Results:** Check BFCL_PROJECT_ROOT: /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251023_181057/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
BFCL results CSV file not found at: /home/chyyang/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251023_181057/bfcl_results/score/data_overall.csv
```

### 6. LooGLE (Long Document QA)
- **Status:** ❌ Not run
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
Log file not found
```

### 7. Image Number Check
- **Status:** ❌ Not run
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
Log file not found
```

### 8. Image Size Check
- **Status:** ❌ Not run
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
Log file not found
```

