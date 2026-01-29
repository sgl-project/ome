# Comprehensive Evaluation Summary

**Timestamp:** Thu Jan 15 11:33:35 PST 2026
**Model:** meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8
**Server:** http://localhost:8080

## Test Results

### 1. Feature Tests
- **Status:** ✅ Completed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
2026-01-15 09:48:38,242 - INFO - ============================================================
2026-01-15 09:48:38,242 - INFO - Total tests: 4
2026-01-15 09:48:38,242 - INFO - Passed tests: 4
2026-01-15 09:48:38,242 - INFO - Failed tests: 0
2026-01-15 09:48:38,242 - INFO - Pass rate: 100.00%
2026-01-15 09:48:38,242 - INFO - All feature sanity checks completed successfully.
2026-01-15 09:48:38,242 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8'}
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
2026-01-15 09:56:27,445 - INFO - ============================================================
2026-01-15 09:56:27,445 - INFO - CONSISTENCY CHECK SUMMARY
2026-01-15 09:56:27,445 - INFO - ============================================================
2026-01-15 09:56:27,446 - INFO - Sequential tests average consistency rate: 97.73%
2026-01-15 09:56:27,446 - INFO - Concurrent tests average consistency rate: 72.82%
2026-01-15 09:56:27,446 - INFO - Overall average consistency rate: 85.27%
2026-01-15 09:56:27,446 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.7', 'server_api': 'http://localhost:8080/v1/chat/completions', 'model_name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.8098|±  |0.0035|

```

### 5. BFCL (Function Calling)
- **Status:** ✅ Completed
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260115_094829/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
Rank,Overall Acc
1,31.93%
```

### 6. LooGLE (Long Document QA)
- **Status:** ✅ Completed
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```

Scoring batches:   0%|          | 0/18 [00:00<?, ?it/s]
Scoring batches:   6%|▌         | 1/18 [00:09<02:36,  9.23s/it]
Scoring batches:  11%|█         | 2/18 [00:18<02:24,  9.00s/it]
Scoring batches:  17%|█▋        | 3/18 [00:26<02:09,  8.64s/it]
Scoring batches:  22%|██▏       | 4/18 [00:34<02:01,  8.64s/it]
Scoring batches:  28%|██▊       | 5/18 [00:43<01:49,  8.46s/it]
Scoring batches:  33%|███▎      | 6/18 [00:52<01:47,  8.93s/it]
Scoring batches:  39%|███▉      | 7/18 [01:01<01:37,  8.90s/it]
Scoring batches:  44%|████▍     | 8/18 [01:09<01:26,  8.67s/it]
Scoring batches:  50%|█████     | 9/18 [01:18<01:18,  8.70s/it]
Scoring batches:  56%|█████▌    | 10/18 [01:26<01:08,  8.55s/it]
Scoring batches:  61%|██████    | 11/18 [01:36<01:02,  8.88s/it]
Scoring batches:  67%|██████▋   | 12/18 [01:43<00:49,  8.24s/it]
Scoring batches:  72%|███████▏  | 13/18 [01:51<00:41,  8.36s/it]
Scoring batches:  78%|███████▊  | 14/18 [02:00<00:33,  8.41s/it]
Scoring batches:  83%|████████▎ | 15/18 [02:09<00:25,  8.47s/it]
Scoring batches:  89%|████████▉ | 16/18 [02:17<00:17,  8.51s/it]
Scoring batches:  94%|█████████▍| 17/18 [02:26<00:08,  8.58s/it]
Scoring batches: 100%|██████████| 18/18 [02:27<00:00,  6.44s/it]
Scoring batches: 100%|██████████| 18/18 [02:27<00:00,  8.21s/it]
Average BERTScore (F1): 84.13%
```

