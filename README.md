# overture

本项目是对 [overture](https://github.com/shawn1m/overture) 的魔改，由于改动较大，且不再兼容原版配置文件，故决定单独发布。

相比原版改动如下：

- 监听地址改为数组，可设置多个监听地址【不兼容原版配置文件】
- 新增屏蔽域名和屏蔽 IP 功能，可用于广告过滤等用途
- 新增替换域名和替换 IP 功能
- 新增独立查询日志文件，便于审查请求
- 缓存正在进行中的请求，防止同时重复发出同一个请求
- UDP 上游如果返回 truncated 消息，自动切换到 TCP 再次请求
- 对于 UDP 客户端，在必要时截断响应，以符合 DNS 标准
- 对于主动返回的空结果（hosts、屏蔽域名），增加 SOA 记录
- 连接池增加 MaxIdle 设置，以兼容最新版连接池库
- 连接池默认 IdleTimeout 改为 8 秒，因为主流公共服务器均为 10 秒超时
- 如果设置了 NoCookie，即使不设置 Client IP 也会生效，以兼容 dnspod 上游
- 优化结果判断，增加查询类型判断（如查询 AAAA 但只返回了 CNAME），优化对 SOA 结果的处理
- 优化缓存策略，不再缓存不含 SOA 的空结果，不再缓存查询类型不匹配的结果
- 优化错误日志，当所有上游都失败时输出查询摘要，便于检查问题
- edns clinet subnet mask 设置为 /16(IPv4) 和 /56(IPv6)
- 调度器增加 AlternativeFirst 选项，避免隐私泄露给主服务器（自用瞎改）
- ~~修复 suffix-tree 无法匹配的问题（原项目已合并 [#239](https://github.com/shawn1m/overture/pull/239)）~~
- ~~修复 hosts 与主流逻辑不符合的问题（原项目已合并 [#240](https://github.com/shawn1m/overture/pull/240)）~~

## 下载

<https://github.com/wzv5/overture/releases/latest>

或通过 [Scoop](https://scoop.sh):

``` text
scoop bucket add wzv5 https://github.com/wzv5/ScoopBucket
scoop install wzv5/overture
```

## License

This project is under the MIT license. See the [LICENSE](LICENSE) file for the full license text.
