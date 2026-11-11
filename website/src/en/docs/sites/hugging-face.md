---
title: HuggingFace Download Proxy
order: 6
---

# HuggingFace Download Proxy (hugging_face)

## Overview

The `hugging_face` mode turns Pavonis into a download proxy for HuggingFace models and datasets, fully compatible with the `hf` (`huggingface-cli`) CLI as well as `transformers`, `datasets` and other libraries.

File data (including downloads from the HuggingFace storage CDN and via the XET protocol) is also transferred through Pavonis, so clients never need to connect to HuggingFace directly.

::: warning self_url Required
The `hugging_face` mode requires `self_url` to be set, in the format `https://your-domain` (no path, no trailing `/`).
:::

## Supported Path Patterns

Pavonis only allows the following path formats (matching HuggingFace Hub download path conventions):

| Path Format | Description |
|-------------|-------------|
| `/api/models/{author}/{repo}/...` | Model metadata API |
| `/api/datasets/{author}/{repo}/...` | Dataset metadata API |
| `/{author}/{repo}/resolve/{revision}/...` | Model file download |
| `/datasets/{author}/{repo}/resolve/{revision}/...` | Dataset file download |

`{revision}` can be a branch, a tag or a commit SHA. Legacy repo names without the author (e.g. `bert-base-uncased`) also work.

All other paths return `404`.

## Settings

> Corresponding Go config type: `HuggingFaceProxySettings` (currently no fields)

The `hugging_face` mode currently has no additional `settings` options.

## Configuration Example

```yaml
sites:
  - id: huggingface
    host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
```

## Client Usage

### hf CLI

```bash
HF_ENDPOINT=https://hf.example.com hf download \
  meta-llama/Llama-3.2-1B \
  --local-dir ./llama-3.2-1b
```

### Python (transformers / datasets)

```python
import os
os.environ["HF_ENDPOINT"] = "https://hf.example.com"

# Use transformers as usual
from transformers import AutoTokenizer
tokenizer = AutoTokenizer.from_pretrained("google-bert/bert-base-uncased")
```

Or set the environment variable before running, without modifying code:

```bash
export HF_ENDPOINT=https://hf.example.com
python your_script.py
```
