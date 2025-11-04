# Comprehensive Evaluation Summary

**Timestamp:** Wed Oct 22 21:42:47 UTC 2025
**Model:** openai/gpt-oss-20b
**Server:** http://localhost:8091

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2025-10-22 21:42:47,293 - INFO - ============================================================
2025-10-22 21:42:47,293 - INFO - Total tests: 4
2025-10-22 21:42:47,293 - INFO - Passed tests: 4
2025-10-22 21:42:47,293 - INFO - Failed tests: 0
2025-10-22 21:42:47,293 - INFO - Pass rate: 100.00%
2025-10-22 21:42:47,293 - INFO - All feature sanity checks completed successfully.
2025-10-22 21:42:47,293 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.3.post1', 'server_api': 'http://localhost:8091/v1/chat/completions', 'model_name': 'openai/gpt-oss-20b'}
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
2025-10-22 21:56:25,559 - INFO - ============================================================
2025-10-22 21:56:25,559 - INFO - CONSISTENCY CHECK SUMMARY
2025-10-22 21:56:25,559 - INFO - ============================================================
2025-10-22 21:56:25,559 - INFO - Sequential tests average consistency rate: 100.00%
2025-10-22 21:56:25,559 - INFO - Concurrent tests average consistency rate: 100.00%
2025-10-22 21:56:25,559 - INFO - Overall average consistency rate: 100.00%
2025-10-22 21:56:25,559 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.3.post1', 'server_api': 'http://localhost:8091/v1/chat/completions', 'model_name': 'openai/gpt-oss-20b'}
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
- **Results:** Check BFCL_PROJECT_ROOT: /home/chyyang/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251022_214241/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
BFCL results CSV file not found at: /home/chyyang/sanity_check/openai/evaluation_results_SGLang_v0_5_3_post1_20251022_214241/bfcl_results/score/data_overall.csv
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

