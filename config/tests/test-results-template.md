# {Vendor} Test Results

**Test Date**: YYYY-MM-DD
**Cluster**: moirai-eu-frankfurt-1-dev

## Summary

| Model | Status | Download Time | Startup Time | Inference Test |
|-------|--------|---------------|--------------|----------------|
| {model-1} | ✅/❌ | Xm | Xm | ✅/❌ |
| {model-2} | ✅/❌ | Xm | Xm | ✅/❌ |

## Detailed Results

### {Model Name}

**Config Files**:
- Model: `config/models/{vendor}/{model}.yaml`
- Runtime: `config/runtimes/srt/{vendor}/{model}-rt.yaml`
- InferenceService: `config/samples/isvc/{vendor}/{model}.yaml`

**Test Steps**:
1. Applied Model: `kubectl apply -f config/models/{vendor}/{model}.yaml`
   - Status: ✅/❌
   - Time to Ready: X minutes

2. Applied Runtime: `kubectl apply -f config/runtimes/srt/{vendor}/{model}-rt.yaml`
   - Status: ✅/❌

3. Created Namespace: `kubectl create namespace {model-name}`
   - Status: ✅/❌

4. Applied InferenceService: `kubectl apply -f config/samples/isvc/{vendor}/{model}.yaml`
   - Status: ✅/❌
   - Time to Ready: X minutes

5. Inference Test:
   ```bash
   # Port-forward to engine service
   kubectl port-forward svc/{model-name}-engine 8080:8080 -n {model-name}

   # Test with HuggingFace model ID (from runtime's --served-model-name)
   curl -s http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "{vendor}/{Model-Name}",
       "messages": [{"role": "user", "content": "hello, who are you"}],
       "max_tokens": 100,
       "temperature": 0
     }'
   ```
   - Status: ✅/❌
   - Response Time: X ms

**Resource Usage**:
- GPUs: X
- Memory: X GB
- Pods: X running

**Issues Encountered**:
- None / Description of issues

**Cleanup**:
```bash
kubectl delete -f config/samples/isvc/{vendor}/{model}.yaml
kubectl delete namespace {model-name}
kubectl delete -f config/runtimes/srt/{vendor}/{model}-rt.yaml
kubectl delete -f config/models/{vendor}/{model}.yaml
```

---

## Notes

- Any additional observations or recommendations
