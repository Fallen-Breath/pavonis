---
title: HuggingFace 下载代理
order: 6
---

# HuggingFace 下载代理（hugging_face）

## 简介

`hugging_face` 模式将 Pavonis 作为 HuggingFace 模型和数据集文件的下载代理，完整兼容 `hf`（`huggingface-cli`）命令行以及 `transformers`、`datasets` 等库的下载行为。

文件数据（包括 HuggingFace 文件存储 CDN 与 XET 协议的下载）同样经由 Pavonis 传输，客户端全程无需直连 HuggingFace。

::: warning self_url 必填
`hugging_face` 模式要求必须配置 `self_url`，格式为 `https://你的域名`（不含路径，不含尾部 `/`）。
:::

## 支持的访问路径

Pavonis 只允许以下格式的路径（符合 HuggingFace Hub 的下载路径规范）：

| 路径格式 | 说明 |
|----------|------|
| `/api/models/{author}/{repo}/...` | 模型元数据 API |
| `/api/datasets/{author}/{repo}/...` | 数据集元数据 API |
| `/{author}/{repo}/resolve/{revision}/...` | 模型文件下载 |
| `/datasets/{author}/{repo}/resolve/{revision}/...` | 数据集文件下载 |

`{revision}` 可以是分支名、标签或 commit SHA。不带 author 的旧式仓库名（如 `bert-base-uncased`）同样可用。

其他路径将返回 `404`。

## 配置项（settings）

> 对应 Go 配置类型：`HuggingFaceProxySettings`（当前无字段）

`hugging_face` 模式当前无额外的 `settings` 配置项。

## 配置示例

```yaml
sites:
  - id: huggingface
    host: hf.example.com
    mode: hugging_face
    self_url: https://hf.example.com
```

## 客户端使用方式

### hf 命令行

```bash
HF_ENDPOINT=https://hf.example.com hf download \
  meta-llama/Llama-3.2-1B \
  --local-dir ./llama-3.2-1b
```

### Python（transformers / datasets）

```python
import os
os.environ["HF_ENDPOINT"] = "https://hf.example.com"

# 之后正常使用 transformers 即可
from transformers import AutoTokenizer
tokenizer = AutoTokenizer.from_pretrained("google-bert/bert-base-uncased")
```

也可以在启动前设置环境变量，无需修改代码：

```bash
export HF_ENDPOINT=https://hf.example.com
python your_script.py
```
