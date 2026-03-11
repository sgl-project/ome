# Comprehensive Evaluation Summary

**Timestamp:** Sat Mar  7 23:58:30 PST 2026
**Model:** gpt-oss-120b
**Server:** http://localhost:8084

## Test Results

### 1. Feature Tests
- **Status:** ❌ Failed
- **Log:** feature_tests.log

#### Feature Test Summary (last 10 lines):
```
Exception: 1 tests failed out of 4
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.9-smg-b367912d', 'server_api': 'http://localhost:8084/v1/chat/completions', 'model_name': 'gpt-oss-120b'}
Traceback (most recent call last):
  File "/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/openai_chat_feature_tests.py", line 341, in <module>
    main()
  File "/Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/openai_chat_feature_tests.py", line 331, in main
    raise Exception(
Exception: 1 tests failed out of 4
```

Failed test:
```
2026-03-07 21:39:09,217 - INFO - Found non-ASCII token: '分析'
```

### 2. Version Comparison
- **Status:** ✅ Completed
- **Results:** version_comparison.csv
- **Log:** version_comparison.log

#### Version Comparison Details:
```
Current Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.9-smg-b367912d', 'server_api': 'http://localhost:8084/v1/chat/completions', 'model_name': 'gpt-oss-120b', 'max_tokens': [100, 200, 300]}
Detected mode: generate_current

=== Generating Current Results (v0.5.9-smg-b367912d) ===

Generating results for v0.5.9-smg-b367912d (gpt-oss-120b)
Force regenerate: False
Using 8 concurrent workers
Executing 33 API requests concurrently...

Processing requests:   0%|          | 0/33 [00:00<?, ?req/s]
Processing requests:   3%|▎         | 1/33 [00:02<01:10,  2.21s/req]
Processing requests:   9%|▉         | 3/33 [00:02<00:21,  1.42req/s]
Processing requests:  15%|█▌        | 5/33 [00:02<00:12,  2.29req/s]
Processing requests:  18%|█▊        | 6/33 [00:03<00:12,  2.24req/s]
Processing requests:  24%|██▍       | 8/33 [00:03<00:08,  3.03req/s]
Processing requests:  27%|██▋       | 9/33 [00:04<00:09,  2.48req/s]
Processing requests:  33%|███▎      | 11/33 [00:05<00:10,  2.14req/s]
Processing requests:  36%|███▋      | 12/33 [00:05<00:09,  2.27req/s]
Processing requests:  39%|███▉      | 13/33 [00:06<00:08,  2.29req/s]
Processing requests:  42%|████▏     | 14/33 [00:06<00:07,  2.52req/s]
Processing requests:  45%|████▌     | 15/33 [00:07<00:08,  2.21req/s]
Processing requests:  52%|█████▏    | 17/33 [00:07<00:06,  2.32req/s]
Processing requests:  55%|█████▍    | 18/33 [00:08<00:05,  2.82req/s]
Processing requests:  58%|█████▊    | 19/33 [00:08<00:04,  3.02req/s]
Processing requests:  67%|██████▋   | 22/33 [00:09<00:04,  2.63req/s]
Processing requests:  70%|██████▉   | 23/33 [00:09<00:03,  2.67req/s]
Processing requests:  76%|███████▌  | 25/33 [00:10<00:02,  3.28req/s]
Processing requests:  79%|███████▉  | 26/33 [00:10<00:02,  3.17req/s]
Processing requests:  85%|████████▍ | 28/33 [00:11<00:01,  3.77req/s]
Processing requests:  88%|████████▊ | 29/33 [00:11<00:01,  2.71req/s]
Processing requests:  94%|█████████▍| 31/33 [00:12<00:00,  3.77req/s]
Processing requests:  97%|█████████▋| 32/33 [00:12<00:00,  4.33req/s]
Processing requests: 100%|██████████| 33/33 [00:13<00:00,  1.97req/s]
Processing requests: 100%|██████████| 33/33 [00:13<00:00,  2.44req/s]
✅ 33 requests successful, ❌ 0 failed
Saving 33 results to disk...
Generated/loaded 33 current results

Done!
```

