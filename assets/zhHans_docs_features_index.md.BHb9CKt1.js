import{_ as s,o as n,c as e,ag as p}from"./chunks/framework.DlQ8ZhpE.js";const f=JSON.parse('{"title":"功能特性","description":"","frontmatter":{"title":"功能特性","order":2},"headers":[],"relativePath":"zhHans/docs/features/index.md","filePath":"zhHans/docs/features/index.md"}'),i={name:"zhHans/docs/features/index.md"};function t(l,a,o,r,c,d){return n(),e("div",null,[...a[0]||(a[0]=[p(`<h1 id="功能特性" tabindex="-1">功能特性 <a class="header-anchor" href="#功能特性" aria-label="Permalink to &quot;功能特性&quot;">​</a></h1><p>一个请求在 Pavonis 中依次经过的环节：</p><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes github-light github-dark vp-code" tabindex="0"><code><span class="line"><span>客户端</span></span>
<span class="line"><span>  │</span></span>
<span class="line"><span>  ▼</span></span>
<span class="line"><span>按 host / path_prefix 选择站点 ── 无匹配 ──► 404</span></span>
<span class="line"><span>  │</span></span>
<span class="line"><span>  ▼</span></span>
<span class="line"><span>static_files 路径 ──────────────── 命中 ──► 资源限制 ──► 本地文件 / 静态内容</span></span>
<span class="line"><span>  │ 未命中</span></span>
<span class="line"><span>  ▼</span></span>
<span class="line"><span>资源限制（请求频率、并发） ─────── 超限 ──► 429</span></span>
<span class="line"><span>  │</span></span>
<span class="line"><span>  ▼</span></span>
<span class="line"><span>客户端认证（启用时） ───────────── 失败 ──► 401</span></span>
<span class="line"><span>  │</span></span>
<span class="line"><span>  ▼</span></span>
<span class="line"><span>请求头修改 → 出站连接（IP 池 / 请求代理） ──► 上游</span></span>
<span class="line"><span>（上传与下载的数据受带宽限制约束）</span></span></code></pre></div><h2 id="站点模式-site-modes" tabindex="-1">站点模式（Site Modes） <a class="header-anchor" href="#站点模式-site-modes" aria-label="Permalink to &quot;站点模式（Site Modes）&quot;">​</a></h2><p>Pavonis 的核心能力通过<strong>站点模式</strong>来体现。一个实例可以同时运行任意数量的站点，每个站点选择一种模式，覆盖容器镜像、代码托管、机器学习平台、包管理等主流场景。</p><p>→ <a href="./../sites/">详细文档：站点模式</a></p><h2 id="静态文件" tabindex="-1">静态文件 <a class="header-anchor" href="#静态文件" aria-label="Permalink to &quot;静态文件&quot;">​</a></h2><p>站点可以在指定路径上直接返回本地文件或固定内容，例如首页或健康检查接口。即使站点启用了客户端认证，这些路径也无需认证。</p><p>→ <a href="./../config/sites#staticfileconfig-静态文件">详细文档：StaticFileConfig</a></p><h2 id="出站-ip-池" tabindex="-1">出站 IP 池 <a class="header-anchor" href="#出站-ip-池" aria-label="Permalink to &quot;出站 IP 池&quot;">​</a></h2><p>当你的服务器持有大段 IPv6 地址时（例如 HE Tunnel Broker 分配的 <code>/48</code> 子网），可以启用 IP 池，让 Pavonis 从池内随机或按规则选取出站 IP，有效规避上游平台对单一 IP 的速率限制。支持 <code>random</code>（随机）和 <code>ip_hash</code>（按客户端 IP 固定映射）等策略，也可以按站点单独覆盖。</p><p>→ <a href="./ip-pool">详细文档：出站 IP 池</a></p><h2 id="资源限制" tabindex="-1">资源限制 <a class="header-anchor" href="#资源限制" aria-label="Permalink to &quot;资源限制&quot;">​</a></h2><p>按客户端或所有客户端合计限制带宽、请求频率和并发请求数，还可以把站点分成配额组，为不同站点设置不同的配额。</p><p>→ <a href="./rate-limiting">详细文档：资源限制</a></p><h2 id="认证、请求头与请求代理" tabindex="-1">认证、请求头与请求代理 <a class="header-anchor" href="#认证、请求头与请求代理" aria-label="Permalink to &quot;认证、请求头与请求代理&quot;">​</a></h2><p>容器镜像站点和 <code>any_url</code> 站点支持客户端身份验证，用户文件支持热重载，更新用户列表无需重启。Pavonis 默认清理反向代理相关的请求头，避免将代理链信息泄漏给上游。上游请求还可以配置一个或多个请求代理，某个代理连不上时自动切换到其他代理。</p><p>→ <a href="./auth-and-headers">详细文档：认证、请求头与请求代理</a></p>`,18)])])}const u=s(i,[["render",t]]);export{f as __pageData,u as default};
