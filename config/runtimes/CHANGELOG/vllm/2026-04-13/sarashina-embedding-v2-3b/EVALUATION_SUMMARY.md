Using CPython 3.12.11
Creating virtual environment at: .venv
Downloading setuptools (1.1MiB)
Downloading networkx (2.0MiB)
Downloading sympy (6.0MiB)
Downloading sentencepiece (1.2MiB)
Downloading tokenizers (2.8MiB)
Downloading scipy (19.2MiB)
Downloading hf-xet (2.6MiB)
Downloading numpy (5.0MiB)
Downloading scikit-learn (7.7MiB)
Downloading torch (71.0MiB)
Downloading transformers (11.4MiB)
 Downloaded setuptools
 Downloaded sentencepiece
 Downloaded networkx
 Downloaded hf-xet
 Downloaded tokenizers
 Downloaded numpy
 Downloaded sympy
 Downloaded scikit-learn
 Downloaded transformers
 Downloaded scipy
 Downloaded torch
Installed 32 packages in 279ms

Model: sarashina-embedding-v2-3b
Endpoint: http://localhost:8087/v1

Encoding query...

============================================================
  Query Embedding
============================================================
  Shape: (1, 2560)
  Dtype: float64
  Min value: -80.062500
  Max value: 97.000000
  Mean value: -0.010407
  Std deviation: 5.872528

  First 10 values of first embedding:
  [-13.328125     0.09521484   1.06835938   1.26269531  -3.38476562
  -7.34765625   2.81640625   1.27441406   1.33007812   8.25      ]
============================================================

Encoding texts...

============================================================
  Text Embeddings
============================================================
  Shape: (3, 2560)
  Dtype: float64
  Min value: -98.375000
  Max value: 93.937500
  Mean value: 0.008038
  Std deviation: 6.016381

  First 10 values of first embedding:
  [-7.7109375  -1.95898438 -6.12109375 -0.31958008  5.328125   -3.24609375
 -5.421875    5.35546875 -2.18359375  8.9375    ]
============================================================

Computing similarity scores...

============================================================
  SIMILARITY SCORES
============================================================

<Query>
```
task: I will provide a query, so please search for related passages that answer the given web search query.
query: Is there a Sarashina text embedding model?
```

Rank   Score      Text
------------------------------------------------------------
1      0.905916   text: Sarashina Embedding is a Japanese em...
2      0.881070   text: Sarashina is a Japanese large-scale ...
3      0.798812   text: The Sarashina Diary is an autobiogra...
============================================================

Expected best match: index 2
Actual best match:   index 2

[PASS] Model correctly ranked the most relevant text highest!
