---
title: 安装
---

## 二进制安装

前往 GitHub 仓库的 [Release](https://github.com/Fallen-Breath/pavonis/releases) 页面，下载对应平台的发布包

Pavonis 使用 Go 语言静态编译构建，发布包仅含单个可执行文件，不依赖其他系统库

下载发布包，解压后，可得一个 `pavonis` 或 `pavonis.exe` 的单个可执行文件

```bash
$ # 测试运行
$ ./pavonis -v
Pavonis v0.1.0

$ # 正式运行，传入配置文件路径
$ ./pavonis -c /path/to/config.yml
INFO[2025-05-28 21:25:15.168] Pavonis initializing ...
...
```

## Docker

Pavonis 支持通过容器形式部署，其镜像位于 Docker Hub [fallenbreath/pavonis](https://hub.docker.com/r/fallenbreath/pavonis)

可通过如下面命令直接运行 Pavonis

```bash
$ # 测试运行
$ docker run --rm fallenbreath/pavonis -v
Pavonis v0.1.0

$ # 正式运行，挂载配置文件
$ docker run --name pavonis -v ./config.yml:/etc/pavonis/config.yml -p 8009:8009 fallenbreath/pavonis
Pavonis v0.1.0
```

### Docker Compose

```yaml
services:
  pavonis:
    image: fallenbreath/pavonis
    restart: unless-stopped
    ports:
      - '8009:8009'
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./config.yml:/etc/pavonis/config.yml
```