### 7. Image Number Check
- **Status:** ✅ Completed (After adding `--limit-mm-data-per-request='{"image": 10}'`, the image number check failed when sending 20 images in the Image Number Check)
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2026-01-27 16:45:08,638 - INFO - Starting image number check with config: {'server-engine': 'SGLang', 'server-version': 'v0.5.7', 'server-api': 'http://localhost:8080/v1/chat/completions', 'model-name': 'meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8', 'image-size': 1024, 'max-tokens': 128, 'num-images': [1, 2, 5, 10, 20], 'output-folder-path': '/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260127_164505/image_number_check'}
2026-01-27 16:45:16,853 - INFO - Testing with 1 images (size: 1024x1024)
2026-01-27 16:45:18,924 - INFO - Number of images 1: Success
2026-01-27 16:45:18,925 - INFO - Testing with 2 images (size: 1024x1024)
2026-01-27 16:45:22,152 - INFO - Number of images 2: Success
2026-01-27 16:45:22,152 - INFO - Testing with 5 images (size: 1024x1024)
2026-01-27 16:45:29,732 - INFO - Number of images 5: Success
2026-01-27 16:45:29,733 - INFO - Testing with 10 images (size: 1024x1024)
2026-01-27 16:45:42,737 - INFO - Number of images 10: Success
2026-01-27 16:45:42,738 - INFO - Testing with 20 images (size: 1024x1024)
2026-01-27 16:46:06,759 - INFO - Number of images 20: Failed - Error 400: {"object":"error","message":"Image count 20 exceeds limit 10 per request.","type":"BadRequestError","param":null,"code":400}
2026-01-27 16:46:11,750 - INFO - Testing with image saved as PNG but declared as PNG
2026-01-27 16:46:14,376 - INFO - Test PNG-as-PNG: Success
2026-01-27 16:46:14,377 - INFO - Testing with image saved as PNG but declared as JPEG
2026-01-27 16:46:17,449 - INFO - Test PNG-as-JPEG: Success
2026-01-27 16:46:17,449 - INFO - Testing with image saved as PNG but declared as WEBP
2026-01-27 16:46:20,316 - INFO - Test PNG-as-WEBP: Success
2026-01-27 16:46:20,316 - INFO - Testing with image saved as JPEG but declared as PNG
2026-01-27 16:46:22,877 - INFO - Test JPEG-as-PNG: Success
2026-01-27 16:46:22,877 - INFO - Testing with image saved as JPEG but declared as JPEG
2026-01-27 16:46:25,362 - INFO - Test JPEG-as-JPEG: Success
2026-01-27 16:46:25,362 - INFO - Testing with image saved as JPEG but declared as WEBP
2026-01-27 16:46:27,791 - INFO - Test JPEG-as-WEBP: Success
2026-01-27 16:46:27,791 - INFO - Testing with image saved as WEBP but declared as PNG
2026-01-27 16:46:30,363 - INFO - Test WEBP-as-PNG: Success
2026-01-27 16:46:30,363 - INFO - Testing with image saved as WEBP but declared as JPEG
2026-01-27 16:46:32,808 - INFO - Test WEBP-as-JPEG: Success
2026-01-27 16:46:32,808 - INFO - Testing with image saved as WEBP but declared as WEBP
2026-01-27 16:46:35,265 - INFO - Test WEBP-as-WEBP: Success
2026-01-27 16:46:35,281 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260127_164505/image_number_check/image_number_check_SGLang_v0.5.7.csv
2026-01-27 16:46:35,282 - ERROR - 1 tests failed out of 14
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
2026-01-15 11:32:19,035 - INFO - Testing image dimensions: 4097x4097
2026-01-15 11:32:22,099 - INFO - Dimensions 4097x4097: Success
2026-01-15 11:32:22,099 - INFO - Testing image dimensions: 8192x8192
2026-01-15 11:33:15,192 - INFO - Dimensions 8192x8192: Success
2026-01-15 11:33:15,198 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/evaluation_results_SGLang_v0_5_7_20260115_094829/image_size_check/image_size_check_SGLang_v0.5.7.csv
```

## Files Generated