### 3. Consistency Checks
- **Status:** ✅ Completed
- **Log:** consistency_check.log

#### Consistency Check Summary (last 8 lines):
```
2026-03-07 21:45:49,818 - INFO - ============================================================
2026-03-07 21:45:49,818 - INFO - CONSISTENCY CHECK SUMMARY
2026-03-07 21:45:49,818 - INFO - ============================================================
2026-03-07 21:45:49,818 - INFO - Sequential tests average consistency rate: 90.00%
2026-03-07 21:45:49,819 - INFO - Concurrent tests average consistency rate: 77.73%
2026-03-07 21:45:49,819 - INFO - Overall average consistency rate: 83.86%
2026-03-07 21:45:49,819 - INFO - ============================================================
Configuration: {'server_engine': 'SGLang', 'server_version': 'v0.5.9-smg-b367912d', 'server_api': 'http://localhost:8084/v1/chat/completions', 'model_name': 'gpt-oss-120b'}
```

### 4. lm_eval (MMLU Pro)
- **Status:** ✅ Completed
- **Results:** lm_eval_results/
- **Log:** lm_eval.log

#### lm_eval Results (last 5 lines):
```

| Groups |Version|    Filter    |n-shot|  Metric   |   |Value |   |Stderr|
|--------|------:|--------------|------|-----------|---|-----:|---|-----:|
|mmlu_pro|      2|custom-extract|      |exact_match|↑  |0.7469|±  |0.0038|

```

### 5. BFCL (Function Calling)
- **Status:** ❌ Failed (Not run in this PR. BFCL failed in previous PR for the same model)
- **Results:** Check BFCL_PROJECT_ROOT: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_9-smg-b367912d_20260307_213859/bfcl_results
- **Logs:** bfcl.log, bfcl_eval.log

#### BFCL Overall Accuracy:
```
BFCL results CSV file not found at: /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_9-smg-b367912d_20260307_213859/bfcl_results/score/data_overall.csv
```

### 6. LooGLE (Long Document QA)
- **Status:** ❌ Failed (LooGLE failed in previous PR for the same model)
- **Results:** loogle_results/
- **Log:** loogle.log

#### LooGLE Results (last 2 lines):
```
                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
AttributeError: 'NoneType' object has no attribute 'strip'
```
Note: 
- The reason it failed is that some responses have content=None.
- The test also failed in previous PR (https://bitbucket.oci.oraclecorp.com/projects/GENAICORE/repos/ome/pull-requests/1036/overview) for the same model.
- The model does generate response for some long context requests.

### 7. Image Number Check
- **Status:** ✅ Completed
- **Results:** image_number_check/
- **Log:** image_number_check.log

#### Image Number Check Results (last 5 lines):
```
2026-03-07 23:57:47,924 - INFO - Testing with image saved as WEBP but declared as JPEG
2026-03-07 23:57:49,572 - INFO - Test WEBP-as-JPEG: Success
2026-03-07 23:57:49,572 - INFO - Testing with image saved as WEBP but declared as WEBP
2026-03-07 23:57:51,209 - INFO - Test WEBP-as-WEBP: Success
2026-03-07 23:57:51,224 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_9-smg-b367912d_20260307_213859/image_number_check/image_number_check_SGLang_v0.5.9-smg-b367912d.csv
```

### 8. Image Size Check
- **Status:** ✅ Completed
- **Results:** image_size_check/
- **Log:** image_size_check.log

#### Image Size Check Results (last 5 lines):
```
2026-03-07 23:58:02,872 - INFO - Testing image dimensions: 4097x4097
2026-03-07 23:58:11,737 - INFO - Dimensions 4097x4097: Success
2026-03-07 23:58:11,737 - INFO - Testing image dimensions: 8192x8192
2026-03-07 23:58:15,989 - INFO - Dimensions 8192x8192: Success
2026-03-07 23:58:15,993 - INFO - Results saved to /Users/panxia/Documents/BitBucket/moirai-internal/sanity_check/openai/SGLang_v0_5_9-smg-b367912d_20260307_213859/image_size_check/image_size_check_SGLang_v0.5.9-smg-b367912d.csv
```

## Files Generated
