---
title: HuggingFace 下载代理
order: 6
---

# HuggingFace 下载代理（hugging_face）

## 简介

`hugging_face` 模式将 Pavonis 作为 HuggingFace 模型和数据集文件的下载代理，完整兼容 `huggingface-cli` 以及 `transformers`、`datasets` 等库的下载行为。

Pavonis 会自动处理 HuggingFace 下载流程中涉及的多个子域名（XET 协议相关），并将响应中的重定向链接、`Link` 头部、`X-Xet-Cas-Url` 头部统一重写为指向本代理的地址，客户端全程无需绕过 Pavonis。

::: warning self_url 必填
`hugging_face` 模式要求必须配置 `self_url`，格式为 `https://你的域名`（不含路径，不含尾部 `/`）。
:::

## 支持的访问路径

Pavonis 只允许以下格式的路径（符合 HuggingFace Hub 的下载路径规范）：

| 路径格式 | 说明 |
|----------|------|
| `/api/models/{author}/{repo}/...` | 模型元数据 API |
| `/api/datasets/{author}/{repo}/...` | 数据集元数据 API |
| `/{author}/{repo}/resolve/{commit-sha}/...` | 模型文件下载 |
| `/datasets/{author}/{repo}/resolve/{commit-sha}/...` | 数据集文件下载 |

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

### huggingface-cli

```bash
HF_ENDPOINT=https://hf.example.com huggingface-cli download \
  meta-llama/Llama-3.2-1B \
  --local-dir ./llama-3.2-1b
```

### Python（transformers / datasets）

```python
import os
os.environ["HF_ENDPOINT"] = "https://hf.example.com"

# 之后正常使用 transformers 即可
from transformers import AutoTokenizer
tokenizer = AutoTokenizer.from_pretrained("bert-base-uncased")
```

也可以在启动前设置环境变量，无需修改代码：

```bash
export HF_ENDPOINT=https://hf.example.com
python your_script.py
```
