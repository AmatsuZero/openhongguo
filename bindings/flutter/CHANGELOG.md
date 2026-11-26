## 未发布（同步上游 0.3.1 / 0.4.0 / 0.4.1）

- 播放改走 V5 取址接口，请求参数和签名与 7.4.3.32 版客户端一致，修复取播放模型失败。
- 读取演员作品时先取作品页的 tab 列表，再按「剧作」tab 请求；搜索联想关键词参数改为 `q`。
- 新增选项 `signerUrl` 和 `signerToken`（Go 是 `SignerURL`、`SignerToken`），可把请求签名交给 openhongguo-signer 服务；不设时仍用内置签名，服务不可用时返回 `SIGNER_UNAVAILABLE`，不回退。命令行用 `OHG_SIGNER_URL` / `OHG_SIGNER_TOKEN`。
- 修复取播放地址得到空结果：服务端从 2026-11-23 起拒绝带模拟器状态值的签名。内置签名改为与 App 自带签名库 v04.10.03 一致，设备环境按真机格式生成。
- 使用签名服务时，需把镜像升级到 `beilunyang/openhongguo-signer:v0.4.1`；旧版签名带模拟器状态值，播放会得到空结果。

## 0.1.0

- 第一个版本。
