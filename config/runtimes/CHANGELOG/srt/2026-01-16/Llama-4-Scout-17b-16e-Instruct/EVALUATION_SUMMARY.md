# Comprehensive Evaluation Summary

**Timestamp:** Thu Jan 15 19:16:10 PST 2026
**Model:** meta-llama/Llama-4-Scout-17B-16E-Instruct
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-01-15 16:33:27,682 - ERROR - 'Test for logprobs length and non-English characters in logprobs tokens (issue #16838)' validation failures:
2026-01-15 16:33:27,682 - ERROR -   - All tokens are ASCII: validation failed
2026-01-15 16:33:27,682 - INFO - ============================================================
2026-01-15 16:33:27,682 - INFO - FEATURE TEST SUMMARY
2026-01-15 16:33:27,682 - INFO - ============================================================
2026-01-15 16:33:27,682 - INFO - Total tests: 4
2026-01-15 16:33:27,683 - INFO - Passed tests: 3
2026-01-15 16:33:27,683 - INFO - Failed tests: 1
2026-01-15 16:33:27,683 - INFO - Pass rate: 75.00%
2026-01-15 16:33:27,683 - INFO - Failed test cases:
2026-01-15 16:33:27,683 - INFO -   - Test for logprobs length and non-English characters in logprobs tokens (issue #16838)
2026-01-15 16:33:27,683 - INFO - ============================================================
2026-01-15 16:33:27,683 - ERROR - Error during testing: 1 tests failed out of 4
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
2026-01-15 16:42:04,103 - INFO - ============================================================
2026-01-15 16:42:04,104 - INFO - CONSISTENCY CHECK SUMMARY
2026-01-15 16:42:04,104 - INFO - ============================================================
2026-01-15 16:42:04,104 - INFO - Sequential tests average consistency rate: 98.18%
2026-01-15 16:42:04,104 - INFO - Concurrent tests average consistency rate: 89.27%
2026-01-15 16:42:04,104 - INFO - Overall average consistency rate: 93.73%
2026-01-15 16:42:04,104 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Scout-17B-16E-Instruct'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7546|±  |0.0038|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260115_163318/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,24.72%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [00:09<02:47,  9.87s/it]
Scoring batches:  11%|█         | 2/18 [00:18<02:30,  9.43s/it]
Scoring batches:  17%|█▋        | 3/18 [00:27<02:16,  9.10s/it]
Scoring batches:  22%|██▏       | 4/18 [00:35<02:02,  8.75s/it]
Scoring batches:  28%|██▊       | 5/18 [00:44<01:53,  8.73s/it]
Scoring batches:  33%|███▎      | 6/18 [00:54<01:49,  9.11s/it]
Scoring batches:  39%|███▉      | 7/18 [01:02<01:37,  8.88s/it]
Scoring batches:  44%|████▍     | 8/18 [01:11<01:28,  8.87s/it]
Scoring batches:  50%|█████     | 9/18 [01:18<01:14,  8.29s/it]
Scoring batches:  56%|█████▌    | 10/18 [01:27<01:06,  8.30s/it]
Scoring batches:  61%|██████    | 11/18 [01:36<01:00,  8.65s/it]
Scoring batches:  67%|██████▋   | 12/18 [01:42<00:46,  7.75s/it]
Scoring batches:  72%|███████▏  | 13/18 [01:50<00:39,  7.98s/it]
Scoring batches:  78%|███████▊  | 14/18 [01:58<00:32,  8.01s/it]
Scoring batches:  83%|████████▎ | 15/18 [02:07<00:24,  8.15s/it]
Scoring batches:  89%|████████▉ | 16/18 [02:15<00:16,  8.34s/it]
Scoring batches:  94%|█████████▍| 17/18 [02:24<00:08,  8.51s/it]
Scoring batches: 100%|██████████| 18/18 [02:26<00:00,  6.57s/it]
Scoring batches: 100%|██████████| 18/18 [02:26<00:00,  8.16s/it]
Average BERTScore (F1): 84.32%
```

### 7. Image Number Check
- **Status:** ✅ Completed (After adding `--limit-mm-data-per-request='{"image": 10}'`, the image number check failed when sending 20 images in the Image Number Check)
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2026-01-27 15:22:16,220 - INFO - Starting image number check with config: {'server-engine': 'SGLang', 'server-version': 'v0.5.7', 'server-api': 'http://localhost:8080/v1/chat/completions', 'model-name': 'meta-llama/Llama-4-Scout-17B-16E-Instruct', 'image-size': 1024, 'max-tokens': 128, 'num-images': [1, 2, 5, 10, 20], 'output-folder-path': '/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260127_152212/image_number_check'}
2026-01-27 15:22:29,078 - INFO - Testing with 1 images (size: 1024x1024)
2026-01-27 15:22:34,066 - INFO - Number of images 1: Success
2026-01-27 15:22:34,067 - INFO - Testing with 2 images (size: 1024x1024)
2026-01-27 15:22:38,593 - INFO - Number of images 2: Success
2026-01-27 15:22:38,593 - INFO - Testing with 5 images (size: 1024x1024)
2026-01-27 15:22:49,203 - INFO - Number of images 5: Success
2026-01-27 15:22:49,203 - INFO - Testing with 10 images (size: 1024x1024)
2026-01-27 15:23:05,909 - INFO - Number of images 10: Success
2026-01-27 15:23:05,909 - INFO - Testing with 20 images (size: 1024x1024)
2026-01-27 15:23:18,949 - INFO - Number of images 20: Failed - Error 400: {"object":"error","message":"Image count 20 exceeds limit 10 per request.","type":"BadRequestError","param":null,"code":400}
2026-01-27 15:23:24,045 - INFO - Testing with image saved as PNG but declared as PNG
2026-01-27 15:23:29,183 - INFO - Test PNG-as-PNG: Success
2026-01-27 15:23:29,184 - INFO - Testing with image saved as PNG but declared as JPEG
2026-01-27 15:23:31,798 - INFO - Test PNG-as-JPEG: Success
2026-01-27 15:23:31,798 - INFO - Testing with image saved as PNG but declared as WEBP
2026-01-27 15:23:34,575 - INFO - Test PNG-as-WEBP: Success
2026-01-27 15:23:34,576 - INFO - Testing with image saved as JPEG but declared as PNG
2026-01-27 15:23:37,472 - INFO - Test JPEG-as-PNG: Success
2026-01-27 15:23:37,472 - INFO - Testing with image saved as JPEG but declared as JPEG
2026-01-27 15:23:40,105 - INFO - Test JPEG-as-JPEG: Success
2026-01-27 15:23:40,106 - INFO - Testing with image saved as JPEG but declared as WEBP
2026-01-27 15:23:42,735 - INFO - Test JPEG-as-WEBP: Success
2026-01-27 15:23:42,735 - INFO - Testing with image saved as WEBP but declared as PNG
2026-01-27 15:23:45,158 - INFO - Test WEBP-as-PNG: Success
2026-01-27 15:23:45,158 - INFO - Testing with image saved as WEBP but declared as JPEG
2026-01-27 15:23:47,610 - INFO - Test WEBP-as-JPEG: Success
2026-01-27 15:23:47,610 - INFO - Testing with image saved as WEBP but declared as WEBP
2026-01-27 15:23:50,068 - INFO - Test WEBP-as-WEBP: Success
2026-01-27 15:23:50,094 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260127_152212/image_number_check/image_number_check_SGLang_v0.5.7.csv
2026-01-27 15:23:50,095 - ERROR - 1 tests failed out of 14
Traceback (most recent call last):
  File "/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/openai_image_number_check.py", line 397, in <module>
    image_number_check()
  File "/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/openai_image_number_check.py", line 394, in image_number_check
    raise Exception(error_msg)
Exception: 1 tests failed out of 14
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2026-01-15 19:15:17,529 - INFO - Testing image dimensions: 4097x4097
2026-01-15 19:15:37,117 - INFO - Dimensions 4097x4097: Success
2026-01-15 19:15:37,117 - INFO - Testing image dimensions: 8192x8192
2026-01-15 19:15:53,996 - INFO - Dimensions 8192x8192: Success
2026-01-15 19:15:54,000 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260115_163318/image_size_check/image_size_check_SGLang_v0.5.7.csv
```

## Files Generated
