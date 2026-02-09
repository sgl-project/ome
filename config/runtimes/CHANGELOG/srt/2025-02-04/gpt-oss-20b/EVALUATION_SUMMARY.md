# Comprehensive Evaluation Summary

**Timestamp:** Wed Feb 04 22:37:06 UTC 2026
**Model:** openai/gpt-oss-20b
**Server:** http://gpt-oss-20b-router.gpt-oss-20b.svc.cluster.local:8080

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
Exception: 1 tests failed out of 4
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://gpt-oss-20b-router.gpt-oss-20b.svc.cluster.local:8080/v1/chat/completions', 'model_name': 'openai/gpt-oss-20b'}
Traceback (most recent call last):
  File "/sanity_check/openai_chat_feature_tests.py", line 341, in <module>
    main()
  File "/sanity_check/openai_chat_feature_tests.py", line 331, in main
    raise Exception(
Exception: 1 tests failed out of 4
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
Error in making request: {"error":{"type":"Bad Request","code":"harmony_logprobs_not_supported","message":"logprobs are not supported for Harmony models"}}
✅ 0 requests successful, ❌ 33 failed
Failed requests:
  - Prompt 2, max_tokens=200
  - Prompt 3, max_tokens=100
  - Prompt 3, max_tokens=300
  - Prompt 1, max_tokens=300
  - Prompt 5, max_tokens=200
  - Prompt 1, max_tokens=200
  - Prompt 3, max_tokens=200
  - Prompt 2, max_tokens=300
  - Prompt 1, max_tokens=100
  - Prompt 4, max_tokens=200
  ... and 23 more
Generated/loaded 0 current results

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-02-04 20:07:59,868 - INFO - ============================================================
2026-02-04 20:07:59,868 - INFO - CONSISTENCY CHECK SUMMARY
2026-02-04 20:07:59,868 - INFO - ============================================================
2026-02-04 20:07:59,868 - INFO - Sequential tests average consistency rate: 79.55%
2026-02-04 20:07:59,868 - INFO - Concurrent tests average consistency rate: 77.82%
2026-02-04 20:07:59,868 - INFO - Overall average consistency rate: 78.68%
2026-02-04 20:07:59,868 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://gpt-oss-20b-router.gpt-oss-20b.svc.cluster.local:8080/v1/chat/completions', 'model_name': 'openai/gpt-oss-20b'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7486|±  |0.0038|

```

### 5. BFCL (Function Calling)
- **Status:** ❌ Failed
- **Results:** Check BFCL_PROJECT_ROOT: /sanity_check/evaluation_results_SGLang_v0_5_7_20260121_200400/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
BFCL results CSV file not found at: /sanity_check/evaluation_results_SGLang_v0_5_7_20260121_200400/bfcl_results/score/data_overall.csv
```

### 6. LooGLE (Long Document QA)
- **Status:** ❌ Failed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
AttributeError: 'NoneType' object has no attribute 'strip'
```

### 7. Image Number Check
- **Status:** ✅ Completed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2026-02-04 20:43:17,907 - INFO - Testing with image saved as WEBP but declared as JPEG
2026-02-04 20:43:18,644 - INFO - Test WEBP-as-JPEG: Success
2026-02-04 20:43:18,644 - INFO - Testing with image saved as WEBP but declared as WEBP
2026-02-04 20:43:19,379 - INFO - Test WEBP-as-WEBP: Success
2026-02-04 20:43:19,385 - INFO - Results saved to /sanity_check/evaluation_results_SGLang_v0_5_7_20260121_200400/image_number_check/image_number_check_SGLang_v0.5.7.csv
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2026-02-04 20:43:26,028 - INFO - Testing image dimensions: 4097x4097
2026-02-04 20:43:27,448 - INFO - Dimensions 4097x4097: Success
2026-02-04 20:43:27,448 - INFO - Testing image dimensions: 8192x8192
2026-02-04 20:43:29,034 - INFO - Dimensions 8192x8192: Success
2026-02-04 20:43:29,049 - INFO - Results saved to /sanity_check/evaluation_results_SGLang_v0_5_7_20260121_200400/image_size_check/image_size_check_SGLang_v0.5.7.csv
```

## Files Generated
- image_number_check/image_number_check_SGLang_v0.5.7.csv
- image_size_check.log
- lm_eval.log
- EVALUATION_SUMMARY.md
- image_size_check/image_size_check_SGLang_v0.5.7.csv
- lm_eval_results/openai__gpt-oss-20b/results_2026-02-04T20-34-00.413782.json
- loogle.log
- evaluation.log
- consistency_check.log